using System.Text.Json;
using MediatR;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Options;

namespace Quorum.Client.MediatR;

/// <summary>
/// Turns Quorum webhook deliveries into command executions. Point your webhook endpoint at
/// <see cref="DispatchAsync(Stream, string?, CancellationToken)"/>: an <c>approved</c> delivery
/// decodes the command from the request payload and sends it through MediatR in a fresh scope;
/// every terminal delivery publishes <see cref="RequestDecided"/>.
/// </summary>
public sealed class ApprovalDispatcher(
    IServiceScopeFactory scopes,
    ApprovalRegistry registry,
    IOptions<QuorumApprovalOptions> options,
    IOptions<QuorumClientOptions> clientOptions)
{
    /// <summary>Verifies a delivery against the configured secret, then dispatches it. Throws <see cref="InvalidSignatureException"/> when the signature does not verify; respond 401 and do not retry.</summary>
    public async Task<DispatchResult> DispatchAsync(Stream body, string? signature, CancellationToken cancellationToken = default)
    {
        var secret = options.Value.WebhookSecret
            ?? throw new InvalidOperationException($"{nameof(QuorumApprovalOptions)}.{nameof(QuorumApprovalOptions.WebhookSecret)} is not set.");
        var delivery = await Webhooks.ParseAsync(body, signature, secret, cancellationToken).ConfigureAwait(false);
        return await DispatchAsync(delivery, cancellationToken).ConfigureAwait(false);
    }

    /// <summary>Dispatches a delivery you have already verified.</summary>
    public async Task<DispatchResult> DispatchAsync(WebhookPayload delivery, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(delivery);
        var request = delivery.Request;
        if (!registry.TryGetCommand(request.Type, out var commandType) || !request.IsTerminal || WasBypassed(delivery))
        {
            return new DispatchResult(DispatchOutcome.Ignored, request, null);
        }

        var command = JsonSerializer.Deserialize(request.Payload, commandType, clientOptions.Value.PayloadSerializerOptions)
            ?? throw new JsonException($"Payload of {request.Type} {request.Id} decoded to null.");

        using var scope = scopes.CreateScope();
        var sp = scope.ServiceProvider;
        object? result = null;
        var outcome = DispatchOutcome.Declined;
        if (delivery.Event == WebhookEvent.Approved)
        {
            sp.GetRequiredService<ApprovalContext>().Request = request;
            result = await sp.GetRequiredService<ISender>().Send(command, cancellationToken).ConfigureAwait(false);
            outcome = DispatchOutcome.Executed;
        }
        await sp.GetRequiredService<IPublisher>().Publish(new RequestDecided(request, command, result), cancellationToken).ConfigureAwait(false);
        return new DispatchResult(outcome, request, result);
    }

    // A bypassed request is approved with no approvals and already ran inline
    // when it was submitted; its delivery must not run it again.
    private static bool WasBypassed(WebhookPayload delivery) =>
        delivery.Event == WebhookEvent.Approved && delivery.Approvals is not { Count: > 0 } && delivery.Request.Approvals is not { Count: > 0 };
}
