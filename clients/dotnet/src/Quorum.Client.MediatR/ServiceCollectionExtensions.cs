using System.Reflection;
using MediatR;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.DependencyInjection.Extensions;

namespace Quorum.Client.MediatR;

/// <summary>Settings for routing commands through Quorum.</summary>
public sealed class QuorumApprovalOptions
{
    /// <summary>The secret of the webhook subscription that delivers decisions to this application.</summary>
    public string? WebhookSecret { get; set; }
}

/// <summary>Registers the approval pipeline.</summary>
public static class QuorumApprovalServiceCollectionExtensions
{
    /// <summary>
    /// Adds <see cref="Approvals"/> for submitting commands, <see cref="ApprovalDispatcher"/> for
    /// the webhook endpoint, and the pipeline guard that keeps marked commands out of their
    /// handlers until approved. Call it after <c>AddMediatR</c> and <c>AddQuorumClient</c>. <paramref name="assemblies"/> are scanned for commands marked
    /// <see cref="RequiresApprovalAttribute"/>; the calling assembly is used when none are given.
    /// </summary>
    public static IServiceCollection AddQuorumApprovals(this IServiceCollection services, Action<QuorumApprovalOptions>? configure = null, params Assembly[] assemblies)
    {
        ArgumentNullException.ThrowIfNull(services);
        if (assemblies.Length == 0) assemblies = [Assembly.GetCallingAssembly()];

        services.AddOptions<QuorumApprovalOptions>().Configure(configure ?? (_ => { }));
        services.TryAddSingleton(new ApprovalRegistry(assemblies));
        services.TryAddSingleton<ApprovalDispatcher>();
        services.TryAddScoped<Approvals>();
        services.TryAddScoped<ApprovalContext>();
        services.TryAddScoped<IApprovalContext>(sp => sp.GetRequiredService<ApprovalContext>());
        services.TryAddEnumerable(ServiceDescriptor.Transient(typeof(IPipelineBehavior<,>), typeof(ApprovalBehavior<,>)));
        return services;
    }
}
