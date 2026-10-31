using System.Text.Json;

namespace Quorum.Client;

// The API's types, as described by api/openapi.yaml. Property names map to
// the wire by snake_case (see QuorumJson). The test suite drives the real
// server with unmapped members disallowed, so a field the server sends that
// is missing here fails the build's tests rather than being silently dropped.

/// <summary>Where a request is in its life.</summary>
public enum RequestStatus { Pending, Approved, Rejected, Cancelled, Expired }

/// <summary>A checker's vote.</summary>
public enum Decision { Approved, Rejected }

/// <summary>How a stage combines role and permission rules when both are set.</summary>
public enum AuthorizationMode { Role, Permission, Any, All }

/// <summary>When a stage's rejections end the request.</summary>
public enum RejectionPolicy { Any, Threshold }

/// <summary>Comparison in a bypass rule's payload condition.</summary>
public enum BypassOp { Eq, Ne, Lt, Lte, Gt, Gte, In }

/// <summary>Events a webhook can subscribe to.</summary>
public enum WebhookEvent { ReviewRequested, Approved, Rejected, Cancelled, Expired }

/// <summary>Why a <c>review_requested</c> event was sent.</summary>
public enum ReviewReason { Created, StageAdvanced, BypassLimitExceeded }

/// <summary>An approval request.</summary>
public sealed record Request
{
    public required Guid Id { get; init; }
    public required string TenantId { get; init; }
    public string? IdempotencyKey { get; init; }
    public required string Type { get; init; }
    /// <summary>The change being proposed, exactly as the maker sent it. Decode it with <see cref="PayloadAs{T}"/>.</summary>
    public required JsonElement Payload { get; init; }
    public required RequestStatus Status { get; init; }
    public required string MakerId { get; init; }
    /// <summary>The maker's identity attributes captured at creation, such as <c>branch_id</c>.</summary>
    public IReadOnlyDictionary<string, string>? MakerAttributes { get; init; }
    public IReadOnlyList<string>? EligibleReviewers { get; init; }
    public JsonElement? Metadata { get; init; }
    /// <summary>Hash of the policy's identity fields, used to refuse duplicate pending requests.</summary>
    public string? Fingerprint { get; init; }
    public required int CurrentStage { get; init; }
    public DateTimeOffset? ExpiresAt { get; init; }
    public required DateTimeOffset CreatedAt { get; init; }
    public required DateTimeOffset UpdatedAt { get; init; }
    public IReadOnlyList<Approval>? Approvals { get; init; }
    /// <summary>Whether the caller may approve or reject right now. Present on reads and decisions.</summary>
    public bool? ViewerCanAct { get; init; }
    public int? TotalStages { get; init; }

    /// <summary>Whether the request can still change.</summary>
    [System.Text.Json.Serialization.JsonIgnore]
    public bool IsTerminal => Status != RequestStatus.Pending;

    /// <summary>Decodes the payload with the client's payload serializer settings (web defaults unless configured).</summary>
    public T? PayloadAs<T>(JsonSerializerOptions? options = null) =>
        Payload.Deserialize<T>(options ?? QuorumJson.DefaultPayloadOptions);
}

/// <summary>One checker's recorded decision.</summary>
public sealed record Approval
{
    public required Guid Id { get; init; }
    public required Guid RequestId { get; init; }
    public required string CheckerId { get; init; }
    public required Decision Decision { get; init; }
    public required int StageIndex { get; init; }
    public string? Comment { get; init; }
    public required DateTimeOffset CreatedAt { get; init; }
}

/// <summary>One entry in a request's audit trail.</summary>
public sealed record AuditLog
{
    public required Guid Id { get; init; }
    public required string TenantId { get; init; }
    public required Guid RequestId { get; init; }
    /// <summary>created, approved, rejected, cancelled, expired, bypassed, stage_advanced, webhook_sent or webhook_failed.</summary>
    public required string Action { get; init; }
    public required string ActorId { get; init; }
    public JsonElement? Details { get; init; }
    public required DateTimeOffset CreatedAt { get; init; }
}

/// <summary>One page of a list endpoint.</summary>
public sealed record Page<T>
{
    public required IReadOnlyList<T> Data { get; init; }
    public required int Total { get; init; }
    [System.Text.Json.Serialization.JsonPropertyName("page")]
    public required int PageNumber { get; init; }
}

/// <summary>Restricts a stage to checkers whose identity attribute matches the rule's sources.</summary>
public sealed record ScopeRule
{
    public required string Attribute { get; init; }
    public bool? MatchMaker { get; init; }
    /// <summary>Dot-notation path into the payload.</summary>
    public string? PayloadPath { get; init; }
    public IReadOnlyList<string>? Values { get; init; }
    /// <summary>Lets a checker with no value for the attribute act on any request.</summary>
    public bool? AllowUnscopedCheckers { get; init; }
}

/// <summary>One step of a policy.</summary>
public sealed record ApprovalStage
{
    /// <summary>Sequential from 0.</summary>
    public required int Index { get; init; }
    public string? Name { get; init; }
    public required int RequiredApprovals { get; init; }
    public IReadOnlyList<string>? AllowedCheckerRoles { get; init; }
    public IReadOnlyList<string>? AllowedPermissions { get; init; }
    /// <summary>Required when both roles and permissions are set.</summary>
    public AuthorizationMode? AuthorizationMode { get; init; }
    /// <summary>Defaults to <see cref="RejectionPolicy.Any"/>.</summary>
    public RejectionPolicy? RejectionPolicy { get; init; }
    /// <summary>Required for the <see cref="RejectionPolicy.Threshold"/> rejection policy.</summary>
    public int? MaxCheckers { get; init; }
    public IReadOnlyList<ScopeRule>? Scope { get; init; }
}

/// <summary>A comparison against a payload field.</summary>
public sealed record BypassCondition
{
    public required string Path { get; init; }
    public required BypassOp Op { get; init; }
    public required object? Value { get; init; }
}

/// <summary>A sum cap for a bypass limit.</summary>
public sealed record BypassMaxSum
{
    public required string Path { get; init; }
    public required double Value { get; init; }
}

/// <summary>A velocity limit on a bypass rule.</summary>
public sealed record BypassLimit
{
    /// <summary>A Go duration, additionally accepting a <c>d</c> suffix.</summary>
    public required string Window { get; init; }
    public int? MaxCount { get; init; }
    public BypassMaxSum? MaxSum { get; init; }
    public IReadOnlyList<string>? GroupBy { get; init; }
}

/// <summary>Requests that are approved on creation.</summary>
public sealed record BypassRule
{
    public string? Name { get; init; }
    public IReadOnlyList<string>? MakerRoles { get; init; }
    public IReadOnlyList<string>? MakerPermissions { get; init; }
    public IReadOnlyDictionary<string, IReadOnlyList<string>>? MakerAttributes { get; init; }
    public IReadOnlyList<BypassCondition>? Payload { get; init; }
    public IReadOnlyList<BypassLimit>? Limits { get; init; }
}

/// <summary>Body of <see cref="QuorumClient.CreatePolicyAsync"/>.</summary>
public sealed record PolicyInput
{
    public required string Name { get; init; }
    public required string RequestType { get; init; }
    public required IReadOnlyList<ApprovalStage> Stages { get; init; }
    /// <summary>Payload paths that identify the thing being changed; two pending requests may not share them.</summary>
    public IReadOnlyList<string>? IdentityFields { get; init; }
    public string? DynamicAuthorizationUrl { get; init; }
    public string? DynamicAuthorizationSecret { get; init; }
    /// <summary>A Go duration such as <c>24h</c>; see <see cref="Duration"/>.</summary>
    public string? AutoExpireDuration { get; init; }
    public JsonElement? DisplayTemplate { get; init; }
    public IReadOnlyList<BypassRule>? BypassRules { get; init; }
}

/// <summary>Body of <see cref="QuorumClient.UpdatePolicyAsync"/>. A null field is left unchanged; an empty list clears it.</summary>
public sealed record PolicyUpdate
{
    public string? Name { get; init; }
    public IReadOnlyList<ApprovalStage>? Stages { get; init; }
    public IReadOnlyList<string>? IdentityFields { get; init; }
    public string? DynamicAuthorizationUrl { get; init; }
    public string? DynamicAuthorizationSecret { get; init; }
    public string? AutoExpireDuration { get; init; }
    public JsonElement? DisplayTemplate { get; init; }
    public IReadOnlyList<BypassRule>? BypassRules { get; init; }
}

/// <summary>The rules for one request type in a tenant.</summary>
public sealed record Policy
{
    public required Guid Id { get; init; }
    public required string TenantId { get; init; }
    public required string Name { get; init; }
    public required string RequestType { get; init; }
    public required IReadOnlyList<ApprovalStage> Stages { get; init; }
    public IReadOnlyList<string>? IdentityFields { get; init; }
    public string? DynamicAuthorizationUrl { get; init; }
    public string? AutoExpireDuration { get; init; }
    public JsonElement? DisplayTemplate { get; init; }
    public IReadOnlyList<BypassRule>? BypassRules { get; init; }
    public required DateTimeOffset CreatedAt { get; init; }
    public required DateTimeOffset UpdatedAt { get; init; }
    /// <summary>Advice from the server about the policy as saved, such as a scope attribute the auth mode cannot supply.</summary>
    public IReadOnlyList<string>? Warnings { get; init; }
}

/// <summary>Body of <see cref="QuorumClient.CreateWebhookAsync"/>.</summary>
public sealed record WebhookInput
{
    public required Uri Url { get; init; }
    public required IReadOnlyList<WebhookEvent> Events { get; init; }
    /// <summary>Shared secret for the HMAC signature.</summary>
    public required string Secret { get; init; }
    /// <summary>Restrict deliveries to one request type.</summary>
    public string? RequestType { get; init; }
}

/// <summary>A webhook subscription. The secret is never returned.</summary>
public sealed record Webhook
{
    public required Guid Id { get; init; }
    public required string TenantId { get; init; }
    public required Uri Url { get; init; }
    public required IReadOnlyList<string> Events { get; init; }
    public string? RequestType { get; init; }
    public required bool Active { get; init; }
    public required DateTimeOffset CreatedAt { get; init; }
}

/// <summary>Who may act on the stage a request is waiting on. Sent with <c>review_requested</c>.</summary>
public sealed record ReviewRequest
{
    public required ReviewReason Reason { get; init; }
    public required int StageIndex { get; init; }
    public string? StageName { get; init; }
    public required int TotalStages { get; init; }
    public required int RequiredApprovals { get; init; }
    public required IReadOnlyList<string> EligibleReviewers { get; init; }
    public IReadOnlyList<string>? AllowedRoles { get; init; }
    public IReadOnlyList<string>? AllowedPermissions { get; init; }
    public string? AuthorizationMode { get; init; }
    public required IReadOnlyList<ResolvedScope> Scope { get; init; }
    public IReadOnlyList<string>? ExcludedUserIds { get; init; }
    public DateTimeOffset? ExpiresAt { get; init; }
}

/// <summary>A scope rule resolved against a concrete request.</summary>
public sealed record ResolvedScope
{
    public required string Attribute { get; init; }
    public required IReadOnlyList<string> Values { get; init; }
    public bool? AllowUnscopedCheckers { get; init; }
}

/// <summary>The body of every webhook delivery.</summary>
public sealed record WebhookPayload
{
    public required WebhookEvent Event { get; init; }
    public required Request Request { get; init; }
    public IReadOnlyList<Approval>? Approvals { get; init; }
    public ReviewRequest? Review { get; init; }
    public required DateTimeOffset Timestamp { get; init; }
}

/// <summary>One component of the server's health report.</summary>
public sealed record ComponentHealth
{
    public required string Status { get; init; }
    public string? Error { get; init; }
}

/// <summary>The server's health report.</summary>
public sealed record Health
{
    /// <summary><c>healthy</c> or <c>unhealthy</c>.</summary>
    public required string Status { get; init; }
    /// <summary>Release the server was built from, or <c>dev</c>.</summary>
    public required string Version { get; init; }
    public required IReadOnlyDictionary<string, ComponentHealth> Components { get; init; }
}

internal sealed record ErrorBody(string Error);
internal sealed record CreateRequestInput(string Type, JsonElement Payload, IReadOnlyList<string>? EligibleReviewers, JsonElement? Metadata);
internal sealed record DecisionInput(string? Comment);
internal sealed record ListBody<T>(IReadOnlyList<T> Data);
