using System.Net.Http.Headers;

namespace Quorum.Client;

/// <summary>Puts an identity on outgoing requests. Use <see cref="Identity"/> against a server in <c>trust</c> mode and <see cref="BearerToken"/> against one in <c>verify</c> mode.</summary>
public abstract record Credentials
{
    internal abstract void Apply(HttpRequestHeaders headers, TrustHeaderNames names, string? defaultTenant);
}

/// <summary>
/// A trusted identity, sent as gateway headers. Only safe when the client itself is the
/// trusted gateway, or sits behind one that strips these headers from the outside world.
/// </summary>
public sealed record Identity : Credentials
{
    public required string UserId { get; init; }
    /// <summary>Falls back to the client's configured tenant.</summary>
    public string? TenantId { get; init; }
    public IReadOnlyList<string>? Roles { get; init; }
    public IReadOnlyList<string>? Permissions { get; init; }
    /// <summary>Named identity facts such as <c>branch_id</c>. Each is sent in the header the server reads it from; see <see cref="TrustHeaderNames.Attributes"/>.</summary>
    public IReadOnlyDictionary<string, string>? Attributes { get; init; }

    internal override void Apply(HttpRequestHeaders headers, TrustHeaderNames names, string? defaultTenant)
    {
        if (!string.IsNullOrEmpty(UserId)) headers.TryAddWithoutValidation(names.UserId, UserId);
        var tenant = TenantId ?? defaultTenant;
        if (!string.IsNullOrEmpty(tenant)) headers.TryAddWithoutValidation(names.TenantId, tenant);
        if (Roles is { Count: > 0 }) headers.TryAddWithoutValidation(names.Roles, string.Join(",", Roles));
        if (Permissions is { Count: > 0 }) headers.TryAddWithoutValidation(names.Permissions, string.Join(",", Permissions));
        if (Attributes is null) return;
        foreach (var (attr, value) in Attributes)
        {
            if (!string.IsNullOrEmpty(value) && names.Attributes.TryGetValue(attr, out var header))
                headers.TryAddWithoutValidation(header, value);
        }
    }
}

/// <summary>A JWT for a server in <c>verify</c> mode.</summary>
public sealed record BearerToken(string Token) : Credentials
{
    internal override void Apply(HttpRequestHeaders headers, TrustHeaderNames names, string? defaultTenant) =>
        headers.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", Token);
}

/// <summary>The headers a <c>trust</c> mode server reads identity from. They must match the server's <c>auth.trust</c> configuration.</summary>
public sealed record TrustHeaderNames
{
    public string UserId { get; init; } = "X-User-ID";
    public string Roles { get; init; } = "X-User-Roles";
    public string Permissions { get; init; } = "X-User-Permissions";
    public string TenantId { get; init; } = "X-Tenant-ID";
    /// <summary>Maps an attribute name to its header, e.g. <c>{ ["branch_id"] = "X-User-Branch" }</c>.</summary>
    public IReadOnlyDictionary<string, string> Attributes { get; init; } = new Dictionary<string, string>();
}
