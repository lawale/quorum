<svelte:options customElement={{ tag: "quorum-approval-panel", shadow: "open" }} />

<script lang="ts">
  import { createClient, ApiError } from '../lib/api';
  import { getGlobalConfig, hasGlobalApiUrl } from '../lib/config';
  import type { SSEConnection } from '../lib/api';
  import type { Request, Policy, Approval, AuditLog } from '../lib/types';
  import { formatDate, timeAgo, getDisplay } from '../lib/utils';
  import type { ResolvedDisplay } from '../lib/types';
  import StatusBadge from './internal/StatusBadge.svelte';
  import StageBar from './internal/StageBar.svelte';
  import AuditTimeline from './internal/AuditTimeline.svelte';

  let {
    'request-id': requestId = '',
    'api-url': apiUrl = '',
    token = '',
    'tenant-id': tenantId = '',
    'auth-headers': authHeadersStr = '',
    'poll-interval': pollIntervalStr = '30000',
    'suppress-errors': suppressErrorsStr,
    'sse': sseStr = 'true',
  }: {
    'request-id'?: string;
    'api-url'?: string;
    token?: string;
    'tenant-id'?: string;
    'auth-headers'?: string;
    'poll-interval'?: string;
    'suppress-errors'?: string;
    'sse'?: string;
  } = $props();

  // Any presence of the attribute suppresses inline errors, unless explicitly "false".
  // Bare attribute → "", explicit value → "true"/"false"/etc, absent → undefined.
  let suppressErrors = $derived(suppressErrorsStr !== undefined && suppressErrorsStr !== 'false');

  let req: Request | null = $state(null);
  let policy: Policy | null = $state(null);
  let approvals: Approval[] = $derived(req?.approvals ?? []);
  let auditLogs: AuditLog[] = $state([]);
  let error: string | null = $state(null);
  let loading = $state(true);
  let actionLoading = $state(false);
  let comment = $state('');
  let activeTab: 'details' | 'payload' | 'audit' = $state('details');
  let showRawPayload = $state(false);
  let pollTimer: ReturnType<typeof setInterval> | undefined;
  let sseConnection: SSEConnection | null = null;
  let sseActive = false;
  let sseEnabled = $derived(sseStr !== 'false');
  let isPending = $state(false);

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

  function dispatch(name: string, detail: unknown) {
    const el = document.querySelector(`quorum-approval-panel[request-id="${requestId}"]`);
    el?.dispatchEvent(new CustomEvent(name, { detail, bubbles: true, composed: true }));
  }

  async function load() {
    if (!requestId || (!apiUrl && !hasGlobalApiUrl())) return;
    loading = true;
    error = null;
    try {
      const client = getClient();
      const [r, logs] = await Promise.all([
        client.getRequest(requestId),
        client.getAudit(requestId),
      ]);
      req = r;
      auditLogs = logs;
      policy = await client.getPolicyByType(req.type);
      isPending = req.status === 'pending';
      if (!isPending) {
        closeSSE();
        stopPolling();
      }
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Failed to load';
      dispatch('quorum:error', { action: 'load', message: error, status: e instanceof ApiError ? e.status : 0 });
    } finally {
      loading = false;
    }
  }

  async function handleAction(action: 'approve' | 'reject') {
    if (!req) return;
    actionLoading = true;
    try {
      const client = getClient();
      const fn = action === 'approve' ? client.approve : client.reject;
      const updated = await fn(req.id, comment || undefined);
      req = updated;
      comment = '';
      dispatch(action === 'approve' ? 'quorum:approved' : 'quorum:rejected', {
        requestId: req.id,
        request: updated,
      });
      await load();
    } catch (e) {
      const msg = e instanceof ApiError ? e.message : 'Action failed';
      error = msg;
      dispatch('quorum:error', { action, message: msg, status: e instanceof ApiError ? e.status : 0 });
    } finally {
      actionLoading = false;
    }
  }

  $effect(() => {
    if (requestId && apiUrl) load();
  });

  // Re-evaluate after QuorumEmbed.configure() is called
  $effect(() => {
    function onConfigured() {
      if (requestId && !apiUrl && hasGlobalApiUrl()) load();
    }
    document.addEventListener('quorum:configured', onConfigured);
    return () => document.removeEventListener('quorum:configured', onConfigured);
  });

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

  // SSE-first with polling fallback. They are mutually exclusive:
  // - SSE active → no polling
  // - SSE fails/disconnects → start polling as fallback
  // Uses isPending (a primitive boolean) to avoid re-running when load()
  // creates a new req object with the same status.
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

<div class="panel">
  {#if loading && !req}
    <div class="loading">Loading request...</div>
  {:else if error && !req && !suppressErrors}
    <div class="error-box">{error}</div>
  {:else if req}
    <!-- Header -->
    <div class="header">
      <div class="header-left">
        <StatusBadge status={req.status} />
        <span class="type">{policy?.name ?? req.type}</span>
      </div>
      <span class="id" title={req.id}>{req.id.slice(0, 8)}...</span>
    </div>

    <div class="meta">
      <span>Maker: <strong>{req.maker_id}</strong></span>
      <span>Created: {timeAgo(req.created_at)}</span>
      {#if req.expires_at}
        <span>Expires: {formatDate(req.expires_at)}</span>
      {/if}
    </div>

    <!-- Stage Progress -->
    {#if policy}
      <div class="section">
        <StageBar stages={policy.stages} currentStage={req.current_stage} {approvals} status={req.status} />
      </div>
    {/if}

    <!-- Tabs -->
    <div class="tabs">
      <button class="tab" class:active={activeTab === 'details'} onclick={() => activeTab = 'details'}>Details</button>
      <button class="tab" class:active={activeTab === 'payload'} onclick={() => activeTab = 'payload'}>Payload</button>
      <button class="tab" class:active={activeTab === 'audit'} onclick={() => activeTab = 'audit'}>Audit Trail</button>
    </div>

    <div class="tab-content">
      {#if activeTab === 'details'}
        <div class="detail-grid">
          <div class="detail-row">
            <span class="detail-label">Status</span>
            <StatusBadge status={req.status} />
          </div>
          <div class="detail-row">
            <span class="detail-label">Type</span>
            <span class="detail-value">{req.type}</span>
          </div>
          <div class="detail-row">
            <span class="detail-label">Stage</span>
            <span class="detail-value">{req.current_stage + 1} of {policy?.stages.length ?? '?'}</span>
          </div>
          {#if req.idempotency_key}
            <div class="detail-row">
              <span class="detail-label">Idempotency Key</span>
              <span class="detail-value mono">{req.idempotency_key}</span>
            </div>
          {/if}
        </div>
      {:else if activeTab === 'payload'}
        {@const displayData = getDisplay(req.metadata)}
        {#if displayData && !showRawPayload}
          <div class="display-view">
            {#if displayData.title}
              <h3 class="display-title">{displayData.title}</h3>
            {/if}
            <div class="display-fields">
              {#each displayData.fields as field}
                <div class="display-row">
                  <span class="display-label">{field.label}</span>
                  <span class="display-value">{field.value}</span>
                </div>
              {/each}
            </div>
            {#if displayData.items && displayData.items.length > 0}
              <div class="display-items">
                {#each displayData.items as item}
                  <div class="display-item">
                    <div class="display-item-title">{item.title}</div>
                    {#each item.fields as field}
                      <div class="display-row display-row-sm">
                        <span class="display-label">{field.label}</span>
                        <span class="display-value">{field.value}</span>
                      </div>
                    {/each}
                  </div>
                {/each}
              </div>
            {/if}
            <button class="raw-toggle" onclick={() => showRawPayload = true}>Show raw payload</button>
          </div>
        {:else}
          <pre class="payload">{JSON.stringify(req.payload, null, 2)}</pre>
          {#if displayData}
            <button class="raw-toggle" onclick={() => showRawPayload = false}>Show formatted view</button>
          {/if}
        {/if}
      {:else if activeTab === 'audit'}
        <AuditTimeline logs={auditLogs} />
      {/if}
    </div>

    <!-- Actions -->
    {#if req.viewer_can_act}
      <div class="actions">
        {#if error && !suppressErrors}
          <div class="action-error">{error}</div>
        {/if}
        <textarea
          class="comment-input"
          placeholder="Add a comment (optional)"
          bind:value={comment}
          rows="2"
        ></textarea>
        <div class="action-buttons">
          <button class="btn btn-approve" disabled={actionLoading} onclick={() => handleAction('approve')}>
            {actionLoading ? 'Processing...' : 'Approve'}
          </button>
          <button class="btn btn-reject" disabled={actionLoading} onclick={() => handleAction('reject')}>
            {actionLoading ? 'Processing...' : 'Reject'}
          </button>
        </div>
      </div>
    {/if}
  {:else}
    <div class="error-box">Request not found</div>
  {/if}
</div>

<style>
  .panel {
    font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    border: none;
    box-shadow: 0 4px 24px -4px rgba(25, 28, 29, 0.06);
    border-radius: 8px;
    padding: 16px;
    background: #fff;
    color: #191c1d;
    max-width: 600px;
  }
  .loading { font-size: 13px; color: #464555; padding: 16px 0; text-align: center; }
  .error-box { font-size: 13px; color: #991b1b; padding: 8px 12px; background: #fee2e2; border-radius: 6px; }
  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }
  .header-left { display: flex; align-items: center; gap: 8px; }
  .type { font-size: 13px; color: #464555; }
  .id { font-size: 11px; color: #464555; font-family: monospace; }
  .meta {
    display: flex;
    gap: 16px;
    font-size: 12px;
    color: #464555;
    margin-bottom: 16px;
    flex-wrap: wrap;
  }
  .meta strong { color: #191c1d; }
  .section { margin-bottom: 16px; }
  .tabs {
    display: flex;
    border-bottom: 1px solid rgba(199, 196, 216, 0.15);
    gap: 0;
    margin-bottom: 12px;
  }
  .tab {
    padding: 8px 16px;
    font-size: 13px;
    font-weight: 500;
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    color: #464555;
    cursor: pointer;
    transition: all 0.15s;
  }
  .tab:hover { color: #191c1d; }
  .tab.active { color: #4f46e5; border-bottom-color: #4f46e5; }
  .tab-content { min-height: 80px; }
  .detail-grid { display: flex; flex-direction: column; gap: 8px; }
  .detail-row { display: flex; align-items: center; gap: 8px; }
  .detail-label { font-size: 12px; color: #464555; min-width: 100px; }
  .detail-value { font-size: 13px; color: #191c1d; }
  .mono { font-family: monospace; font-size: 12px; }
  .payload {
    font-size: 12px;
    font-family: monospace;
    background: #f3f4f5;
    border: 1px solid rgba(199, 196, 216, 0.15);
    border-radius: 6px;
    padding: 12px;
    overflow-x: auto;
    white-space: pre-wrap;
    word-break: break-word;
    margin: 0;
    color: #191c1d;
  }
  .actions {
    margin-top: 16px;
    padding-top: 16px;
    border-top: 1px solid rgba(199, 196, 216, 0.15);
  }
  .action-error {
    font-size: 12px;
    color: #ef4444;
    margin-bottom: 8px;
  }
  .comment-input {
    width: 100%;
    padding: 8px 10px;
    font-size: 13px;
    border: 1px solid rgba(199, 196, 216, 0.40);
    border-radius: 6px;
    resize: vertical;
    font-family: inherit;
    box-sizing: border-box;
    margin-bottom: 8px;
  }
  .comment-input:focus { outline: none; border-color: #4f46e5; box-shadow: 0 0 0 2px rgba(79,70,229,0.15); }
  .action-buttons { display: flex; gap: 8px; }
  .btn {
    flex: 1;
    padding: 8px 16px;
    font-size: 13px;
    font-weight: 500;
    border: none;
    border-radius: 6px;
    cursor: pointer;
    transition: background 0.15s;
  }
  .btn:disabled { opacity: 0.6; cursor: not-allowed; }
  .btn-approve { background: #10b981; color: #fff; }
  .btn-approve:hover:not(:disabled) { background: #059669; }
  .btn-reject { background: #ef4444; color: #fff; }
  .btn-reject:hover:not(:disabled) { background: #dc2626; }

  .display-view { }
  .display-title {
    font-size: 15px;
    font-weight: 600;
    color: #191c1d;
    margin: 0 0 12px 0;
  }
  .display-fields { display: flex; flex-direction: column; gap: 6px; }
  .display-row { display: flex; align-items: baseline; gap: 8px; }
  .display-row-sm { padding-left: 8px; }
  .display-label { font-size: 12px; color: #464555; min-width: 120px; flex-shrink: 0; }
  .display-value { font-size: 13px; color: #191c1d; }
  .display-items { margin-top: 12px; display: flex; flex-direction: column; gap: 8px; }
  .display-item {
    padding: 8px 12px;
    background: #f3f4f5;
    border: 1px solid rgba(199, 196, 216, 0.15);
    border-radius: 6px;
  }
  .display-item-title { font-size: 13px; font-weight: 600; color: #191c1d; margin-bottom: 4px; }
  .raw-toggle {
    margin-top: 10px;
    padding: 4px 10px;
    font-size: 11px;
    background: none;
    border: 1px solid rgba(199, 196, 216, 0.40);
    border-radius: 4px;
    color: #464555;
    cursor: pointer;
  }
  .raw-toggle:hover { background: #f3f4f5; color: #191c1d; }
</style>
