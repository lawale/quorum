using System.Collections.Concurrent;
using System.Reflection;
using System.Text.Json;
using System.ComponentModel;

namespace Quorum.Client.MediatR;

/// <summary>Maps Quorum request types to the command types that carry <see cref="RequiresApprovalAttribute"/>. Infrastructure; registered by <c>AddQuorumApprovals</c>.</summary>
[EditorBrowsable(EditorBrowsableState.Never)]
public sealed class ApprovalRegistry
{
    private static readonly ConcurrentDictionary<Type, string?> RequestTypes = new();
    private readonly Dictionary<string, Type> _commands = new(StringComparer.Ordinal);

    public ApprovalRegistry(IEnumerable<Assembly> assemblies)
    {
        foreach (var type in assemblies.SelectMany(SafeTypes))
        {
            var requestType = RequestTypeOf(type);
            if (requestType is null) continue;
            if (_commands.TryGetValue(requestType, out var existing) && existing != type)
            {
                throw new InvalidOperationException($"{type} and {existing} both claim the Quorum request type '{requestType}'.");
            }
            _commands[requestType] = type;
        }
    }

    public bool TryGetCommand(string requestType, out Type command) => _commands.TryGetValue(requestType, out command!);

    /// <summary>The request type of a command, or null when it needs no approval.</summary>
    public static string? RequestTypeOf(Type command) => RequestTypes.GetOrAdd(command, static t =>
    {
        var attr = t.GetCustomAttribute<RequiresApprovalAttribute>();
        if (attr is null) return null;
        return attr.RequestType ?? JsonNamingPolicy.SnakeCaseLower.ConvertName(t.Name);
    });

    private static IEnumerable<Type> SafeTypes(Assembly assembly)
    {
        try
        {
            return assembly.GetTypes();
        }
        catch (ReflectionTypeLoadException e)
        {
            return e.Types.Where(t => t is not null)!;
        }
    }
}

/// <summary>Scoped state: set by the dispatcher before replaying an approved command, read by the behaviour and by handlers. Inject <see cref="IApprovalContext"/> instead.</summary>
[EditorBrowsable(EditorBrowsableState.Never)]
public sealed class ApprovalContext : IApprovalContext
{
    public Request? Request { get; set; }
}
