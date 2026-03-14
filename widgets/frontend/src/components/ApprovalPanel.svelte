<svelte:options customElement={{ tag: "quorum-approval-panel", shadow: "open" }} />

<script lang="ts">
  import { createClient, ApiError } from '../lib/api';
  import type { Request, Policy, Approval, AuditLog } from '../lib/types';
  import { formatDate, timeAgo } from '../lib/utils';
  import StatusBadge from './internal/StatusBadge.svelte';
  import StageBar from './internal/StageBar.svelte';
  import AuditTimeline from './internal/AuditTimeline.svelte';

  let {
    'request-id': requestId = '',
    'api-url': apiUrl = '',
    token = '',
    'auth-headers': authHeadersStr = '',
    'poll-interval': pollIntervalStr = '30000',
  }: {
    'request-id'?: string;
    'api-url'?: string;
    token?: string;
    'auth-headers'?: string;
    'poll-interval'?: string;
  } = $props();

  let req: Request | null = $state(null);
  let policy: Policy | null = $state(null);
  let approvals: Approval[] = $state([]);
  let auditLogs: AuditLog[] = $state([]);
  let error: string | null = $state(null);
  let loading = $state(true);
  let actionLoading = $state(false);
  let comment = $state('');
  let activeTab: 'details' | 'payload' | 'audit' = $state('details');
  let pollTimer: ReturnType<typeof setInterval> | undefined;

  function getClient() {
    let authHeaders: Record<string, string> | undefined;
    if (authHeadersStr) {
      try { authHeaders = JSON.parse(authHeadersStr); } catch {}
    }
    return createClient({ apiUrl, token: token || undefined, authHeaders });
  }

  function dispatch(name: string, detail: unknown) {
    const el = document.querySelector(`quorum-approval-panel[request-id="${requestId}"]`);
    el?.dispatchEvent(new CustomEvent(name, { detail, bubbles: true, composed: true }));
  }

  async function load() {
    if (!requestId || !apiUrl) return;
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
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Failed to load';
      dispatch('quorum:error', { message: error, status: e instanceof ApiError ? e.status : 0 });
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
      dispatch('quorum:error', { message: msg, status: e instanceof ApiError ? e.status : 0 });
    } finally {
      actionLoading = false;
    }
  }

  $effect(() => {
    if (requestId && apiUrl) load();
  });

  $effect(() => {
    if (pollTimer) clearInterval(pollTimer);
    const interval = parseInt(pollIntervalStr) || 30000;
    if (req?.status === 'pending' && interval > 0) {
      pollTimer = setInterval(() => load(), interval);
    }
    return () => { if (pollTimer) clearInterval(pollTimer); };
  });
</script>

<div class="panel">
  {#if loading && !req}
    <div class="loading">Loading request...</div>
  {:else if error && !req}
    <div class="error-box">{error}</div>
  {:else if req}
    <!-- Header -->
    <div class="header">
      <div class="header-left">
        <StatusBadge status={req.status} />
        <span class="type">{req.type}</span>
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
        <pre class="payload">{JSON.stringify(req.payload, null, 2)}</pre>
      {:else if activeTab === 'audit'}
        <AuditTimeline logs={auditLogs} />
      {/if}
    </div>

    <!-- Actions -->
    {#if req.status === 'pending'}
      <div class="actions">
        {#if error}
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
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    border: 1px solid #e5e7eb;
    border-radius: 8px;
    padding: 16px;
    background: #fff;
    color: #111827;
    max-width: 600px;
  }
  .loading { font-size: 13px; color: #6b7280; padding: 16px 0; text-align: center; }
  .error-box { font-size: 13px; color: #ef4444; padding: 8px 12px; background: #fef2f2; border-radius: 6px; }
  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }
  .header-left { display: flex; align-items: center; gap: 8px; }
  .type { font-size: 13px; color: #6b7280; font-family: monospace; }
  .id { font-size: 11px; color: #9ca3af; font-family: monospace; }
  .meta {
    display: flex;
    gap: 16px;
    font-size: 12px;
    color: #6b7280;
    margin-bottom: 16px;
    flex-wrap: wrap;
  }
  .meta strong { color: #374151; }
  .section { margin-bottom: 16px; }
  .tabs {
    display: flex;
    border-bottom: 1px solid #e5e7eb;
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
    color: #6b7280;
    cursor: pointer;
    transition: all 0.15s;
  }
  .tab:hover { color: #374151; }
  .tab.active { color: #4f46e5; border-bottom-color: #4f46e5; }
  .tab-content { min-height: 80px; }
  .detail-grid { display: flex; flex-direction: column; gap: 8px; }
  .detail-row { display: flex; align-items: center; gap: 8px; }
  .detail-label { font-size: 12px; color: #9ca3af; min-width: 100px; }
  .detail-value { font-size: 13px; color: #374151; }
  .mono { font-family: monospace; font-size: 12px; }
  .payload {
    font-size: 12px;
    font-family: monospace;
    background: #f9fafb;
    border: 1px solid #e5e7eb;
    border-radius: 6px;
    padding: 12px;
    overflow-x: auto;
    white-space: pre-wrap;
    word-break: break-word;
    margin: 0;
    color: #374151;
  }
  .actions {
    margin-top: 16px;
    padding-top: 16px;
    border-top: 1px solid #e5e7eb;
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
    border: 1px solid #d1d5db;
    border-radius: 6px;
    resize: vertical;
    font-family: inherit;
    box-sizing: border-box;
    margin-bottom: 8px;
  }
  .comment-input:focus { outline: none; border-color: #6366f1; box-shadow: 0 0 0 2px rgba(99,102,241,0.15); }
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
</style>
