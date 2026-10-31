using MediatR;

namespace Quorum.Client.MediatR;

/// <summary>
/// Submits commands for approval. The command is the request payload; its handler runs when
/// <see cref="ApprovalDispatcher"/> receives the approved delivery, whether a checker approved
/// it or a bypass rule did.
/// </summary>
public sealed class Approvals(QuorumClient quorum, IQuorumIdentityProvider? identity = null)
{
    /// <summary>
    /// Opens a Quorum request for <paramref name="command"/> as the current identity and returns
    /// it. A <see cref="RequestStatus.Pending"/> result is waiting on checkers; an
    /// <see cref="RequestStatus.Approved"/> one matched a bypass rule. Either way the handler runs
    /// from the webhook, so react to the decision in a <see cref="RequestDecided"/> handler.
    /// </summary>
    public Task<Request> SubmitAsync<TResponse>(IRequest<TResponse> command, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(command);
        var requestType = ApprovalRegistry.RequestTypeOf(command.GetType())
            ?? throw new ArgumentException($"{command.GetType().Name} is not marked [RequiresApproval].", nameof(command));

        var client = identity is null ? quorum : quorum.As(identity.Credentials);
        return client.CreateRequestAsync(new CreateRequest
        {
            Type = requestType,
            Payload = command,
            IdempotencyKey = (command as IIdempotentCommand)?.IdempotencyKey,
        }, cancellationToken);
    }
}
