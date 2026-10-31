---
title: Demo and sample apps
description: Three sample applications on Go, Node.js and Python share one Quorum instance. Try them live or run the whole demo locally with Docker Compose.
---

## Live demo

Three sample applications share one hosted Quorum instance. Each has a profile switcher in the navigation bar, so you can create a request as a maker and then switch to a reviewer to approve or reject it.

| App | Tech | Scenario |
|-----|------|----------|
| [Banking](https://quorum.olawalelawal.dev/demo/banking) | Go + html/template | Multi-stage wire transfer approval with webhook callbacks |
| [Expense tracker](https://quorum.olawalelawal.dev/demo/expenses) | Node.js + Express | Single-stage expense approval with status polling |
| [Access portal](https://quorum.olawalelawal.dev/demo/access) | Python + Flask | Threshold-based security review (2-of-3 voting) |

The form on the [landing page](/) runs against the same instance, in its own `landing` tenant: each visitor opens a wire transfer and signs it as four different people, and the server's guards answer every click. The demo is reset from time to time, and anything you create there is public.

## Run the demo locally

```bash
docker compose --profile demo up
```

This starts everything: PostgreSQL, Quorum with the admin console, the policies each app needs (via `scripts/setup.sh`), seed data, and all three sample apps on `localhost:3001`, `3002` and `3003`. The console is at [localhost:8080/console/](http://localhost:8080/console/), signed in as `admin` / `admin123`.

## What each app demonstrates

- **Banking**: a webhook-driven flow on the [Go client](/docs/clients/go/). Transfers execute automatically when Quorum sends an approved webhook, verified with `client.ParseWebhook`.
- **Expense tracker**: a polling-based flow. The detail page fetches the latest status from Quorum on each page load.
- **Access portal**: threshold voting. Two of three security reviewers must approve, and not every rejection is fatal.

Each app embeds all three Quorum [widgets](/docs/integrate/widgets/): `<quorum-request-list>` on the dashboard as an approval queue, `<quorum-stage-progress>` on detail pages for stage visualisation, and `<quorum-approval-panel>` for approve and reject actions. Clicking a row in the request list navigates to the matching local detail page.

The source for all three is under [`examples/`](https://github.com/lawale/quorum/tree/master/examples) in the repository.

## Seed data

Quorum ships with a seed script that populates a fresh instance with sample policies, requests, and approvals:

```bash
# Against a running Quorum server
make seed

# Or via Docker Compose
docker compose --profile seed up
```

The seed script creates:

- **4 policies**: Expense Approval, Wire Transfer, System Access, Account Closure
- **6 requests** by different makers (alice, bob, charlie, dave, eve, frank)
- **Sample approvals**: 2 approved, 1 advanced to stage 2, 3 pending
- **1 webhook** (httpbin.org for testing)
- **Console admin**: `admin` / `admin123`
