using System.Text;
using System.Text.Json;
using MediatR;
using Microsoft.Extensions.DependencyInjection;
using Quorum.Client.MediatR;

namespace Quorum.Client.Tests;

[Collection(nameof(ApiServerCollection))]
public sealed class ApprovalTests(ApiServerFixture server)
{
    private const string Secret = "whsec_test";

    private static readonly Identity Alice = new() { UserId = "alice", Roles = ["teller"] };
    private static readonly Identity Bob = new() { UserId = "bob", Roles = ["finance"] };
    private static readonly Identity Hq = new() { UserId = "hq", Roles = ["manager"] };

    [RequiresApproval]
    public sealed record SubmitWireTransfer(string TransferId, int Amount) : IRequest<string>, IIdempotentCommand
    {
        string IIdempotentCommand.IdempotencyKey => TransferId;
    }

    [RequiresApproval("petty_expense")]
    public sealed record FileExpense(string ExpenseId, int Amount) : IRequest<string>;

    [RequiresApproval]
    public sealed record CloseAccount(string AccountId) : IRequest<string>;

    public sealed record Ping(string Text) : IRequest<string>;

    public sealed class Ledger
    {
        public List<(object Command, Request? Approval)> Executed { get; } = [];
        public List<RequestDecided> Decided { get; } = [];
    }

    public sealed class CurrentUser : IQuorumIdentityProvider
    {
        public Credentials Credentials { get; set; } = Alice;
    }

    public sealed class Handlers(Ledger ledger, IApprovalContext approval) :
        IRequestHandler<SubmitWireTransfer, string>,
        IRequestHandler<FileExpense, string>,
        IRequestHandler<CloseAccount, string>,
        IRequestHandler<Ping, string>,
        INotificationHandler<RequestDecided>
    {
        public Task<string> Handle(SubmitWireTransfer request, CancellationToken ct) => Execute(request, request.TransferId);
        public Task<string> Handle(FileExpense request, CancellationToken ct) => Execute(request, request.ExpenseId);
        public Task<string> Handle(CloseAccount request, CancellationToken ct) => Execute(request, request.AccountId);
        public Task<string> Handle(Ping request, CancellationToken ct) => Task.FromResult("pong " + request.Text);

        public Task Handle(RequestDecided notification, CancellationToken ct)
        {
            ledger.Decided.Add(notification);
            return Task.CompletedTask;
        }

        private Task<string> Execute(object command, string id)
        {
            ledger.Executed.Add((command, approval.Request));
            return Task.FromResult("executed " + id);
        }
    }

    private ServiceProvider Build()
    {
        var services = new ServiceCollection();
        services.AddLogging();
        services.AddQuorumClient(o =>
        {
            o.BaseAddress = server.Url;
            o.Tenant = "banking";
            o.PayloadSerializerOptions = new JsonSerializerOptions { PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower };
            o.StrictModels = true;
        });
        services.AddMediatR(cfg => cfg.RegisterServicesFromAssembly(typeof(ApprovalTests).Assembly));
        services.AddQuorumApprovals(o => o.WebhookSecret = Secret, typeof(ApprovalTests).Assembly);
        services.AddScoped<CurrentUser>();
        services.AddScoped<IQuorumIdentityProvider>(sp => sp.GetRequiredService<CurrentUser>());
        services.AddSingleton<Ledger>();
        return services.BuildServiceProvider();
    }

    private static async Task<Request> SubmitAs<TResponse>(ServiceProvider sp, Credentials who, IRequest<TResponse> command)
    {
        using var scope = sp.CreateScope();
        scope.ServiceProvider.GetRequiredService<CurrentUser>().Credentials = who;
        return await scope.ServiceProvider.GetRequiredService<Approvals>().SubmitAsync(command);
    }

    private static async Task<TResponse> SendAs<TResponse>(ServiceProvider sp, Credentials who, IRequest<TResponse> command)
    {
        using var scope = sp.CreateScope();
        scope.ServiceProvider.GetRequiredService<CurrentUser>().Credentials = who;
        return await scope.ServiceProvider.GetRequiredService<ISender>().Send(command);
    }

    private static Task<Policy> PolicyAsync(QuorumClient quorum, string requestType, IReadOnlyList<BypassRule>? bypass = null) =>
        quorum.As(Hq).CreatePolicyAsync(new PolicyInput
        {
            Name = requestType,
            RequestType = requestType,
            Stages = [new ApprovalStage { Index = 0, Name = "Finance", RequiredApprovals = 1, AllowedCheckerRoles = ["finance"] }],
            BypassRules = bypass,
        });

    private static (byte[] Body, string Signature) Delivery(WebhookEvent evt, Request request)
    {
        var payload = new WebhookPayload { Event = evt, Request = request, Approvals = request.Approvals, Timestamp = DateTimeOffset.UtcNow };
        var body = JsonSerializer.SerializeToUtf8Bytes(payload, QuorumJson.Options);
        return (body, Webhooks.Sign(body, Secret));
    }

    [Fact]
    public async Task CommandIsHeldUntilApprovedThenExecutedFromTheWebhook()
    {
        await using var sp = Build();
        var quorum = sp.GetRequiredService<QuorumClient>();
        var ledger = sp.GetRequiredService<Ledger>();
        await PolicyAsync(quorum, "submit_wire_transfer");

        var command = new SubmitWireTransfer("T-100", 5000);
        var pending = await SubmitAs(sp, Alice, command);
        Assert.Equal("submit_wire_transfer", pending.Type);
        Assert.Equal(RequestStatus.Pending, pending.Status);
        Assert.Equal("alice", pending.MakerId);
        Assert.Equal(command, pending.PayloadAs<SubmitWireTransfer>(new JsonSerializerOptions { PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower }));
        Assert.Empty(ledger.Executed);

        var again = await SubmitAs(sp, Alice, command);
        Assert.Equal(pending.Id, again.Id);

        var direct = await Assert.ThrowsAsync<ApprovalRequiredException>(() => SendAs(sp, Alice, command));
        Assert.Equal("submit_wire_transfer", direct.RequestType);
        Assert.Empty(ledger.Executed);

        var approved = await quorum.As(Bob).ApproveAsync(pending.Id, "ok");
        Assert.Equal(RequestStatus.Approved, approved.Status);

        var dispatcher = sp.GetRequiredService<ApprovalDispatcher>();
        var (body, signature) = Delivery(WebhookEvent.Approved, approved);
        var result = await dispatcher.DispatchAsync(new MemoryStream(body), signature);

        Assert.Equal(DispatchOutcome.Executed, result.Outcome);
        Assert.Equal("executed T-100", result.Result);
        var (executed, context) = Assert.Single(ledger.Executed);
        Assert.Equal(command, executed);
        Assert.Equal(approved.Id, context!.Id);
        var decided = Assert.Single(ledger.Decided);
        Assert.Equal("executed T-100", decided.Result);
        Assert.Equal(command, decided.Command);

        var tampered = Encoding.UTF8.GetBytes(Encoding.UTF8.GetString(body).Replace("5000", "9000"));
        await Assert.ThrowsAsync<InvalidSignatureException>(() => dispatcher.DispatchAsync(new MemoryStream(tampered), signature));
        Assert.Single(ledger.Executed);
    }

    [Fact]
    public async Task RejectedCommandNeverRuns()
    {
        await using var sp = Build();
        var quorum = sp.GetRequiredService<QuorumClient>();
        var ledger = sp.GetRequiredService<Ledger>();
        await PolicyAsync(quorum, "close_account");

        var pending = await SubmitAs(sp, Alice, new CloseAccount("A-9"));
        var rejected = await quorum.As(Bob).RejectAsync(pending.Id, "no");

        var (body, signature) = Delivery(WebhookEvent.Rejected, rejected);
        var result = await sp.GetRequiredService<ApprovalDispatcher>().DispatchAsync(new MemoryStream(body), signature);

        Assert.Equal(DispatchOutcome.Declined, result.Outcome);
        Assert.Empty(ledger.Executed);
        var decided = Assert.Single(ledger.Decided);
        Assert.Equal(RequestStatus.Rejected, decided.Request.Status);
        Assert.Null(decided.Result);
    }

    [Fact]
    public async Task BypassedCommandStillRunsFromTheWebhook()
    {
        await using var sp = Build();
        var quorum = sp.GetRequiredService<QuorumClient>();
        var ledger = sp.GetRequiredService<Ledger>();
        await PolicyAsync(quorum, "petty_expense",
            [new BypassRule { Name = "petty cash", Payload = [new BypassCondition { Path = "amount", Op = BypassOp.Lt, Value = 50 }] }]);

        var bypassed = await SubmitAs(sp, Alice, new FileExpense("E-1", 20));
        Assert.Equal(RequestStatus.Approved, bypassed.Status);
        Assert.Equal("petty_expense", bypassed.Type);
        Assert.Empty(ledger.Executed);

        var (body, signature) = Delivery(WebhookEvent.Approved, await quorum.As(Alice).GetRequestAsync(bypassed.Id));
        var result = await sp.GetRequiredService<ApprovalDispatcher>().DispatchAsync(new MemoryStream(body), signature);
        Assert.Equal(DispatchOutcome.Executed, result.Outcome);
        Assert.Equal("executed E-1", result.Result);
        var (_, context) = Assert.Single(ledger.Executed);
        Assert.Equal(bypassed.Id, context!.Id);

        var pending = await SubmitAs(sp, Alice, new FileExpense("E-2", 500));
        Assert.Equal(RequestStatus.Pending, pending.Status);
        Assert.Single(ledger.Executed);
    }

    [Fact]
    public async Task UnmarkedCommandsAndForeignDeliveriesPassThrough()
    {
        await using var sp = Build();
        Assert.Equal("pong hi", await SendAs(sp, Alice, new Ping("hi")));
        await Assert.ThrowsAsync<ArgumentException>(() => SubmitAs(sp, Alice, new Ping("hi")));

        var quorum = sp.GetRequiredService<QuorumClient>();
        await PolicyAsync(quorum, "not_a_command");
        var foreign = await quorum.As(Alice).CreateRequestAsync(new CreateRequest { Type = "not_a_command", Payload = new { amount = 1 } });
        var decided = await quorum.As(Bob).ApproveAsync(foreign.Id);

        var (body, signature) = Delivery(WebhookEvent.Approved, decided);
        var result = await sp.GetRequiredService<ApprovalDispatcher>().DispatchAsync(new MemoryStream(body), signature);
        Assert.Equal(DispatchOutcome.Ignored, result.Outcome);
        Assert.Empty(sp.GetRequiredService<Ledger>().Executed);
    }
}
