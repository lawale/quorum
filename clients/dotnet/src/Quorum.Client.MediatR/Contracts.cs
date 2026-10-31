using MediatR;

namespace Quorum.Client.MediatR;

/// <summary>
/// Marks a MediatR request that must be approved in Quorum before its handler runs. The
/// command itself is the request payload; the checker approves exactly the object that later executes.
/// </summary>
/// <param name="requestType">The Quorum request type that selects the policy. Defaults to the class name in snake_case, so <c>SubmitWireTransfer</c> becomes <c>submit_wire_transfer</c>.</param>
[AttributeUsage(AttributeTargets.Class | AttributeTargets.Struct, Inherited = false)]
public sealed class RequiresApprovalAttribute(string? requestType = null) : Attribute
{
    public string? RequestType { get; } = requestType;
}

/// <summary>A command that carries its own idempotency key, so a retried submission returns the request opened the first time.</summary>
public interface IIdempotentCommand
{
    string IdempotencyKey { get; }
}

/// <summary>Supplies the identity a command is submitted under, typically from the current HTTP user. Register as scoped.</summary>
public interface IQuorumIdentityProvider
{
    Credentials Credentials { get; }
}

/// <summary>What a handler can learn about the approval it is running under. Inject it into handlers that need the maker, the approvers or the request ID.</summary>
public interface IApprovalContext
{
    /// <summary>The approved request this handler is executing, or null when the command did not require approval.</summary>
    Request? Request { get; }
}

/// <summary>
/// Thrown when a command marked <see cref="RequiresApprovalAttribute"/> is sent straight to its
/// handler. Submit it with <see cref="Approvals.SubmitAsync{TResponse}"/> instead; the handler runs
/// from the approved webhook.
/// </summary>
public sealed class ApprovalRequiredException(Type command, string requestType)
    : InvalidOperationException($"{command.Name} requires approval as '{requestType}'. Submit it through Approvals.SubmitAsync; its handler runs when Quorum approves it.")
{
    public Type Command { get; } = command;
    public string RequestType { get; } = requestType;
}

/// <summary>
/// Published when a request reaches a terminal status. For an approved request it is published
/// after the command's handler has run, with the handler's result.
/// </summary>
/// <param name="Request">The request as delivered by the webhook.</param>
/// <param name="Command">The command decoded from the request payload.</param>
/// <param name="Result">What the handler returned, for approved requests only.</param>
public sealed record RequestDecided(Request Request, object Command, object? Result) : INotification;

/// <summary>What <see cref="ApprovalDispatcher"/> did with a delivery.</summary>
public enum DispatchOutcome
{
    /// <summary>The delivery was not for a known command, or was not a decision. Nothing ran.</summary>
    Ignored,
    /// <summary>The request was approved and the command's handler ran.</summary>
    Executed,
    /// <summary>The request was rejected, cancelled or expired. <see cref="RequestDecided"/> was published.</summary>
    Declined,
}

/// <summary>The result of dispatching one delivery.</summary>
/// <param name="Outcome">What happened.</param>
/// <param name="Request">The request the delivery was about.</param>
/// <param name="Result">The handler's result when <paramref name="Outcome"/> is <see cref="DispatchOutcome.Executed"/>.</param>
public sealed record DispatchResult(DispatchOutcome Outcome, Request Request, object? Result);
