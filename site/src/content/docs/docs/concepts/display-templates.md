---
title: Display templates
description: Define how a request payload is presented to human reviewers, with labelled fields, formatters, repeating items and before/after change tables.
sidebar:
  order: 5
---

By default, reviewers see the raw JSON payload, which often contains system IDs and machine-oriented data. Display templates let you define how a payload should be presented to human reviewers.

## How it works

1. Define a `display_template` on your policy with field labels, paths into the payload, and optional formatters.
2. When a request is created, Quorum resolves the template against the payload and stores the result in `metadata.display`.
3. The widgets and console render the resolved fields as a clean label-value view instead of raw JSON.

Templates are resolved once at creation time, so editing a policy template only affects future requests.

## Template format

```json
{
  "title": "Wire Transfer - {{amount | currency}}",
  "fields": [
    { "label": "From Account", "path": "source_account_id" },
    { "label": "Amount", "path": "amount", "format": "currency" },
    { "label": "Destination", "path": "destination" }
  ],
  "items": {
    "path": "profiles",
    "label_path": "name",
    "fields": [
      { "label": "Email", "path": "email" },
      { "label": "Role", "path": "role" }
    ]
  }
}
```

- **`title`**: interpolated string using `{{path}}` or `{{path | format}}` placeholders.
- **`fields`**: top-level label-value pairs extracted from the payload via dot-notation paths.
- **`items`**: optional repeating section for batch or list payloads, such as multiple profiles in one request.
- **Built-in formatters:** `currency` ($1,234.56), `date` (Mar 14, 2026), `number` (1,000,000), `truncate` (50 chars max).
- **Missing values** fall back to `"-"`.

**Consumer override:** if the request already includes `metadata.display` at creation time, it takes precedence over the policy template. This lets consumers provide custom display data for edge cases.

## Proposed changes (before / after)

Many approvals are not about a new thing but about a change to an existing one: a branch's transaction limit, a user's role, a product's price. For those, the reviewer needs to see what is there now next to what is being proposed. Put both snapshots in the payload and add a `changes` block to the template:

```json
{
  "title": "Update Branch {{current.code}}",
  "changes": {
    "current_path": "current",
    "proposed_path": "proposed",
    "fields": [
      { "label": "Name", "path": "name" },
      { "label": "Daily Limit", "path": "daily_limit", "format": "currency" },
      { "label": "Status", "path": "status" }
    ]
  }
}
```

With a payload such as:

```json
{
  "current":  { "code": "LAG-01", "name": "Lagos Island", "daily_limit": 500000, "status": "active" },
  "proposed": { "code": "LAG-01", "name": "Lagos Island", "daily_limit": 750000, "status": "inactive" }
}
```

Quorum resolves `metadata.display.changes` to one row per field with `from`, `to`, and a `changed` flag. The approval panel widget and the console render these as a before/after table with changed values highlighted, and show how many fields differ.

- **`current_path` / `proposed_path`**: dot-notation paths to the two snapshots inside the payload. Omit the current snapshot for a creation, or the proposed one for a deletion; the missing side renders as `"-"`.
- **`fields`**: optional. When omitted, Quorum compares every leaf attribute found in either snapshot, sorted by path, with labels derived from the path (`billing.account_id` becomes `Billing / Account Id`). List fields explicitly to control order, labels, and formatting.
- **`changed`** compares the raw values, not the formatted strings.
- `changes` can be combined with `title`, `fields`, and `items` in the same template.

Your webhook consumer receives the full request, so on approval it can read the `proposed` snapshot from the payload and apply it. Quorum decides; your application performs the write.
