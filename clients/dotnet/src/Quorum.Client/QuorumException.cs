using System.Net;

namespace Quorum.Client;

/// <summary>An error response from the Quorum server.</summary>
public sealed class QuorumException : Exception
{
    public QuorumException(HttpStatusCode statusCode, string detail, HttpMethod method, string path)
        : base($"quorum: {method} {path}: {(int)statusCode} {detail}")
    {
        StatusCode = statusCode;
        Detail = detail;
        Method = method;
        Path = path;
    }

    /// <summary>HTTP status code.</summary>
    public HttpStatusCode StatusCode { get; }
    /// <summary>The server's error message exactly as sent.</summary>
    public string Detail { get; }
    public HttpMethod Method { get; }
    public string Path { get; }

    public bool IsBadRequest => StatusCode == HttpStatusCode.BadRequest;
    public bool IsUnauthorized => StatusCode == HttpStatusCode.Unauthorized;
    public bool IsForbidden => StatusCode == HttpStatusCode.Forbidden;
    public bool IsNotFound => StatusCode == HttpStatusCode.NotFound;
    public bool IsConflict => StatusCode == HttpStatusCode.Conflict;
}

/// <summary>A webhook delivery whose signature did not verify.</summary>
public sealed class InvalidSignatureException() : Exception("quorum: webhook signature invalid");
