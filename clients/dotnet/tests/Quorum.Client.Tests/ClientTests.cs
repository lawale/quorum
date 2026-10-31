using System.Text.Json;

namespace Quorum.Client.Tests;

[Collection(nameof(ApiServerCollection))]
public sealed class ClientTests(ApiServerFixture server)
{
    private const string Tenant = "banking";

    private static readonly Identity Alice = new() { UserId = "alice", Roles = ["teller"], Attributes = new Dictionary<string, string> { ["branch_id"] = "LAG-01" } };
    private static readonly Identity Bob = new() { UserId = "bob", Roles = ["finance"], Attributes = new Dictionary<string, string> { ["branch_id"] = "LAG-01" } };
    private static readonly Identity Carol = new() { UserId = "carol", Roles = ["manager"], Attributes = new Dictionary<string, string> { ["branch_id"] = "ABJ-02" } };
    private static readonly Identity Hq = new() { UserId = "hq", Roles = ["manager"] };

    private sealed record Transfer(string TransferId, int Amount);

    // One server serves the whole class, so each test uses its own request type.
    private QuorumClient NewClient() => new(server.Url, o =>
    {
        o.Tenant = Tenant;
        o.TrustHeaders = new TrustHeaderNames { Attributes = new Dictionary<string, string> { ["branch_id"] = "X-User-Branch" } };
        o.PayloadSerializerOptions = new JsonSerializerOptions { PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower };
        o.StrictModels = true;
    });

    private static Task<Policy> WirePolicyAsync(QuorumClient c, string requestType, IReadOnlyList<ApprovalStage>? stages = null) =>
        c.As(Hq).CreatePolicyAsync(new PolicyInput
        {
            Name = $"Policy for {requestType}",
            RequestType = requestType,
            Stages = stages ??
            [
                new ApprovalStage { Index = 0, Name = "Finance", RequiredApprovals = 1, AllowedCheckerRoles = ["finance"] },
                new ApprovalStage { Index = 1, Name = "Management", RequiredApprovals = 1, AllowedCheckerRoles = ["manager"] },
            ],
            IdentityFields = ["transfer_id"],
            AutoExpireDuration = Duration.Format(TimeSpan.FromHours(24)),
        });

    private static async Task<QuorumException> Rejects(Func<Task> call)
    {
        var ex = await Assert.ThrowsAsync<QuorumException>(call);
        return ex;
    }

    [Fact]
    public async Task WireTransferGoesThroughBothStages()
    {
        var c = NewClient();
        await WirePolicyAsync(c, "wire_transfer");

        var created = await c.As(Alice).CreateRequestAsync(new CreateRequest
        {
            Type = "wire_transfer",
            Payload = new Transfer("T-1", 5000),
            IdempotencyKey = "T-1",
        });
        Assert.Equal(RequestStatus.Pending, created.Status);
        Assert.Equal("alice", created.MakerId);
        Assert.Equal(Tenant, created.TenantId);
        Assert.Equal("LAG-01", created.MakerAttributes?["branch_id"]);
        Assert.Equal(new Transfer("T-1", 5000), created.PayloadAs<Transfer>(new JsonSerializerOptions { PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower }));

        var replay = await c.As(Alice).CreateRequestAsync(new CreateRequest { Type = "wire_transfer", Payload = new Transfer("T-1", 5000), IdempotencyKey = "T-1" });
        Assert.Equal(created.Id, replay.Id);

        Assert.True((await Rejects(() => c.As(Alice).CreateRequestAsync(new CreateRequest { Type = "wire_transfer", Payload = new Transfer("T-1", 1) }))).IsConflict);

        var asBob = await c.As(Bob).GetRequestAsync(created.Id);
        Assert.True(asBob.ViewerCanAct);
        Assert.Equal(2, asBob.TotalStages);

        Assert.True((await Rejects(() => c.As(Alice).ApproveAsync(created.Id))).IsForbidden);
        Assert.True((await Rejects(() => c.As(Carol).ApproveAsync(created.Id))).IsForbidden);

        var afterBob = await c.As(Bob).ApproveAsync(created.Id, "looks right");
        Assert.Equal(RequestStatus.Pending, afterBob.Status);
        Assert.Equal(1, afterBob.CurrentStage);
        Assert.True((await Rejects(() => c.As(Bob).ApproveAsync(created.Id))).IsForbidden);

        var final = await c.As(Carol).ApproveAsync(created.Id);
        Assert.Equal(RequestStatus.Approved, final.Status);
        Assert.True(final.IsTerminal);
        Assert.Equal(2, final.Approvals!.Count);
        Assert.Equal("looks right", final.Approvals[0].Comment);
        Assert.Equal(Decision.Approved, final.Approvals[0].Decision);

        Assert.True((await Rejects(() => c.As(Hq).ApproveAsync(created.Id))).IsConflict);

        var trail = await c.As(Alice).AuditAsync(created.Id);
        Assert.Equal(["created", "approved", "stage_advanced", "approved"], trail.Select(l => l.Action));

        var approved = await c.As(Hq).ListRequestsAsync(new ListRequestsFilter { Status = RequestStatus.Approved, Type = "wire_transfer" });
        Assert.Equal(1, approved.Total);
        Assert.Equal(created.Id, approved.Data[0].Id);
        var pending = await c.As(Hq).ListRequestsAsync(new ListRequestsFilter { Status = RequestStatus.Pending, Type = "wire_transfer" });
        Assert.Equal(0, pending.Total);
        Assert.Empty(pending.Data);

        Assert.True((await Rejects(() => c.As(Alice).GetRequestAsync(Guid.NewGuid()))).IsNotFound);
    }

    [Fact]
    public async Task RequestsCanBeRejectedOrCancelled()
    {
        var c = NewClient();
        await WirePolicyAsync(c, "refund");

        var rejected = await c.As(Alice).CreateRequestAsync(new CreateRequest { Type = "refund", Payload = new Transfer("R-1", 10) });
        var res = await c.As(Bob).RejectAsync(rejected.Id, "wrong account");
        Assert.Equal(RequestStatus.Rejected, res.Status);

        var cancelled = await c.As(Alice).CreateRequestAsync(new CreateRequest { Type = "refund", Payload = new Transfer("R-2", 10) });
        Assert.True((await Rejects(() => c.As(Bob).CancelAsync(cancelled.Id))).IsForbidden);
        var done = await c.As(Alice).CancelAsync(cancelled.Id);
        Assert.Equal(RequestStatus.Cancelled, done.Status);
        Assert.True((await Rejects(() => c.As(Alice).CancelAsync(cancelled.Id))).IsConflict);
    }

    [Fact]
    public async Task BranchScopeFollowsTheMaker()
    {
        var c = NewClient();
        await WirePolicyAsync(c, "branch_transfer",
        [
            new ApprovalStage
            {
                Index = 0,
                RequiredApprovals = 1,
                AllowedCheckerRoles = ["finance", "manager"],
                Scope = [new ScopeRule { Attribute = "branch_id", MatchMaker = true, AllowUnscopedCheckers = true }],
            },
        ]);

        var req = await c.As(Alice).CreateRequestAsync(new CreateRequest { Type = "branch_transfer", Payload = new Transfer("B-1", 10) });

        Assert.True((await Rejects(() => c.As(Carol).ApproveAsync(req.Id))).IsForbidden);
        var seen = await c.As(Carol).GetRequestAsync(req.Id);
        Assert.NotEqual(true, seen.ViewerCanAct);

        // Head office carries no branch at all, which the rule allows.
        var res = await c.As(Hq).ApproveAsync(req.Id);
        Assert.Equal(RequestStatus.Approved, res.Status);
    }

    [Fact]
    public async Task WaitForDecisionFollowsTheStreamAndHonoursTheDeadline()
    {
        var c = NewClient();
        await WirePolicyAsync(c, "stream_transfer", [new ApprovalStage { Index = 0, RequiredApprovals = 1, AllowedCheckerRoles = ["finance"] }]);

        var req = await c.As(Alice).CreateRequestAsync(new CreateRequest { Type = "stream_transfer", Payload = new Transfer("S-1", 10) });

        using var waitCts = new CancellationTokenSource(TimeSpan.FromSeconds(10));
        var waiting = c.As(Alice).WaitForDecisionAsync(req.Id, TimeSpan.FromMilliseconds(50), waitCts.Token);
        await Task.Delay(100);
        await c.As(Bob).ApproveAsync(req.Id);
        var decided = await waiting;
        Assert.Equal(RequestStatus.Approved, decided.Status);

        var again = await c.As(Alice).WaitForDecisionAsync(req.Id);
        Assert.Equal(RequestStatus.Approved, again.Status);

        var pending = await c.As(Alice).CreateRequestAsync(new CreateRequest { Type = "stream_transfer", Payload = new Transfer("S-2", 10) });
        using var shortCts = new CancellationTokenSource(TimeSpan.FromMilliseconds(300));
        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => c.As(Alice).WaitForDecisionAsync(pending.Id, cancellationToken: shortCts.Token));

        var events = new List<StreamEvent>();
        await foreach (var ev in c.As(Alice).EventsAsync(req.Id)) events.Add(ev);
        var only = Assert.Single(events);
        Assert.Equal(req.Id, only.RequestId);
        Assert.Equal(RequestStatus.Approved, only.Status);
        Assert.True(only.IsTerminal);
    }

    [Fact]
    public async Task PolicyLifecycleIncludingBypassRules()
    {
        var c = NewClient().As(Hq);

        var p = await c.CreatePolicyAsync(new PolicyInput
        {
            Name = "Expenses",
            RequestType = "expense",
            Stages = [new ApprovalStage { Index = 0, RequiredApprovals = 1, AllowedCheckerRoles = ["manager"] }],
            BypassRules =
            [
                new BypassRule
                {
                    Name = "petty cash",
                    Payload = [new BypassCondition { Path = "amount", Op = BypassOp.Lt, Value = 50 }],
                    Limits = [new BypassLimit { Window = "1d", MaxCount = 3 }],
                },
            ],
            AutoExpireDuration = Duration.Format(TimeSpan.FromHours(72)),
        });
        Assert.Equal(TimeSpan.FromHours(72), Duration.Parse(p.AutoExpireDuration!));

        Assert.True((await Rejects(() => c.CreatePolicyAsync(new PolicyInput { Name = "Again", RequestType = "expense", Stages = p.Stages }))).IsConflict);

        var bad = await Rejects(() => c.CreatePolicyAsync(new PolicyInput
        {
            Name = "Bad",
            RequestType = "bad",
            Stages = [new ApprovalStage { Index = 0, RequiredApprovals = 1, RejectionPolicy = RejectionPolicy.Threshold }],
        }));
        Assert.True(bad.IsBadRequest);
        Assert.Contains("max_checkers", bad.Detail);

        var small = await c.As(Alice).CreateRequestAsync(new CreateRequest { Type = "expense", Payload = new { amount = 20 } });
        Assert.Equal(RequestStatus.Approved, small.Status);

        var got = await c.GetPolicyAsync(p.Id);
        Assert.Equal("Expenses", got.Name);
        Assert.Single(got.BypassRules!);
        Assert.Equal(BypassOp.Lt, got.BypassRules![0].Payload![0].Op);

        var list = await c.ListPoliciesAsync(new Paging(PerPage: 100));
        Assert.Contains(list.Data, x => x.Id == p.Id);

        var updated = await c.UpdatePolicyAsync(p.Id, new PolicyUpdate { Name = "Expenses v2", BypassRules = [] });
        Assert.Equal("Expenses v2", updated.Name);
        Assert.Empty(updated.BypassRules ?? []);
        Assert.Single(updated.Stages);

        var again = await c.As(Alice).CreateRequestAsync(new CreateRequest { Type = "expense", Payload = new { amount = 20 } });
        Assert.Equal(RequestStatus.Pending, again.Status);

        await c.DeletePolicyAsync(p.Id);
        Assert.True((await Rejects(() => c.GetPolicyAsync(p.Id))).IsNotFound);
        Assert.True((await Rejects(() => c.DeletePolicyAsync(p.Id))).IsNotFound);
    }

    [Fact]
    public async Task WebhookLifecycle()
    {
        var c = NewClient().As(Hq);

        Assert.True((await Rejects(() => c.CreateWebhookAsync(new WebhookInput { Url = new Uri("https://hooks.example.com/quorum"), Events = [], Secret = "s3cret" }))).IsBadRequest);

        var wh = await c.CreateWebhookAsync(new WebhookInput
        {
            Url = new Uri("https://hooks.example.com/quorum"),
            Events = [WebhookEvent.Approved, WebhookEvent.Rejected, WebhookEvent.ReviewRequested],
            Secret = "s3cret",
            RequestType = "wire_transfer",
        });
        Assert.True(wh.Active);
        Assert.Equal(["approved", "rejected", "review_requested"], wh.Events);

        var list = await c.ListWebhooksAsync();
        Assert.Equal(1, list.Total);
        Assert.Equal(wh.Id, list.Data[0].Id);

        await c.DeleteWebhookAsync(wh.Id);
        Assert.True((await Rejects(() => c.DeleteWebhookAsync(wh.Id))).IsNotFound);
    }

    [Fact]
    public async Task ParseWebhookAcceptsOnlyGenuineDeliveries()
    {
        const string secret = "s3cret";
        var now = DateTimeOffset.UtcNow;
        var payload = new WebhookPayload
        {
            Event = WebhookEvent.ReviewRequested,
            Request = new Request
            {
                Id = Guid.NewGuid(),
                TenantId = Tenant,
                Type = "wire_transfer",
                Status = RequestStatus.Pending,
                Payload = JsonSerializer.SerializeToElement(new { transfer_id = "T-9" }),
                MakerId = "alice",
                CurrentStage = 0,
                CreatedAt = now,
                UpdatedAt = now,
            },
            Review = new ReviewRequest
            {
                Reason = ReviewReason.Created,
                StageIndex = 0,
                TotalStages = 1,
                RequiredApprovals = 1,
                EligibleReviewers = [],
                AllowedRoles = ["finance"],
                Scope = [new ResolvedScope { Attribute = "branch_id", Values = ["LAG-01"] }],
                ExcludedUserIds = ["alice"],
            },
            Timestamp = now,
        };
        var body = JsonSerializer.Serialize(payload, QuorumJson.Options);

        var got = Webhooks.Parse(body, Webhooks.Sign(body, secret), secret);
        Assert.Equal(WebhookEvent.ReviewRequested, got.Event);
        Assert.Equal(payload.Request.Id, got.Request.Id);
        Assert.Equal(ReviewReason.Created, got.Review!.Reason);
        Assert.Equal("LAG-01", got.Review.Scope[0].Values[0]);

        using var stream = new MemoryStream(System.Text.Encoding.UTF8.GetBytes(body));
        Assert.Equal(payload.Request.Id, (await Webhooks.ParseAsync(stream, Webhooks.Sign(body, secret), secret)).Request.Id);

        var tampered = body.Replace("\"alice\"", "\"mallory\"");
        Assert.Throws<InvalidSignatureException>(() => Webhooks.Parse(tampered, Webhooks.Sign(body, secret), secret));
        Assert.Throws<InvalidSignatureException>(() => Webhooks.Parse(body, Webhooks.Sign(body, "other"), secret));
        Assert.Throws<InvalidSignatureException>(() => Webhooks.Parse(body, "", secret));
        Assert.Throws<InvalidSignatureException>(() => Webhooks.Parse(body, null, secret));
    }

    [Fact]
    public async Task IdentityIsRequiredForEverythingButHealth()
    {
        var c = NewClient();
        Assert.True((await Rejects(() => c.ListRequestsAsync())).IsUnauthorized);

        var h = await c.HealthAsync();
        Assert.Equal("healthy", h.Status);
        Assert.Equal("test", h.Version);

        // The test server trusts headers, so a bearer token alone is anonymous.
        Assert.True((await Rejects(() => c.As(new BearerToken("not-a-real-jwt")).ListRequestsAsync())).IsUnauthorized);
    }

    [Fact]
    public async Task DependencyInjectionProducesAWorkingClient()
    {
        var services = new Microsoft.Extensions.DependencyInjection.ServiceCollection();
        services.AddQuorumClient(o =>
        {
            o.BaseAddress = server.Url;
            o.Tenant = Tenant;
        });
        using var provider = Microsoft.Extensions.DependencyInjection.ServiceCollectionContainerBuilderExtensions.BuildServiceProvider(services);
        var c = Microsoft.Extensions.DependencyInjection.ServiceProviderServiceExtensions.GetRequiredService<QuorumClient>(provider);

        var h = await c.HealthAsync();
        Assert.Equal("test", h.Version);
        var page = await c.As(Hq).ListPoliciesAsync();
        Assert.True(page.Total >= 0);
    }
}
