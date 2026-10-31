using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Options;

namespace Quorum.Client;

/// <summary>Registers <see cref="QuorumClient"/> with the host's <c>IHttpClientFactory</c>.</summary>
public static class QuorumServiceCollectionExtensions
{
    /// <summary>
    /// Adds a singleton-style <see cref="QuorumClient"/> backed by a named <see cref="HttpClient"/>.
    /// Inject <see cref="QuorumClient"/> and call <see cref="QuorumClient.As"/> per request.
    /// </summary>
    public static IHttpClientBuilder AddQuorumClient(this IServiceCollection services, Action<QuorumClientOptions> configure)
    {
        ArgumentNullException.ThrowIfNull(services);
        ArgumentNullException.ThrowIfNull(configure);
        services.AddOptions<QuorumClientOptions>().Configure(configure);
        return services
            .AddHttpClient(nameof(QuorumClient), (sp, http) =>
            {
                var o = sp.GetRequiredService<IOptions<QuorumClientOptions>>().Value;
                http.BaseAddress = o.BaseAddress;
                http.Timeout = Timeout.InfiniteTimeSpan; // event streams; per-call timeout is in the options
            })
            .AddTypedClient((http, sp) => new QuorumClient(http, sp.GetRequiredService<IOptions<QuorumClientOptions>>().Value));
    }
}
