<script lang="ts">
  import { requests as requestsApi, deliveries as deliveriesApi } from '../lib/api';
  import { addToast } from '../lib/stores';
  import { formatDate, formatDetails } from '../lib/utils';
  import StatusBadge from '../components/StatusBadge.svelte';
  import LoadingSpinner from '../components/LoadingSpinner.svelte';
  import type { Request, AuditLog, OutboxEntry } from '../lib/types';
  import { getDisplay } from '../lib/types';

  let { id }: { id: string } = $props();

  let req: Request | null = $state(null);
  let auditLogs: AuditLog[] = $state([]);
  let outboxEntries: OutboxEntry[] = $state([]);
  let isLoading = $state(true);
  let activeTab: 'details' | 'payload' | 'audit' | 'deliveries' = $state('details');
  let showRawPayload = $state(false);
  let retryingId: string | null = $state(null);
  let retryingAll = $state(false);
  let deliveryPage = $state(1);
  let deliveryTotal = $state(0);
  const deliveryPerPage = 20;
  let loadingMore = $state(false);

  $effect(() => {
    loadRequest(id);
  });

  async function loadRequest(requestId: string) {
    isLoading = true;
    deliveryPage = 1;
    try {
      const [requestData, auditData, deliveryData] = await Promise.all([
        requestsApi.get(requestId),
        requestsApi.audit(requestId),
        deliveriesApi.list({ request_id: requestId, per_page: deliveryPerPage, page: 1 }).catch(() => ({ data: [], total: 0 })),
      ]);
      req = requestData;
      auditLogs = auditData.data || [];
      outboxEntries = deliveryData.data || [];
      deliveryTotal = deliveryData.total || 0;
    } catch {
      addToast('Failed to load request', 'error');
      window.location.hash = '#/requests';
    } finally {
      isLoading = false;
    }
  }

  async function loadMoreDeliveries() {
    loadingMore = true;
    try {
      const nextPage = deliveryPage + 1;
      const deliveryData = await deliveriesApi.list({ request_id: id, per_page: deliveryPerPage, page: nextPage });
      outboxEntries = [...outboxEntries, ...(deliveryData.data || [])];
      deliveryPage = nextPage;
    } catch {
      addToast('Failed to load more deliveries', 'error');
    } finally {
      loadingMore = false;
    }
  }

  function formatJson(obj: unknown): string {
    return JSON.stringify(obj, null, 2);
  }

  async function retryEntry(entryId: string) {
    retryingId = entryId;
    try {
      await deliveriesApi.retry(entryId);
      addToast('Delivery queued for retry', 'success');
      await loadRequest(id);
    } catch {
      addToast('Failed to retry delivery', 'error');
    } finally {
      retryingId = null;
    }
  }

  async function retryAllFailed() {
    retryingAll = true;
    try {
      const res = await deliveriesApi.retryAllForRequest(id);
      addToast(`${res.reset} deliveries queued for retry`, 'success');
      await loadRequest(id);
    } catch {
      addToast('Failed to retry deliveries', 'error');
    } finally {
      retryingAll = false;
    }
  }

  function deliveryStatusClass(status: string): string {
    switch (status) {
      case 'delivered': return 'bg-status-approved-bg text-status-approved-text';
      case 'failed': return 'bg-status-rejected-bg text-status-rejected-text';
      case 'processing': return 'bg-surface-container text-on-surface-variant';
      case 'pending': return 'bg-status-pending-bg text-status-pending-text';
      default: return 'bg-surface-container text-on-surface-variant';
    }
  }

  function actionColor(action: string): string {
    switch (action) {
      case 'created':
      case 'webhook_sent':
      case 'webhook_failed': return 'bg-surface-container text-on-surface-variant';
      case 'approved':
      case 'stage_advanced': return 'bg-status-approved-bg text-status-approved-text';
      case 'rejected': return 'bg-status-rejected-bg text-status-rejected-text';
      case 'cancelled':
      case 'expired': return 'bg-surface-container text-on-surface-variant';
      default: return 'bg-surface-container text-on-surface-variant';
    }
  }

  let failedCount = $derived(outboxEntries.filter(e => e.status === 'failed').length);
</script>

<div>
  <div class="mb-6">
    <a href="#/requests" class="text-sm text-on-surface-variant hover:text-on-surface">&larr; Back to requests</a>
    <h1 class="text-2xl font-bold text-on-surface mt-2">Request Detail</h1>
  </div>

  {#if isLoading}
    <LoadingSpinner />
  {:else if req}
    <!-- Header card -->
    <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl p-6 mb-6">
      <div class="flex items-start justify-between">
        <div>
          <div class="flex items-center gap-3 mb-2">
            <StatusBadge status={req.status} />
            <span class="text-sm text-on-surface-variant">Stage {req.current_stage}</span>
          </div>
          <p class="font-mono text-sm text-on-surface-variant mb-1">{req.id}</p>
          <p class="text-sm text-on-surface-variant">Type: <span class="font-medium text-on-surface">{req.type}</span></p>
        </div>
        <div class="text-right text-sm text-on-surface-variant">
          <p>Created: {formatDate(req.created_at)}</p>
          <p>Updated: {formatDate(req.updated_at)}</p>
          {#if req.expires_at}
            <p>Expires: {formatDate(req.expires_at)}</p>
          {/if}
        </div>
      </div>

      <div class="mt-4 pt-4 border-t border-outline-variant/15 grid grid-cols-3 gap-4 text-sm">
        <div>
          <span class="text-on-surface-variant">Maker</span>
          <p class="font-medium text-on-surface">{req.maker_id}</p>
        </div>
        {#if req.idempotency_key}
          <div>
            <span class="text-on-surface-variant">Idempotency Key</span>
            <p class="font-mono text-on-surface text-xs">{req.idempotency_key}</p>
          </div>
        {/if}
        {#if req.fingerprint}
          <div>
            <span class="text-on-surface-variant">Fingerprint</span>
            <p class="font-mono text-on-surface text-xs">{req.fingerprint}</p>
          </div>
        {/if}
      </div>

      {#if req.eligible_reviewers && req.eligible_reviewers.length > 0}
        <div class="mt-4 pt-4 border-t border-outline-variant/15">
          <span class="text-sm text-on-surface-variant">Eligible Reviewers</span>
          <div class="flex flex-wrap gap-1 mt-1">
            {#each req.eligible_reviewers as reviewer}
              <span class="inline-flex px-2 py-0.5 bg-indigo-50 text-indigo-700 rounded text-xs">{reviewer}</span>
            {/each}
          </div>
        </div>
      {/if}
    </div>

    <!-- Tabs -->
    <div class="border-b border-outline-variant/15 mb-4">
      <nav class="flex gap-6">
        <button
          onclick={() => activeTab = 'details'}
          class="pb-2 text-sm font-medium border-b-2 transition-colors {activeTab === 'details' ? 'border-primary-container text-primary-container' : 'border-transparent text-on-surface-variant hover:text-on-surface'}"
        >
          Details
        </button>
        <button
          onclick={() => activeTab = 'payload'}
          class="pb-2 text-sm font-medium border-b-2 transition-colors {activeTab === 'payload' ? 'border-primary-container text-primary-container' : 'border-transparent text-on-surface-variant hover:text-on-surface'}"
        >
          Payload
        </button>
        <button
          onclick={() => activeTab = 'audit'}
          class="pb-2 text-sm font-medium border-b-2 transition-colors {activeTab === 'audit' ? 'border-primary-container text-primary-container' : 'border-transparent text-on-surface-variant hover:text-on-surface'}"
        >
          Audit Trail ({auditLogs.length})
        </button>
        <button
          onclick={() => activeTab = 'deliveries'}
          class="pb-2 text-sm font-medium border-b-2 transition-colors {activeTab === 'deliveries' ? 'border-primary-container text-primary-container' : 'border-transparent text-on-surface-variant hover:text-on-surface'}"
        >
          Deliveries ({outboxEntries.length})
        </button>
      </nav>
    </div>

    <!-- Tab content -->
    {#if activeTab === 'details'}
      <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl p-6">
        {#if req.metadata && Object.keys(req.metadata).length > 0}
          <h3 class="text-sm font-medium text-on-surface mb-2">Metadata</h3>
          <pre class="bg-surface-container-low rounded-md p-4 text-sm font-mono text-gray-800 overflow-x-auto">{formatJson(req.metadata)}</pre>
        {:else}
          <p class="text-sm text-on-surface-variant">No additional metadata.</p>
        {/if}
      </div>
    {:else if activeTab === 'payload'}
      {@const displayData = getDisplay(req.metadata)}
      <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl p-6">
        {#if displayData && !showRawPayload}
          {#if displayData.title}
            <h3 class="text-base font-semibold text-on-surface mb-4">{displayData.title}</h3>
          {/if}
          <div class="space-y-2">
            {#each displayData.fields as field}
              <div class="flex items-baseline gap-3">
                <span class="text-sm text-on-surface-variant min-w-[140px] flex-shrink-0">{field.label}</span>
                <span class="text-sm text-on-surface">{field.value}</span>
              </div>
            {/each}
          </div>
          {#if displayData.items && displayData.items.length > 0}
            <div class="mt-4 space-y-2">
              {#each displayData.items as item}
                <div class="bg-surface-container-low rounded-md p-3">
                  <p class="text-sm font-semibold text-on-surface mb-1">{item.title}</p>
                  {#each item.fields as field}
                    <div class="flex items-baseline gap-3 pl-2">
                      <span class="text-xs text-on-surface-variant min-w-[120px] flex-shrink-0">{field.label}</span>
                      <span class="text-sm text-on-surface">{field.value}</span>
                    </div>
                  {/each}
                </div>
              {/each}
            </div>
          {/if}
          <button
            onclick={() => showRawPayload = true}
            class="mt-4 text-xs text-on-surface-variant hover:text-on-surface border border-outline-variant/40 rounded px-2 py-1"
          >Show raw payload</button>
        {:else}
          <pre class="bg-surface-container-low rounded-md p-4 text-sm font-mono text-gray-800 overflow-x-auto">{formatJson(req.payload)}</pre>
          {#if displayData}
            <button
              onclick={() => showRawPayload = false}
              class="mt-4 text-xs text-on-surface-variant hover:text-on-surface border border-outline-variant/40 rounded px-2 py-1"
            >Show formatted view</button>
          {/if}
        {/if}
      </div>
    {:else if activeTab === 'audit'}
      {#if auditLogs.length === 0}
        <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl p-6">
          <p class="text-sm text-on-surface-variant">No audit entries yet.</p>
        </div>
      {:else}
        <div class="space-y-3">
          {#each auditLogs as log}
            <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl p-4">
              <div class="flex items-start justify-between">
                <div class="flex items-center gap-2">
                  <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium {actionColor(log.action)}">
                    {log.action}
                  </span>
                  <span class="text-sm text-on-surface">by <span class="font-medium">{log.actor_id}</span></span>
                </div>
                <span class="text-xs text-on-surface-variant">{formatDate(log.created_at)}</span>
              </div>
              {#if log.details && Object.keys(log.details).length > 0}
                <span class="mt-2 block text-xs text-on-surface-variant whitespace-pre-line">{formatDetails(log.details, log.action)}</span>
              {/if}
            </div>
          {/each}
        </div>
      {/if}
    {:else if activeTab === 'deliveries'}
      {#if outboxEntries.length === 0}
        <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl p-6">
          <p class="text-sm text-on-surface-variant">No delivery entries for this request.</p>
        </div>
      {:else}
        {#if failedCount > 0}
          <div class="mb-3 flex justify-end">
            <button
              onclick={retryAllFailed}
              disabled={retryingAll}
              class="text-sm bg-status-rejected-bg text-status-rejected-text hover:bg-red-100 border border-status-rejected-text/20 rounded-md px-3 py-1.5 disabled:opacity-50"
            >
              {retryingAll ? 'Retrying…' : `Retry All Failed (${failedCount})`}
            </button>
          </div>
        {/if}
        <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl overflow-hidden">
          <table class="min-w-full divide-y divide-outline-variant/15">
            <thead class="bg-surface-container-low">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Webhook URL</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Status</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Attempts</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Last Error</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Created</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Actions</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-outline-variant/15">
              {#each outboxEntries as entry}
                <tr class="hover:bg-surface-container-low">
                  <td class="px-4 py-3 text-xs font-mono text-on-surface max-w-[250px] truncate" title={entry.webhook_url}>{entry.webhook_url}</td>
                  <td class="px-4 py-3">
                    <span class="inline-flex px-2 py-0.5 rounded-full text-xs font-medium {deliveryStatusClass(entry.status)}">{entry.status}</span>
                  </td>
                  <td class="px-4 py-3 text-sm text-on-surface">{entry.attempts} / {entry.max_retries}</td>
                  <td class="px-4 py-3 text-xs text-status-rejected-text max-w-[200px] truncate" title={entry.last_error || ''}>{entry.last_error || '—'}</td>
                  <td class="px-4 py-3 text-xs text-on-surface-variant">{formatDate(entry.created_at)}</td>
                  <td class="px-4 py-3">
                    {#if entry.status === 'failed'}
                      <button
                        onclick={() => retryEntry(entry.id)}
                        disabled={retryingId === entry.id}
                        class="text-xs text-primary-container hover:text-primary disabled:opacity-50 disabled:cursor-not-allowed"
                      >
                        {retryingId === entry.id ? 'Retrying…' : 'Retry'}
                      </button>
                    {:else}
                      <span class="text-xs text-on-surface-variant/60">—</span>
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
        {#if outboxEntries.length < deliveryTotal}
          <div class="mt-3 flex justify-center">
            <button
              onclick={loadMoreDeliveries}
              disabled={loadingMore}
              class="text-sm text-primary-container hover:text-primary border border-indigo-200 rounded-md px-4 py-2 disabled:opacity-50"
            >
              {loadingMore ? 'Loading…' : `Load more (${outboxEntries.length} of ${deliveryTotal})`}
            </button>
          </div>
        {/if}
      {/if}
    {/if}
  {/if}
</div>
