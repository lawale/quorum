---
title: Health and metrics
description: The unauthenticated health endpoint with component status and release version, and Prometheus metrics for request volume, approval rates and webhook delivery.
sidebar:
  order: 5
---

## Health endpoint

The `/health` endpoint returns component-level health status, HTTP 200 when healthy and 503 when unhealthy, and does not require authentication. Use it for load balancer health checks and container orchestration readiness probes.

The response also carries `version`: the release tag the binary was built from (`"dev"` for local builds), so you can confirm which Quorum a deployment is running. The [Go](/docs/clients/go/), [TypeScript](/docs/clients/typescript/) and [.NET](/docs/clients/dotnet/) clients expose the same value, and each reports its own version in its User-Agent.

The binary doubles as a probe, which is how the Docker images define their healthcheck without a shell:

```bash
quorum -health http://localhost:8080
```

## Prometheus metrics

Enable metrics in config:

```yaml
metrics:
  enabled: true
  path: "/metrics"
```

Quorum exports request volume, approval and rejection rates, and webhook delivery outcomes, alongside the standard Go runtime metrics, for monitoring and alerting.

## Audit

Every action is permanently logged with actor, timestamp and details: `created`, `approved`, `rejected`, `cancelled`, `expired`, `bypassed`, `stage_advanced`, `bypass_limit_exceeded`, `webhook_sent` and `webhook_failed`. Read it per request at `/api/v1/requests/{id}/audit` or browse it in the [console](/docs/operate/console/). This is the history to hand to an auditor for SOC 2 or ISO 27001.
