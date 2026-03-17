<svelte:options customElement={{ tag: "quorum-stage-progress", shadow: "open" }} />

<script lang="ts">
  import { createClient, ApiError } from '../lib/api';
  import { getGlobalConfig, hasGlobalApiUrl } from '../lib/config';
  import type { SSEConnection } from '../lib/api';
  import type { Request, Policy, Approval } from '../lib/types';
  import StageBar from './internal/StageBar.svelte';
  import StatusBadge from './internal/StatusBadge.svelte';

  let {
    'request-id': requestId = '',
    'api-url': apiUrl = '',
    token = '',
    'tenant-id': tenantId = '',
    'auth-headers': authHeadersStr = '',
    'poll-interval': pollIntervalStr = '30000',
    'sse': sseStr = 'true',
  }: {
    'request-id'?: string;
    'api-url'?: string;
    token?: string;
    'tenant-id'?: string;
    'auth-headers'?: string;
    'poll-interval'?: string;
    'sse'?: string;
  } = $props();

  let req: Request | null = $state(null);
  let policy: Policy | null = $state(null);
  let approvals: Approval[] = $derived(req?.approvals ?? []);
  let error: string | null = $state(null);
  let loading = $state(true);
  let isPending = $state(false);
  let sseEnabled = $derived(sseStr !== 'false');
  let pollTimer: ReturnType<typeof setInterval> | undefined;
  let sseConnection: SSEConnection | null = null;
  let sseActive = false;

  function getClient() {
    let authHeaders: Record<string, string> | undefined;
    if (authHeadersStr) {
      try { authHeaders = JSON.parse(authHeadersStr); } catch {}
    }
    const global = getGlobalConfig();
    return createClient({
      ...global,
      ...(apiUrl ? { apiUrl } : {}),
      ...(token ? { token } : {}),
      ...(tenantId ? { tenantId } : {}),
      ...(authHeaders ? { authHeaders } : {}),
    });
  }

  async function load() {
    if (!requestId || (!apiUrl && !hasGlobalApiUrl())) return;
    loading = true;
    error = null;
    try {
      const client = getClient();
      req = await client.getRequest(requestId);
      policy = await client.getPolicyByType(req.type);
      isPending = req.status === 'pending';
      if (!isPending) {
        closeSSE();
        stopPolling();
      }
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Failed to load';
    } finally {
      loading = false;
    }
  }

  function startPolling() {
    if (pollTimer) clearInterval(pollTimer);
    const interval = parseInt(pollIntervalStr) || 30000;
    if (interval > 0) {
      pollTimer = setInterval(() => load(), interval);
    }
  }

  function stopPolling() {
    if (pollTimer) { clearInterval(pollTimer); pollTimer = undefined; }
  }

  function closeSSE() {
    if (sseConnection) { sseConnection.close(); sseConnection = null; }
    sseActive = false;
  }

  $effect(() => {
    if (requestId && apiUrl) load();
  });

  $effect(() => {
    function onConfigured() {
      if (requestId && !apiUrl && hasGlobalApiUrl()) load();
    }
    document.addEventListener('quorum:configured', onConfigured);
    return () => document.removeEventListener('quorum:configured', onConfigured);
  });

  // Listen for ApprovalPanel events (when both widgets are on the same page)
  $effect(() => {
    function onQuorumEvent(e: Event) {
      const detail = (e as CustomEvent).detail;
      if (detail?.requestId === requestId) load();
    }
    document.addEventListener('quorum:approved', onQuorumEvent);
    document.addEventListener('quorum:rejected', onQuorumEvent);
    return () => {
      document.removeEventListener('quorum:approved', onQuorumEvent);
      document.removeEventListener('quorum:rejected', onQuorumEvent);
    };
  });

  // SSE-first with polling fallback (same pattern as ApprovalPanel)
  $effect(() => {
    closeSSE();
    stopPolling();

    if (!isPending) return;

    if (sseEnabled && requestId && apiUrl) {
      try {
        const client = getClient();
        sseConnection = client.connectSSE(
          requestId,
          () => load(),
          () => {
            sseActive = false;
            sseConnection = null;
            if (isPending) startPolling();
          },
        );
        sseActive = true;
      } catch {
        startPolling();
      }
    } else {
      startPolling();
    }

    return () => {
      closeSSE();
      stopPolling();
    };
  });
</script>

<div class="container">
  {#if loading}
    <div class="loading">Loading...</div>
  {:else if error}
    <div class="error">{error}</div>
  {:else if req && policy}
    <div class="header">
      <StatusBadge status={req.status} />
      <span class="type">{req.type}</span>
    </div>
    <StageBar stages={policy.stages} currentStage={req.current_stage} {approvals} status={req.status} />
  {:else}
    <div class="error">Request or policy not found</div>
  {/if}
</div>

<style>
  .container {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    padding: 12px;
  }
  .header {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 12px;
  }
  .type {
    font-size: 13px;
    color: #6b7280;
    font-family: monospace;
  }
  .loading, .error {
    font-size: 13px;
    padding: 8px 0;
  }
  .loading { color: #6b7280; }
  .error { color: #ef4444; }
</style>
