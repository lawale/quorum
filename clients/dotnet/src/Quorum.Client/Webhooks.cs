using System.Security.Cryptography;
using System.Text;
using System.Text.Json;

namespace Quorum.Client;

/// <summary>Verifies and decodes webhook deliveries.</summary>
public static class Webhooks
{
    /// <summary>The header carrying the HMAC-SHA256 of a delivery's body.</summary>
    public const string SignatureHeader = "X-Signature-256";

    /// <summary>The largest delivery <see cref="ParseAsync"/> will read.</summary>
    public const int MaxBodyBytes = 4 << 20;

    /// <summary>Produces the signature header value Quorum would send for <paramref name="body"/>. For tests and for proxies that re-sign deliveries.</summary>
    public static string Sign(ReadOnlySpan<byte> body, string secret)
    {
        Span<byte> mac = stackalloc byte[HMACSHA256.HashSizeInBytes];
        HMACSHA256.HashData(Encoding.UTF8.GetBytes(secret), body, mac);
        return "sha256=" + Convert.ToHexString(mac).ToLowerInvariant();
    }

    /// <inheritdoc cref="Sign(ReadOnlySpan{byte}, string)"/>
    public static string Sign(string body, string secret) => Sign(Encoding.UTF8.GetBytes(body), secret);

    /// <summary>Reports whether <paramref name="signature"/>, in the form <c>sha256=&lt;hex&gt;</c>, is the HMAC-SHA256 of <paramref name="body"/> under <paramref name="secret"/>. The comparison is constant-time.</summary>
    public static bool Verify(ReadOnlySpan<byte> body, string? signature, string secret)
    {
        if (string.IsNullOrEmpty(signature) || string.IsNullOrEmpty(secret)) return false;
        if (!signature.StartsWith("sha256=", StringComparison.Ordinal)) return false;
        byte[] given;
        try { given = Convert.FromHexString(signature.AsSpan("sha256=".Length)); }
        catch (FormatException) { return false; }
        Span<byte> mac = stackalloc byte[HMACSHA256.HashSizeInBytes];
        HMACSHA256.HashData(Encoding.UTF8.GetBytes(secret), body, mac);
        return CryptographicOperations.FixedTimeEquals(given, mac);
    }

    /// <inheritdoc cref="Verify(ReadOnlySpan{byte}, string?, string)"/>
    public static bool Verify(string body, string? signature, string secret) => Verify(Encoding.UTF8.GetBytes(body), signature, secret);

    /// <summary>
    /// Verifies a delivery's signature against <paramref name="secret"/> and decodes it. Throws
    /// <see cref="InvalidSignatureException"/> for anything not signed with the secret; reject the
    /// request when it does. Respond 2xx once you have durably recorded the event, otherwise Quorum retries.
    /// Pass the raw body exactly as received: re-serialised JSON will not verify.
    /// </summary>
    public static WebhookPayload Parse(ReadOnlySpan<byte> body, string? signature, string secret)
    {
        if (!Verify(body, signature, secret)) throw new InvalidSignatureException();
        return JsonSerializer.Deserialize<WebhookPayload>(body, QuorumJson.Options)
            ?? throw new JsonException("quorum: webhook body was null");
    }

    /// <inheritdoc cref="Parse(ReadOnlySpan{byte}, string?, string)"/>
    public static WebhookPayload Parse(string body, string? signature, string secret) => Parse(Encoding.UTF8.GetBytes(body), signature, secret);

    /// <summary>Reads the delivery from a request body stream, then verifies and decodes it as <see cref="Parse(ReadOnlySpan{byte}, string?, string)"/> does.</summary>
    public static async Task<WebhookPayload> ParseAsync(Stream body, string? signature, string secret, CancellationToken cancellationToken = default)
    {
        using var buffer = new MemoryStream();
        var limited = new byte[8192];
        int read;
        while ((read = await body.ReadAsync(limited, cancellationToken).ConfigureAwait(false)) > 0)
        {
            buffer.Write(limited, 0, read);
            if (buffer.Length > MaxBodyBytes) throw new InvalidDataException("quorum: webhook body too large");
        }
        return Parse(buffer.GetBuffer().AsSpan(0, (int)buffer.Length), signature, secret);
    }
}
