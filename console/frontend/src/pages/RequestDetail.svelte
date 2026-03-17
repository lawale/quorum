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

  $effect(() => {
    loadRequest(id);
  });

  async function loadRequest(requestId: string) {
    isLoading = true;
    try {
      const [requestData, auditData, deliveryData] = await Promise.all([
        requestsApi.get(requestId),
        requestsApi.audit(requestId),
        deliveriesApi.list({ request_id: requestId, per_page: 50 }).catch(() => ({ data: [], total: 0 })),
      ]);
      req = requestData;
      auditLogs = auditData.data || [];
      outboxEntries = deliveryData.data || [];
    } catch {
      addToast('Failed to load request', 'error');
      window.location.hash = '#/requests';
    } finally {
      isLoading = false;
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
      case 'delivered': return 'bg-green-100 text-green-800';
      case 'failed': return 'bg-red-100 text-red-800';
      case 'processing': return 'bg-blue-100 text-blue-800';
      case 'pending': return 'bg-yellow-100 text-yellow-800';
      default: return 'bg-gray-100 text-gray-600';
    }
  }

  function actionColor(action: string): string {
    switch (action) {
      case 'created': return 'bg-blue-100 text-blue-800';
      case 'approved': return 'bg-green-100 text-green-800';
      case 'rejected': return 'bg-red-100 text-red-800';
      case 'cancelled': return 'bg-gray-100 text-gray-800';
      case 'expired': return 'bg-orange-100 text-orange-800';
      default: return 'bg-gray-100 text-gray-600';
    }
  }

  let failedCount = $derived(outboxEntries.filter(e => e.status === 'failed').length);
</script>

<div>
  <div class="mb-6">
    <a href="#/requests" class="text-sm text-gray-500 hover:text-gray-700">&larr; Back to requests</a>
    <h1 class="text-2xl font-bold text-gray-900 mt-2">Request Detail</h1>
  </div>

  {#if isLoading}
    <LoadingSpinner />
  {:else if req}
    <!-- Header card -->
    <div class="bg-white shadow-sm rounded-lg border border-gray-200 p-6 mb-6">
      <div class="flex items-start justify-between">
        <div>
          <div class="flex items-center gap-3 mb-2">
            <StatusBadge status={req.status} />
            <span class="text-sm text-gray-500">Stage {req.current_stage}</span>
          </div>
          <p class="font-mono text-sm text-gray-600 mb-1">{req.id}</p>
          <p class="text-sm text-gray-500">Type: <span class="font-medium text-gray-900">{req.type}</span></p>
        </div>
        <div class="text-right text-sm text-gray-500">
          <p>Created: {formatDate(req.created_at)}</p>
          <p>Updated: {formatDate(req.updated_at)}</p>
          {#if req.expires_at}
            <p>Expires: {formatDate(req.expires_at)}</p>
          {/if}
        </div>
      </div>

      <div class="mt-4 pt-4 border-t border-gray-200 grid grid-cols-3 gap-4 text-sm">
        <div>
          <span class="text-gray-500">Maker</span>
          <p class="font-medium text-gray-900">{req.maker_id}</p>
        </div>
        {#if req.idempotency_key}
          <div>
            <span class="text-gray-500">Idempotency Key</span>
            <p class="font-mono text-gray-900 text-xs">{req.idempotency_key}</p>
          </div>
        {/if}
        {#if req.fingerprint}
          <div>
            <span class="text-gray-500">Fingerprint</span>
            <p class="font-mono text-gray-900 text-xs">{req.fingerprint}</p>
          </div>
        {/if}
      </div>

      {#if req.eligible_reviewers && req.eligible_reviewers.length > 0}
        <div class="mt-4 pt-4 border-t border-gray-200">
          <span class="text-sm text-gray-500">Eligible Reviewers</span>
          <div class="flex flex-wrap gap-1 mt-1">
            {#each req.eligible_reviewers as reviewer}
              <span class="inline-flex px-2 py-0.5 bg-indigo-50 text-indigo-700 rounded text-xs">{reviewer}</span>
            {/each}
          </div>
        </div>
      {/if}
    </div>

    <!-- Tabs -->
    <div class="border-b border-gray-200 mb-4">
      <nav class="flex gap-6">
        <button
          onclick={() => activeTab = 'details'}
          class="pb-2 text-sm font-medium border-b-2 transition-colors {activeTab === 'details' ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-gray-500 hover:text-gray-700'}"
        >
          Details
        </button>
        <button
          onclick={() => activeTab = 'payload'}
          class="pb-2 text-sm font-medium border-b-2 transition-colors {activeTab === 'payload' ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-gray-500 hover:text-gray-700'}"
        >
          Payload
        </button>
        <button
          onclick={() => activeTab = 'audit'}
          class="pb-2 text-sm font-medium border-b-2 transition-colors {activeTab === 'audit' ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-gray-500 hover:text-gray-700'}"
        >
          Audit Trail ({auditLogs.length})
        </button>
        <button
          onclick={() => activeTab = 'deliveries'}
          class="pb-2 text-sm font-medium border-b-2 transition-colors {activeTab === 'deliveries' ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-gray-500 hover:text-gray-700'}"
        >
          Deliveries ({outboxEntries.length})
        </button>
      </nav>
    </div>

    <!-- Tab content -->
    {#if activeTab === 'details'}
      <div class="bg-white shadow-sm rounded-lg border border-gray-200 p-6">
        {#if req.metadata && Object.keys(req.metadata).length > 0}
          <h3 class="text-sm font-medium text-gray-700 mb-2">Metadata</h3>
          <pre class="bg-gray-50 rounded-md p-4 text-sm font-mono text-gray-800 overflow-x-auto">{formatJson(req.metadata)}</pre>
        {:else}
          <p class="text-sm text-gray-500">No additional metadata.</p>
        {/if}
      </div>
    {:else if activeTab === 'payload'}
      {@const displayData = getDisplay(req.metadata)}
      <div class="bg-white shadow-sm rounded-lg border border-gray-200 p-6">
        {#if displayData && !showRawPayload}
          {#if displayData.title}
            <h3 class="text-base font-semibold text-gray-900 mb-4">{displayData.title}</h3>
          {/if}
          <div class="space-y-2">
            {#each displayData.fields as field}
              <div class="flex items-baseline gap-3">
                <span class="text-sm text-gray-500 min-w-[140px] flex-shrink-0">{field.label}</span>
                <span class="text-sm text-gray-900">{field.value}</span>
              </div>
            {/each}
          </div>
          {#if displayData.items && displayData.items.length > 0}
            <div class="mt-4 space-y-2">
              {#each displayData.items as item}
                <div class="bg-gray-50 border border-gray-200 rounded-md p-3">
                  <p class="text-sm font-semibold text-gray-900 mb-1">{item.title}</p>
                  {#each item.fields as field}
                    <div class="flex items-baseline gap-3 pl-2">
                      <span class="text-xs text-gray-500 min-w-[120px] flex-shrink-0">{field.label}</span>
                      <span class="text-sm text-gray-900">{field.value}</span>
                    </div>
                  {/each}
                </div>
              {/each}
            </div>
          {/if}
          <button
            onclick={() => showRawPayload = true}
            class="mt-4 text-xs text-gray-500 hover:text-gray-700 border border-gray-300 rounded px-2 py-1"
          >Show raw payload</button>
        {:else}
          <pre class="bg-gray-50 rounded-md p-4 text-sm font-mono text-gray-800 overflow-x-auto">{formatJson(req.payload)}</pre>
          {#if displayData}
            <button
              onclick={() => showRawPayload = false}
              class="mt-4 text-xs text-gray-500 hover:text-gray-700 border border-gray-300 rounded px-2 py-1"
            >Show formatted view</button>
          {/if}
        {/if}
      </div>
    {:else if activeTab === 'audit'}
      {#if auditLogs.length === 0}
        <div class="bg-white shadow-sm rounded-lg border border-gray-200 p-6">
          <p class="text-sm text-gray-500">No audit entries yet.</p>
        </div>
      {:else}
        <div class="space-y-3">
          {#each auditLogs as log}
            <div class="bg-white shadow-sm rounded-lg border border-gray-200 p-4">
              <div class="flex items-start justify-between">
                <div class="flex items-center gap-2">
                  <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium {actionColor(log.action)}">
                    {log.action}
                  </span>
                  <span class="text-sm text-gray-700">by <span class="font-medium">{log.actor_id}</span></span>
                </div>
                <span class="text-xs text-gray-500">{formatDate(log.created_at)}</span>
              </div>
              {#if log.details && Object.keys(log.details).length > 0}
                <span class="mt-2 block text-xs text-gray-600 whitespace-pre-line">{formatDetails(log.details, log.action)}</span>
              {/if}
            </div>
          {/each}
        </div>
      {/if}
    {:else if activeTab === 'deliveries'}
      {#if outboxEntries.length === 0}
        <div class="bg-white shadow-sm rounded-lg border border-gray-200 p-6">
          <p class="text-sm text-gray-500">No delivery entries for this request.</p>
        </div>
      {:else}
        {#if failedCount > 0}
          <div class="mb-3 flex justify-end">
            <button
              onclick={retryAllFailed}
              disabled={retryingAll}
              class="text-sm bg-red-50 text-red-700 hover:bg-red-100 border border-red-200 rounded-md px-3 py-1.5 disabled:opacity-50"
            >
              {retryingAll ? 'Retrying…' : `Retry All Failed (${failedCount})`}
            </button>
          </div>
        {/if}
        <div class="bg-white shadow-sm rounded-lg border border-gray-200 overflow-hidden">
          <table class="min-w-full divide-y divide-gray-200">
            <thead class="bg-gray-50">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Webhook URL</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Attempts</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Last Error</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Created</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Actions</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200">
              {#each outboxEntries as entry}
                <tr class="hover:bg-gray-50">
                  <td class="px-4 py-3 text-xs font-mono text-gray-700 max-w-[250px] truncate" title={entry.webhook_url}>{entry.webhook_url}</td>
                  <td class="px-4 py-3">
                    <span class="inline-flex px-2 py-0.5 rounded-full text-xs font-medium {deliveryStatusClass(entry.status)}">{entry.status}</span>
                  </td>
                  <td class="px-4 py-3 text-sm text-gray-700">{entry.attempts} / {entry.max_retries}</td>
                  <td class="px-4 py-3 text-xs text-red-600 max-w-[200px] truncate" title={entry.last_error || ''}>{entry.last_error || '—'}</td>
                  <td class="px-4 py-3 text-xs text-gray-500">{formatDate(entry.created_at)}</td>
                  <td class="px-4 py-3">
                    {#if entry.status === 'failed'}
                      <button
                        onclick={() => retryEntry(entry.id)}
                        disabled={retryingId === entry.id}
                        class="text-xs text-indigo-600 hover:text-indigo-800 disabled:opacity-50 disabled:cursor-not-allowed"
                      >
                        {retryingId === entry.id ? 'Retrying…' : 'Retry'}
                      </button>
                    {:else}
                      <span class="text-xs text-gray-400">—</span>
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    {/if}
  {/if}
</div>
