---
title: Reviewer scoping
description: Restrict each approval stage to the right reviewers by identity attribute, such as the manager of the same branch, with rules that fail closed.
sidebar:
  order: 4
---

Roles say *what kind* of person may approve. Scope says *which* of them: the manager of the same branch, a lead in the same department, the owner of the affected cost centre. Without it, every manager in the organisation can act on every branch's requests.

## 1. Tell Quorum which identity attributes to read

In `trust` mode they come from headers; in `verify` mode from JWT claims. The names you choose here are the names policies refer to.

```yaml
auth:
  trust:
    attribute_headers:
      branch_id: "X-User-Branch"
  verify:
    claims:
      attributes:
        branch_id: "branch_id"           # or a dotted path such as org.branch.id
```

Only scalar values are read. Quorum captures the maker's attributes on every request it creates and stores them as `maker_attributes`.

## 2. Add `scope` rules to a stage

```json
{
  "index": 0,
  "name": "Branch Manager",
  "required_approvals": 1,
  "allowed_checker_roles": ["branch_manager"],
  "rejection_policy": "any",
  "scope": [
    { "attribute": "branch_id" }
  ]
}
```

This stage now accepts a checker only if their `branch_id` equals the maker's `branch_id`.

A rule names an **attribute** and one or more **sources** of acceptable values. The checker passes the rule when their attribute equals any value the sources produce. Every rule on a stage must pass.

- **`attribute`**: the identity attribute name, as configured above. Required.
- **`match_maker`**: accept the maker's attribute of the same name, captured when the request was created. This is the default when no other source is given.
- **`payload_path`**: accept the value at a dot-notation path in the request payload, for example `"destination_branch_id"`. Use this when a stage belongs to a branch other than the maker's.
- **`values`**: accept any of a fixed list, for example `["HQ"]`.
- **`allow_unscoped_checkers`**: when `true`, a checker who has no value for the attribute at all may act on any request. A checker who *does* have the attribute is still held to it. Defaults to `false`.

## Requests that travel between branches

Because scope is per stage, a request can travel between branches:

```json
"stages": [
  { "index": 0, "name": "Originating branch", "required_approvals": 1, "rejection_policy": "any",
    "scope": [{ "attribute": "branch_id" }] },
  { "index": 1, "name": "Receiving branch or head office", "required_approvals": 1, "rejection_policy": "any",
    "scope": [{ "attribute": "branch_id", "payload_path": "destination_branch_id", "values": ["HQ"] }] }
]
```

Stage 0 is reviewed in the maker's branch. Stage 1 is reviewed by the branch named in the payload, or by head office.

## Validation and failure modes

Attribute names in a policy must match the names configured in step 1. Saving a policy whose scope rules reference an attribute the active auth mode does not supply still succeeds, but the response carries a `warnings` array (and the console shows it) explaining that the stage can never be approved as configured.

Rules fail closed: if no source yields a value, because the maker had no such attribute and the payload path is missing, the checker is out of scope. Out-of-scope decisions return `403`, and `viewer_can_act` on a request reflects the scope check so widgets hide the buttons.

Both sets of attributes are also passed to the [dynamic authorization URL](/docs/concepts/permission-guards/#dynamic-authorization-check) as `maker_attributes` and `checker_attributes`, so an external check can build on them. The `review_requested` [webhook](/docs/integrate/webhooks/) carries the scope with its values already resolved, so your application can find the reviewers to notify.
