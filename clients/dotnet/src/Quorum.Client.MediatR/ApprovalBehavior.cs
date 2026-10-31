using MediatR;

namespace Quorum.Client.MediatR;

/// <summary>
/// Stops a command marked <see cref="RequiresApprovalAttribute"/> from reaching its handler
/// unless <see cref="ApprovalDispatcher"/> is replaying it after approval. Submission goes
/// through <see cref="Approvals.SubmitAsync{TResponse}"/>, never through <c>ISender.Send</c>.
/// </summary>
public sealed class ApprovalBehavior<TRequest, TResponse>(ApprovalContext context) : IPipelineBehavior<TRequest, TResponse>
    where TRequest : notnull
{
    public Task<TResponse> Handle(TRequest request, RequestHandlerDelegate<TResponse> next, CancellationToken cancellationToken)
    {
        var requestType = ApprovalRegistry.RequestTypeOf(typeof(TRequest));
        if (requestType is not null && context.Request?.Type != requestType)
        {
            throw new ApprovalRequiredException(typeof(TRequest), requestType);
        }
        return next(cancellationToken);
    }
}
