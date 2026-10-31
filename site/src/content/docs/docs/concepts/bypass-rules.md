---
title: Bypass rules
description: Declare which requests need no review, such as a root admin's own changes or transfers under a threshold, with velocity limits that stop a threshold from being gamed.
sidebar:
  order: 3
---

Not everything needs a second pair of eyes. A root administrator correcting a branch address, or a transfer under a small threshold, usually goes straight through in a hand-rolled maker-checker, and that exemption lives in code where nobody can see it. Quorum makes it policy.

```json
"bypass_rules": [
  {
    "name": "root admin",
    "maker_roles": ["root_admin"]
  },
  {
    "name": "small internal transfer",
    "maker_attributes": { "branch_id": ["HQ"] },
    "payload": [
      { "path": "amount",   "op": "lt", "value": 1000 },
      { "path": "currency", "op": "in", "value": ["NGN", "GHS"] }
    ]
  }
]
```

When a request is created, Quorum tries the rules in order. A rule matches when **every** condition it specifies holds:

- **`maker_roles`**: the maker holds at least one of these roles.
- **`maker_permissions`**: the maker holds at least one of these permissions.
- **`maker_attributes`**: for each attribute listed, the maker's value is one of the allowed values. Attributes come from the same configuration as [Reviewer scoping](/docs/concepts/reviewer-scoping/).
- **`payload`**: every condition is true for the request payload. `op` is one of `eq`, `ne`, `lt`, `lte`, `gt`, `gte`, `in`. Ordering operators compare numbers only; `in` takes an array. A missing path never satisfies a condition, including `ne`.

A rule must specify at least one condition; an empty rule is rejected, since it would bypass everything.

## Velocity limits

A threshold on its own invites structuring: anyone who learns that transfers under 1,000 skip review will send 900 five times. Give the rule `limits` and it stops matching once recent activity is too high:

```json
{
  "name": "small internal transfer",
  "payload": [ { "path": "amount", "op": "lt", "value": 1000 } ],
  "limits": [
    { "window": "24h", "max_count": 3 },
    { "window": "24h", "max_sum": { "path": "amount", "value": 2500 } },
    { "window": "7d",  "max_sum": { "path": "amount", "value": 5000 }, "group_by": ["payload:destination_account"] }
  ]
}
```

- **`window`**: how far back to look. Go durations such as `24h` or `90m`, plus a `d` suffix for days.
- **`max_count`**: the most requests allowed in the window, counting the one being created.
- **`max_sum`**: the most a numeric payload field may total across the window, again including the new request. Numeric strings are accepted.
- **`group_by`**: what makes two requests "the same activity". Defaults to `["maker"]`. Add `payload:<path>` entries to group by a payload value instead of, or as well as, the maker. The third limit above catches five different people each sending 900 to one destination account.

Every request of the policy's type in the window counts, whatever its status and whether or not it was bypassed. When any limit would be exceeded the rule does not match and Quorum tries the next rule; if none matches, the request goes through the normal stages. Bypass fails safe into review, never into rejection, and a database error during the check is treated the same way.

When a limit is what sent a request to review, Quorum writes `metadata.bypass_limit_exceeded` with the rule, window, measure, observed value and limit, and records a `bypass_limit_exceeded` audit entry. The console and the approval panel widget show this as a notice above the request, so the reviewer knows they are looking at the fourth transfer today rather than just another 900.

Limits make a bypass threshold safe to offer. They are not transaction monitoring: patterns across request types or tenants, or over months, belong in the AML tooling behind your application. Quorum's job is to make sure that when the pattern is visible at approval time, a human sees it.

## What happens on a match

On the first match the request is created and approved in one step. No approval rows are written. The audit log records `created` followed by `bypassed` with the rule name, the usual `approved` [webhooks](/docs/integrate/webhooks/) fire, and SSE subscribers are notified. The response to the create call already carries `"status": "approved"`, so your application can act immediately. Requests that match no rule proceed through the stages as normal.

Bypass applies to the whole policy, not to a stage. If what you want is "head office may approve any branch's request", that is a [scope](/docs/concepts/reviewer-scoping/) rule on the stage, not a bypass.
