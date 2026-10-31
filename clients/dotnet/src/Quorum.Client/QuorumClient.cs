using System.Net;
using System.Net.Http.Headers;
using System.Reflection;
using System.Runtime.CompilerServices;
using System.Text;
using System.Text.Json;

namespace Quorum.Client;

/// <summary>Settings for a <see cref="QuorumClient"/>.</summary>
public sealed class QuorumClientOptions
{
    /// <summary>The Quorum server, for example <c>https://quorum.example.com</c>. Required unless the <see cref="HttpClient"/> already has a base address.</summary>
    public Uri? BaseAddress { get; set; }
    /// <summary>Tenant sent with trusted identities that do not name one themselves.</summary>
    public string? Tenant { get; set; }
    /// <summary>Identity used until <see cref="QuorumClient.As"/> is called.</summary>
    public Credentials? Credentials { get; set; }
    /// <summary>Header names a <c>trust</c> mode server reads identity from.</summary>
    public TrustHeaderNames TrustHeaders { get; set; } = new();
    /// <summary>Per-call timeout. Default 30 seconds. Event streams are not bounded.</summary>
    public TimeSpan Timeout { get; set; } = TimeSpan.FromSeconds(30);
    /// <summary>Replaces the default <c>quorum-dotnet-client/&lt;version&gt;</c>.</summary>
    public string? UserAgent { get; set; }
    /// <summary>How request payloads and metadata are serialised. Default: web defaults (camelCase).</summary>
    public JsonSerializerOptions PayloadSerializerOptions { get; set; } = QuorumJson.DefaultPayloadOptions;

    internal bool StrictModels { get; set; }

    internal QuorumClientOptions Clone() => (QuorumClientOptions)MemberwiseClone();
}

/// <summary>Options every call accepts.</summary>
public sealed record CreateRequest
{
    /// <summary>Selects the policy.</summary>
    public required string Type { get; init; }
    /// <summary>The change being proposed. Any object; serialised with <see cref="QuorumClientOptions.PayloadSerializerOptions"/>. Must produce a non-empty JSON object.</summary>
    public required object Payload { get; init; }
    /// <summary>Per-request allowlist of checker IDs; overrides role and permission rules.</summary>
    public IReadOnlyList<string>? EligibleReviewers { get; init; }
    /// <summary>Free-form data for the consumer; <c>display</c> is rendered by the console and widgets.</summary>
    public object? Metadata { get; init; }
    /// <summary>Retrying a submission with the same key returns the original request instead of opening another.</summary>
    public string? IdempotencyKey { get; init; }
}

/// <summary>Filters for <see cref="QuorumClient.ListRequestsAsync"/>.</summary>
public sealed record ListRequestsFilter
{
    public RequestStatus? Status { get; init; }
    public string? Type { get; init; }
    public string? MakerId { get; init; }
    public string? Search { get; init; }
    public int? Page { get; init; }
    /// <summary>Capped at 100 by the server.</summary>
    public int? PerPage { get; init; }
}

/// <summary>Paging for list calls.</summary>
public sealed record Paging(int? Page = null, int? PerPage = null);

/// <summary>One message from a request's event stream.</summary>
/// <param name="RequestId">The request the event is about.</param>
/// <param name="Status">Set only on the closing event of an already-terminal request. For an <c>updated</c> event it is null: fetch the request to see the change.</param>
public sealed record StreamEvent(Guid RequestId, RequestStatus? Status)
{
    /// <summary>Whether the stream has nothing more to say after this event.</summary>
    public bool IsTerminal => Status is { } s && s != RequestStatus.Pending;
}

/// <summary>
/// Calls one Quorum server. It is safe for concurrent use. Derive per-identity clients with
/// <see cref="As"/>; one instance can serve every maker and checker in a process.
/// </summary>
public sealed class QuorumClient
{
    private readonly HttpClient _http;
    private readonly QuorumClientOptions _options;
    private readonly Uri _baseAddress;
    private readonly JsonSerializerOptions _json;
    private readonly string _userAgent;

    /// <summary>The version of this client package, as published.</summary>
    public static string Version { get; } = ResolveVersion();

    /// <summary>Creates a client with its own <see cref="HttpClient"/>.</summary>
    public QuorumClient(Uri baseAddress, Action<QuorumClientOptions>? configure = null)
        : this(new HttpClient { Timeout = System.Threading.Timeout.InfiniteTimeSpan }, Configure(baseAddress, configure))
    {
    }

    /// <summary>
    /// Creates a client over an <see cref="HttpClient"/> you manage, for example from <c>IHttpClientFactory</c>.
    /// Set its <see cref="HttpClient.Timeout"/> to infinite so event streams are not cut off; the per-call
    /// <see cref="QuorumClientOptions.Timeout"/> still applies.
    /// </summary>
    public QuorumClient(HttpClient httpClient, QuorumClientOptions options)
    {
        _http = httpClient ?? throw new ArgumentNullException(nameof(httpClient));
        _options = options ?? throw new ArgumentNullException(nameof(options));
        var baseAddress = options.BaseAddress ?? httpClient.BaseAddress
            ?? throw new ArgumentException("quorum: a base address is required", nameof(options));
        if (!baseAddress.IsAbsoluteUri || (baseAddress.Scheme != Uri.UriSchemeHttp && baseAddress.Scheme != Uri.UriSchemeHttps))
            throw new ArgumentException($"quorum: base address must be absolute http(s), got {baseAddress}", nameof(options));
        _baseAddress = new Uri(baseAddress.GetLeftPart(UriPartial.Path).TrimEnd('/') + "/");
        _json = options.StrictModels ? QuorumJson.Create(strict: true) : QuorumJson.Options;
        _userAgent = options.UserAgent ?? $"quorum-dotnet-client/{Version}";
    }

    private static QuorumClientOptions Configure(Uri baseAddress, Action<QuorumClientOptions>? configure)
    {
        var o = new QuorumClientOptions { BaseAddress = baseAddress };
        configure?.Invoke(o);
        return o;
    }

    /// <summary>Returns a client that acts as <paramref name="credentials"/>. The receiver is unchanged.</summary>
    public QuorumClient As(Credentials credentials)
    {
        var o = _options.Clone();
        o.Credentials = credentials;
        o.BaseAddress = _baseAddress;
        return new QuorumClient(_http, o);
    }

    /// <summary>Reports the server's health and version. Needs no identity.</summary>
    public async Task<Health> HealthAsync(CancellationToken cancellationToken = default)
    {
        using var response = await SendAsync(HttpMethod.Get, "health", null, null, cancellationToken).ConfigureAwait(false);
        if (response.StatusCode is HttpStatusCode.OK or HttpStatusCode.ServiceUnavailable)
            return await ReadAsync<Health>(response, cancellationToken).ConfigureAwait(false);
        throw await ErrorAsync(response, HttpMethod.Get, "health", cancellationToken).ConfigureAwait(false);
    }

    /// <summary>Submits a request on behalf of the client's identity, who becomes its maker.</summary>
    public async Task<Request> CreateRequestAsync(CreateRequest input, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(input);
        var body = new CreateRequestInput(
            input.Type,
            JsonSerializer.SerializeToElement(input.Payload, _options.PayloadSerializerOptions),
            input.EligibleReviewers,
            input.Metadata is null ? null : JsonSerializer.SerializeToElement(input.Metadata, _options.PayloadSerializerOptions));
        Action<HttpRequestMessage>? extra = input.IdempotencyKey is { Length: > 0 } key
            ? m => m.Headers.TryAddWithoutValidation("Idempotency-Key", key)
            : null;
        return await CallAsync<Request>(HttpMethod.Post, "api/v1/requests", body, extra, cancellationToken).ConfigureAwait(false);
    }

    public Task<Request> GetRequestAsync(Guid id, CancellationToken cancellationToken = default) =>
        CallAsync<Request>(HttpMethod.Get, $"api/v1/requests/{id}", null, null, cancellationToken);

    public Task<Page<Request>> ListRequestsAsync(ListRequestsFilter? filter = null, CancellationToken cancellationToken = default)
    {
        filter ??= new ListRequestsFilter();
        var q = new QueryBuilder();
        if (filter.Status is { } status) q.Add("status", EnumName(status));
        q.Add("type", filter.Type);
        q.Add("maker_id", filter.MakerId);
        q.Add("search", filter.Search);
        q.Add("page", filter.Page);
        q.Add("per_page", filter.PerPage);
        return CallAsync<Page<Request>>(HttpMethod.Get, "api/v1/requests" + q, null, null, cancellationToken);
    }

    /// <summary>Records an approval by the client's identity at the request's current stage.</summary>
    public Task<Request> ApproveAsync(Guid id, string? comment = null, CancellationToken cancellationToken = default) =>
        DecideAsync(id, "approve", comment, cancellationToken);

    /// <summary>Records a rejection by the client's identity; the request becomes terminal.</summary>
    public Task<Request> RejectAsync(Guid id, string? comment = null, CancellationToken cancellationToken = default) =>
        DecideAsync(id, "reject", comment, cancellationToken);

    private Task<Request> DecideAsync(Guid id, string verb, string? comment, CancellationToken cancellationToken) =>
        CallAsync<Request>(HttpMethod.Post, $"api/v1/requests/{id}/{verb}",
            string.IsNullOrEmpty(comment) ? null : new DecisionInput(comment), null, cancellationToken);

    /// <summary>Withdraws a pending request. Only its maker may do so.</summary>
    public Task<Request> CancelAsync(Guid id, CancellationToken cancellationToken = default) =>
        CallAsync<Request>(HttpMethod.Post, $"api/v1/requests/{id}/cancel", null, null, cancellationToken);

    /// <summary>The request's audit trail, oldest first.</summary>
    public async Task<IReadOnlyList<AuditLog>> AuditAsync(Guid id, CancellationToken cancellationToken = default) =>
        (await CallAsync<ListBody<AuditLog>>(HttpMethod.Get, $"api/v1/requests/{id}/audit", null, null, cancellationToken).ConfigureAwait(false)).Data;

    /// <summary>
    /// Subscribes to a request's server-sent events. The sequence ends when the server closes the
    /// stream (the request was already terminal) or the token is cancelled. A server with streaming
    /// disabled throws a 404 <see cref="QuorumException"/> immediately.
    /// </summary>
    public async IAsyncEnumerable<StreamEvent> EventsAsync(Guid id, [EnumeratorCancellation] CancellationToken cancellationToken = default)
    {
        var path = $"api/v1/requests/{id}/events";
        using var message = Build(HttpMethod.Get, path, null, null);
        message.Headers.Accept.Add(new MediaTypeWithQualityHeaderValue("text/event-stream"));
        using var response = await _http.SendAsync(message, HttpCompletionOption.ResponseHeadersRead, cancellationToken).ConfigureAwait(false);
        if (response.StatusCode != HttpStatusCode.OK)
            throw await ErrorAsync(response, HttpMethod.Get, path, cancellationToken).ConfigureAwait(false);

        using var stream = await response.Content.ReadAsStreamAsync(cancellationToken).ConfigureAwait(false);
        using var reader = new StreamReader(stream, Encoding.UTF8);
        string? name = null;
        var data = new StringBuilder();
        while (await reader.ReadLineAsync(cancellationToken).ConfigureAwait(false) is { } line)
        {
            if (line.Length == 0)
            {
                if (name is not null || data.Length > 0)
                {
                    yield return ParseEvent(name, data.ToString());
                    name = null;
                    data.Clear();
                }
            }
            else if (line[0] == ':') { /* keepalive */ }
            else if (line.StartsWith("event:", StringComparison.Ordinal)) name = line["event:".Length..].Trim();
            else if (line.StartsWith("data:", StringComparison.Ordinal)) data.Append(line["data:".Length..].Trim());
        }
    }

    private static StreamEvent ParseEvent(string? name, string data)
    {
        try
        {
            using var doc = JsonDocument.Parse(data);
            var root = doc.RootElement;
            var id = root.TryGetProperty("request_id", out var idEl) ? idEl.GetGuid() : Guid.Empty;
            RequestStatus? status = root.TryGetProperty("status", out var st) && st.ValueKind == JsonValueKind.String
                ? JsonSerializer.Deserialize<RequestStatus>(st.GetRawText(), QuorumJson.Options)
                : null;
            return new StreamEvent(id, status);
        }
        catch (JsonException e)
        {
            throw new JsonException($"quorum: malformed \"{name}\" event", e);
        }
    }

    /// <summary>
    /// Waits until the request reaches a terminal status and returns it. Listens on the event stream
    /// and falls back to polling when streaming is unavailable or drops. Bound the wait with the token.
    /// </summary>
    /// <param name="id">The request to wait on.</param>
    /// <param name="pollInterval">How often to re-fetch the request when the event stream is unavailable. Default 5 seconds.</param>
    /// <param name="cancellationToken">Bounds the wait; the usual way to give up is a token with a timeout.</param>
    public async Task<Request> WaitForDecisionAsync(Guid id, TimeSpan? pollInterval = null, CancellationToken cancellationToken = default)
    {
        var every = pollInterval is { } p && p > TimeSpan.Zero ? p : TimeSpan.FromSeconds(5);
        while (true)
        {
            // A fresh read first: the request may already be decided, and this
            // catches anything that happened before the stream opened.
            var current = await GetRequestAsync(id, cancellationToken).ConfigureAwait(false);
            if (current.IsTerminal) return current;

            bool streamFailed;
            try
            {
                await FollowStreamAsync(id, cancellationToken).ConfigureAwait(false);
                streamFailed = false;
            }
            catch (Exception) when (!cancellationToken.IsCancellationRequested)
            {
                streamFailed = true;
            }
            if (!streamFailed) continue; // the stream closed after an update; re-read

            return await PollAsync(id, every, cancellationToken).ConfigureAwait(false);
        }
    }

    private async Task FollowStreamAsync(Guid id, CancellationToken cancellationToken)
    {
        await foreach (var ev in EventsAsync(id, cancellationToken).ConfigureAwait(false))
        {
            if (ev.IsTerminal) return;
            var current = await GetRequestAsync(id, cancellationToken).ConfigureAwait(false);
            if (current.IsTerminal) return;
        }
    }

    private async Task<Request> PollAsync(Guid id, TimeSpan every, CancellationToken cancellationToken)
    {
        while (true)
        {
            await Task.Delay(every, cancellationToken).ConfigureAwait(false);
            try
            {
                var current = await GetRequestAsync(id, cancellationToken).ConfigureAwait(false);
                if (current.IsTerminal) return current;
            }
            catch (Exception) when (!cancellationToken.IsCancellationRequested)
            {
            }
        }
    }

    public Task<Policy> CreatePolicyAsync(PolicyInput input, CancellationToken cancellationToken = default) =>
        CallAsync<Policy>(HttpMethod.Post, "api/v1/policies", input, null, cancellationToken);

    public Task<Policy> GetPolicyAsync(Guid id, CancellationToken cancellationToken = default) =>
        CallAsync<Policy>(HttpMethod.Get, $"api/v1/policies/{id}", null, null, cancellationToken);

    public Task<Page<Policy>> ListPoliciesAsync(Paging? paging = null, CancellationToken cancellationToken = default) =>
        CallAsync<Page<Policy>>(HttpMethod.Get, "api/v1/policies" + PagingQuery(paging), null, null, cancellationToken);

    /// <summary>Replaces the fields given; an empty list clears a list, a null field keeps it.</summary>
    public Task<Policy> UpdatePolicyAsync(Guid id, PolicyUpdate update, CancellationToken cancellationToken = default) =>
        CallAsync<Policy>(HttpMethod.Put, $"api/v1/policies/{id}", update, null, cancellationToken);

    public Task DeletePolicyAsync(Guid id, CancellationToken cancellationToken = default) =>
        CallAsync<object?>(HttpMethod.Delete, $"api/v1/policies/{id}", null, null, cancellationToken);

    public Task<Webhook> CreateWebhookAsync(WebhookInput input, CancellationToken cancellationToken = default) =>
        CallAsync<Webhook>(HttpMethod.Post, "api/v1/webhooks", input, null, cancellationToken);

    public Task<Page<Webhook>> ListWebhooksAsync(Paging? paging = null, CancellationToken cancellationToken = default) =>
        CallAsync<Page<Webhook>>(HttpMethod.Get, "api/v1/webhooks" + PagingQuery(paging), null, null, cancellationToken);

    public Task DeleteWebhookAsync(Guid id, CancellationToken cancellationToken = default) =>
        CallAsync<object?>(HttpMethod.Delete, $"api/v1/webhooks/{id}", null, null, cancellationToken);

    private async Task<T> CallAsync<T>(HttpMethod method, string path, object? body, Action<HttpRequestMessage>? extra, CancellationToken cancellationToken)
    {
        using var response = await SendAsync(method, path, body, extra, cancellationToken).ConfigureAwait(false);
        if (!response.IsSuccessStatusCode)
            throw await ErrorAsync(response, method, path, cancellationToken).ConfigureAwait(false);
        if (response.StatusCode == HttpStatusCode.NoContent || typeof(T) == typeof(object))
            return default!;
        return await ReadAsync<T>(response, cancellationToken).ConfigureAwait(false);
    }

    private async Task<HttpResponseMessage> SendAsync(HttpMethod method, string path, object? body, Action<HttpRequestMessage>? extra, CancellationToken cancellationToken)
    {
        using var message = Build(method, path, body, extra);
        using var timeout = _options.Timeout > TimeSpan.Zero ? new CancellationTokenSource(_options.Timeout) : null;
        using var linked = timeout is null ? null : CancellationTokenSource.CreateLinkedTokenSource(cancellationToken, timeout.Token);
        var token = linked?.Token ?? cancellationToken;
        try
        {
            return await _http.SendAsync(message, HttpCompletionOption.ResponseContentRead, token).ConfigureAwait(false);
        }
        catch (OperationCanceledException) when (timeout is { IsCancellationRequested: true } && !cancellationToken.IsCancellationRequested)
        {
            throw new TimeoutException($"quorum: {method} {path} did not complete within {_options.Timeout}");
        }
    }

    private HttpRequestMessage Build(HttpMethod method, string path, object? body, Action<HttpRequestMessage>? extra)
    {
        var message = new HttpRequestMessage(method, new Uri(_baseAddress, path));
        message.Headers.TryAddWithoutValidation("User-Agent", _userAgent);
        _options.Credentials?.Apply(message.Headers, _options.TrustHeaders, _options.Tenant);
        if (body is not null)
        {
            message.Content = new StringContent(JsonSerializer.Serialize(body, body.GetType(), _json), Encoding.UTF8, "application/json");
        }
        extra?.Invoke(message);
        return message;
    }

    private async Task<T> ReadAsync<T>(HttpResponseMessage response, CancellationToken cancellationToken)
    {
        await using var stream = await response.Content.ReadAsStreamAsync(cancellationToken).ConfigureAwait(false);
        return await JsonSerializer.DeserializeAsync<T>(stream, _json, cancellationToken).ConfigureAwait(false)
            ?? throw new JsonException("quorum: empty response body");
    }

    private static async Task<QuorumException> ErrorAsync(HttpResponseMessage response, HttpMethod method, string path, CancellationToken cancellationToken)
    {
        string detail = response.ReasonPhrase ?? "request failed";
        try
        {
            var text = await response.Content.ReadAsStringAsync(cancellationToken).ConfigureAwait(false);
            if (text.Length > 0)
            {
                var parsed = JsonSerializer.Deserialize<ErrorBody>(text, QuorumJson.Options);
                if (!string.IsNullOrEmpty(parsed?.Error)) detail = parsed.Error;
            }
        }
        catch (JsonException) { }
        return new QuorumException(response.StatusCode, detail, method, "/" + path);
    }

    private static string EnumName<T>(T value) where T : struct, Enum =>
        JsonSerializer.Serialize(value, QuorumJson.Options).Trim('"');

    private static string PagingQuery(Paging? paging)
    {
        var q = new QueryBuilder();
        q.Add("page", paging?.Page);
        q.Add("per_page", paging?.PerPage);
        return q.ToString();
    }

    private static string ResolveVersion()
    {
        var info = typeof(QuorumClient).Assembly.GetCustomAttribute<AssemblyInformationalVersionAttribute>()?.InformationalVersion;
        if (string.IsNullOrEmpty(info)) return "0.0.0";
        var plus = info.IndexOf('+');
        return plus > 0 ? info[..plus] : info;
    }

    private sealed class QueryBuilder
    {
        private readonly StringBuilder _sb = new();

        public void Add(string key, string? value)
        {
            if (string.IsNullOrEmpty(value)) return;
            _sb.Append(_sb.Length == 0 ? '?' : '&').Append(key).Append('=').Append(Uri.EscapeDataString(value));
        }

        public void Add(string key, int? value)
        {
            if (value is > 0) Add(key, value.Value.ToString(System.Globalization.CultureInfo.InvariantCulture));
        }

        public override string ToString() => _sb.ToString();
    }
}
