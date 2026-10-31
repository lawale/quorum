// Functional tests against the real Quorum server (see server.ts). Every
// request and response is validated against api/openapi.yaml on the way
// through, and the run fails if any documented operation is never called.
import { describe, expect, inject, test } from "vitest";
import {
  InvalidSignatureError,
  QuorumClient,
  QuorumError,
  isBadRequest,
  isConflict,
  isForbidden,
  isNotFound,
  isTerminalEvent,
  isUnauthorized,
  parseWebhook,
  sign,
  type Identity,
  type StreamEvent,
  type WebhookPayload,
} from "../src/index.js";

const tenant = "banking";
const alice: Identity = { userId: "alice", roles: ["teller"], attributes: { branch_id: "LAG-01" } };
const bob: Identity = { userId: "bob", roles: ["finance"], attributes: { branch_id: "LAG-01" } };
const carol: Identity = { userId: "carol", roles: ["manager"], attributes: { branch_id: "ABJ-02" } };
const hq: Identity = { userId: "hq", roles: ["manager"] };

// One server serves the whole file, so each test uses its own request type.
function newClient(): QuorumClient {
  return new QuorumClient(inject("quorumUrl"), {
    tenant,
    trustHeaders: { attributes: { branch_id: "X-User-Branch" } },
  });
}

async function wirePolicy(c: QuorumClient, requestType: string, stages?: Parameters<QuorumClient["createPolicy"]>[0]["stages"]) {
  return c.as(hq).createPolicy({
    name: `Policy for ${requestType}`,
    request_type: requestType,
    stages: stages ?? [
      { index: 0, name: "Finance", required_approvals: 1, allowed_checker_roles: ["finance"] },
      { index: 1, name: "Management", required_approvals: 1, allowed_checker_roles: ["manager"] },
    ],
    identity_fields: ["transfer_id"],
    auto_expire_duration: "24h",
  });
}

async function rejects(p: Promise<unknown>, check: (err: unknown) => boolean, what: string): Promise<void> {
  try {
    await p;
  } catch (err) {
    if (!check(err)) throw new Error(`${what}: unexpected error ${String(err)}`);
    return;
  }
  throw new Error(`${what}: expected an error`);
}

describe("wire transfer", () => {
  test("goes through both stages", async () => {
    const c = newClient();
    await wirePolicy(c, "wire_transfer");

    const created = await c.as(alice).createRequest({
      type: "wire_transfer",
      payload: { transfer_id: "T-1", amount: 5000 },
      idempotencyKey: "T-1",
    });
    expect(created.status).toBe("pending");
    expect(created.maker_id).toBe("alice");
    expect(created.tenant_id).toBe(tenant);
    expect(created.maker_attributes?.["branch_id"]).toBe("LAG-01");

    const replay = await c.as(alice).createRequest({
      type: "wire_transfer",
      payload: { transfer_id: "T-1", amount: 5000 },
      idempotencyKey: "T-1",
    });
    expect(replay.id).toBe(created.id);

    // Same identity fields, no idempotency key: a duplicate pending request.
    await rejects(
      c.as(alice).createRequest({ type: "wire_transfer", payload: { transfer_id: "T-1", amount: 1 } }),
      isConflict,
      "duplicate pending request",
    );

    const asBob = await c.as(bob).getRequest(created.id);
    expect(asBob.viewer_can_act).toBe(true);
    expect(asBob.total_stages).toBe(2);

    await rejects(c.as(alice).approve(created.id), isForbidden, "self-approval");
    await rejects(c.as(carol).approve(created.id), isForbidden, "manager at finance stage");

    const afterBob = await c.as(bob).approve(created.id, "looks right");
    expect(afterBob.status).toBe("pending");
    expect(afterBob.current_stage).toBe(1);
    await rejects(c.as(bob).approve(created.id), isForbidden, "bob again at management stage");

    const final = await c.as(carol).approve(created.id);
    expect(final.status).toBe("approved");
    expect(final.approvals).toHaveLength(2);
    expect(final.approvals?.[0]?.comment).toBe("looks right");

    await rejects(c.as(hq).approve(created.id), isConflict, "approve after terminal");

    const trail = await c.as(alice).audit(created.id);
    expect(trail.map((l) => l.action)).toEqual(["created", "approved", "stage_advanced", "approved"]);

    const approved = await c.as(hq).listRequests({ status: "approved", type: "wire_transfer" });
    expect(approved.total).toBe(1);
    expect(approved.data[0]?.id).toBe(created.id);
    const pending = await c.as(hq).listRequests({ status: "pending", type: "wire_transfer" });
    expect(pending.total).toBe(0);
    expect(pending.data).toEqual([]);

    await rejects(c.as(alice).getRequest(crypto.randomUUID()), isNotFound, "unknown request");
  });

  test("can be rejected or cancelled", async () => {
    const c = newClient();
    await wirePolicy(c, "refund");

    const rejected = await c.as(alice).createRequest({ type: "refund", payload: { transfer_id: "R-1", amount: 10 } });
    const res = await c.as(bob).reject(rejected.id, "wrong account");
    expect(res.status).toBe("rejected");

    const cancelled = await c.as(alice).createRequest({ type: "refund", payload: { transfer_id: "R-2", amount: 10 } });
    await rejects(c.as(bob).cancel(cancelled.id), isForbidden, "cancel by non-maker");
    const done = await c.as(alice).cancel(cancelled.id);
    expect(done.status).toBe("cancelled");
    await rejects(c.as(alice).cancel(cancelled.id), isConflict, "cancel twice");
  });

  test("branch scope follows the maker", async () => {
    const c = newClient();
    await wirePolicy(c, "branch_transfer", [
      {
        index: 0,
        required_approvals: 1,
        allowed_checker_roles: ["finance", "manager"],
        scope: [{ attribute: "branch_id", match_maker: true, allow_unscoped_checkers: true }],
      },
    ]);

    const req = await c.as(alice).createRequest({ type: "branch_transfer", payload: { transfer_id: "B-1", amount: 10 } });

    await rejects(c.as(carol).approve(req.id), isForbidden, "manager from another branch");
    const seen = await c.as(carol).getRequest(req.id);
    expect(seen.viewer_can_act).not.toBe(true);

    // Head office carries no branch at all, which the rule allows.
    const res = await c.as(hq).approve(req.id);
    expect(res.status).toBe("approved");
  });
});

describe("waitForDecision", () => {
  test("follows the event stream and honours the caller's deadline", async () => {
    const c = newClient();
    await wirePolicy(c, "stream_transfer", [{ index: 0, required_approvals: 1, allowed_checker_roles: ["finance"] }]);

    const req = await c.as(alice).createRequest({ type: "stream_transfer", payload: { transfer_id: "S-1", amount: 10 } });

    const waiting = c.as(alice).waitForDecision(req.id, { signal: AbortSignal.timeout(10_000), pollIntervalMs: 50 });
    // Let the waiter subscribe, then decide.
    await new Promise((r) => setTimeout(r, 100));
    await c.as(bob).approve(req.id);
    const decided = await waiting;
    expect(decided.status).toBe("approved");

    // Already decided: returns at once without opening a stream.
    const again = await c.as(alice).waitForDecision(req.id);
    expect(again.status).toBe("approved");

    // Still pending and nobody decides: the caller's deadline wins.
    const pending = await c.as(alice).createRequest({ type: "stream_transfer", payload: { transfer_id: "S-2", amount: 10 } });
    await expect(c.as(alice).waitForDecision(pending.id, { signal: AbortSignal.timeout(300) })).rejects.toMatchObject({
      name: "TimeoutError",
    });

    // The raw stream closes itself for a terminal request after one status event.
    const events: StreamEvent[] = [];
    for await (const ev of c.as(alice).events(req.id)) events.push(ev);
    expect(events).toHaveLength(1);
    expect(events[0]!.status).toBe("approved");
    expect(isTerminalEvent(events[0]!)).toBe(true);
  });
});

describe("policies", () => {
  test("lifecycle including bypass rules", async () => {
    const c = newClient().as(hq);

    const p = await c.createPolicy({
      name: "Expenses",
      request_type: "expense",
      stages: [{ index: 0, required_approvals: 1, allowed_checker_roles: ["manager"] }],
      bypass_rules: [
        {
          name: "petty cash",
          payload: [{ path: "amount", op: "lt", value: 50 }],
          limits: [{ window: "1d", max_count: 3 }],
        },
      ],
      auto_expire_duration: "72h",
    });
    expect(p.auto_expire_duration).toBe("72h");

    await rejects(
      c.createPolicy({ name: "Again", request_type: "expense", stages: p.stages }),
      isConflict,
      "second policy for same type",
    );

    await rejects(
      c.createPolicy({ name: "Bad", request_type: "bad", stages: [{ index: 0, required_approvals: 1, rejection_policy: "threshold" }] }),
      (err) => err instanceof QuorumError && err.status === 400 && err.detail.includes("max_checkers"),
      "invalid policy",
    );

    // Bypass rule in effect: small expense skips review.
    const small = await c.as(alice).createRequest({ type: "expense", payload: { amount: 20 } });
    expect(small.status).toBe("approved");

    const got = await c.getPolicy(p.id);
    expect(got.name).toBe("Expenses");
    expect(got.bypass_rules).toHaveLength(1);

    const list = await c.listPolicies({ perPage: 100 });
    expect(list.data.some((x) => x.id === p.id)).toBe(true);

    const updated = await c.updatePolicy(p.id, { name: "Expenses v2", bypass_rules: [] });
    expect(updated.name).toBe("Expenses v2");
    expect(updated.bypass_rules ?? []).toHaveLength(0);
    expect(updated.stages).toHaveLength(1);

    // Bypass cleared: the same expense now waits for review.
    const again = await c.as(alice).createRequest({ type: "expense", payload: { amount: 20 } });
    expect(again.status).toBe("pending");

    await c.deletePolicy(p.id);
    await rejects(c.getPolicy(p.id), isNotFound, "get after delete");
    await rejects(c.deletePolicy(p.id), isNotFound, "delete twice");
  });
});

describe("webhooks", () => {
  test("lifecycle", async () => {
    const c = newClient().as(hq);

    await rejects(
      c.createWebhook({ url: "https://hooks.example.com/quorum", events: [], secret: "s3cret" }),
      isBadRequest,
      "webhook with no events",
    );

    const wh = await c.createWebhook({
      url: "https://hooks.example.com/quorum",
      events: ["approved", "rejected", "review_requested"],
      secret: "s3cret",
      request_type: "wire_transfer",
    });
    expect(wh.active).toBe(true);
    expect("secret" in wh).toBe(false);

    const list = await c.listWebhooks();
    expect(list.total).toBe(1);
    expect(list.data[0]?.id).toBe(wh.id);

    await c.deleteWebhook(wh.id);
    await rejects(c.deleteWebhook(wh.id), isNotFound, "delete twice");
  });

  test("parseWebhook accepts only genuine deliveries", async () => {
    const secret = "s3cret";
    const now = new Date().toISOString();
    const payload: WebhookPayload = {
      event: "review_requested",
      request: {
        id: crypto.randomUUID(),
        tenant_id: tenant,
        type: "wire_transfer",
        status: "pending",
        payload: { transfer_id: "T-9" },
        maker_id: "alice",
        current_stage: 0,
        created_at: now,
        updated_at: now,
      },
      review: {
        reason: "created",
        stage_index: 0,
        total_stages: 1,
        required_approvals: 1,
        eligible_reviewers: [],
        allowed_roles: ["finance"],
        scope: [{ attribute: "branch_id", values: ["LAG-01"] }],
        excluded_user_ids: ["alice"],
      },
      timestamp: now,
    };
    const body = JSON.stringify(payload);

    const got = await parseWebhook(body, await sign(body, secret), secret);
    expect(got.event).toBe("review_requested");
    expect(got.request.id).toBe(payload.request.id);
    expect(got.review?.scope[0]?.values[0]).toBe("LAG-01");

    // Bytes verify the same as text, as a framework handing over a Buffer would.
    await expect(parseWebhook(new TextEncoder().encode(body), await sign(body, secret), secret)).resolves.toBeTruthy();

    const tampered = body.replace('"alice"', '"mallory"');
    await expect(parseWebhook(tampered, await sign(body, secret), secret)).rejects.toBeInstanceOf(InvalidSignatureError);
    await expect(parseWebhook(body, await sign(body, "other"), secret)).rejects.toBeInstanceOf(InvalidSignatureError);
    await expect(parseWebhook(body, "", secret)).rejects.toBeInstanceOf(InvalidSignatureError);
    await expect(parseWebhook(body, null, secret)).rejects.toBeInstanceOf(InvalidSignatureError);
  });
});

describe("identity", () => {
  test("is required for everything but health", async () => {
    const c = newClient();
    await rejects(c.listRequests(), isUnauthorized, "no credentials");

    const h = await c.health();
    expect(h.status).toBe("healthy");
    expect(h.version).toBe("test");
  });

  test("bearer tokens are sent as Authorization", async () => {
    // The test server trusts headers, so a bearer token alone is anonymous.
    const c = newClient().as({ token: "not-a-real-jwt" });
    await rejects(c.listRequests(), isUnauthorized, "bearer against a trust-mode server");
  });
});
