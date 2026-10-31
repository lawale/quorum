using MediatR;

namespace Quorum.Client.MediatR;

/// <summary>
/// Intercepts commands marked <see cref="RequiresApprovalAttribute"/>. On first submission the
/// command is sent to Quorum and <see cref="ApprovalPendingException"/> is thrown instead of
/// running the handler. When Quorum approves it on creation (a bypass rule matched) the handler
/// runs at once. When <see cref="ApprovalDispatcher"/> replays an approved command, it passes through.
/// </summary>
public sealed class ApprovalBehavior<TRequest, TResponse>(
    QuorumClient quorum,
    ApprovalContext context,
    IQuorumIdentityProvider? identity = null) : IPipelineBehavior<TRequest, TResponse>
    where TRequest : notnull
{
    public async Task<TResponse> Handle(TRequest request, RequestHandlerDelegate<TResponse> next, CancellationToken cancellationToken)
    {
        var requestType = ApprovalRegistry.RequestTypeOf(typeof(TRequest));
        if (requestType is null) return await next(cancellationToken).ConfigureAwait(false);

        if (context.Request is { } approved && approved.Type == requestType)
        {
            return await next(cancellationToken).ConfigureAwait(false);
        }

        var client = identity is null ? quorum : quorum.As(identity.Credentials);
        var created = await client.CreateRequestAsync(new CreateRequest
        {
            Type = requestType,
            Payload = request,
            IdempotencyKey = (request as IIdempotentCommand)?.IdempotencyKey,
        }, cancellationToken).ConfigureAwait(false);

        if (created.Status == RequestStatus.Approved && created.Approvals is not { Count: > 0 })
        {
            context.Request = created;
            return await next(cancellationToken).ConfigureAwait(false);
        }

        throw new ApprovalPendingException(created);
    }
}
