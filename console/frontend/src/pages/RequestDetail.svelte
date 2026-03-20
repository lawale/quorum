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
  let showPayload = $state(false);
  let retryingId: string | null = $state(null);
  let retryingAll = $state(false);
  let deliveryPage = $state(1);
  let deliveryTotal = $state(0);
  const deliveryPerPage = 20;
  let loadingMore = $state(false);
  let auditTrailEl: HTMLElement | undefined = $state(undefined);

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

  function actionDotColor(action: string): string {
    switch (action) {
      case 'created': return 'bg-primary-container';
      case 'approved':
      case 'stage_advanced': return 'bg-emerald-500';
      case 'rejected': return 'bg-red-500';
      case 'cancelled': return 'bg-gray-400';
      case 'expired': return 'bg-orange-500';
      default: return 'bg-surface-container-high';
    }
  }

  function formatActionLabel(action: string): string {
    return action.split('_').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
  }

  function truncateId(val: string): string {
    if (val.length <= 12) return val;
    return val.slice(0, 12) + '...';
  }

  /**
   * Derive the total number of stages and how many are completed
   * from audit logs and the current request state.
   */
  function deriveStages(logs: AuditLog[], request: Request): { total: number; completed: number } {
    // Count stage_advanced actions to know how many stages have been passed
    const advancedCount = logs.filter(l => l.action === 'stage_advanced').length;
    // The total stages is at least current_stage, but also consider advanced stages
    // current_stage is 0-indexed; after all stages pass the request is approved
    // We estimate total stages as max(current_stage, advancedCount) + 1 if still pending
    // If approved/rejected, advancedCount tells us how many stages existed
    const isTerminal = request.status === 'approved' || request.status === 'rejected' || request.status === 'cancelled' || request.status === 'expired';

    if (isTerminal && request.status === 'approved') {
      // All stages completed
      const total = advancedCount > 0 ? advancedCount : request.current_stage + 1;
      return { total, completed: total };
    }

    if (isTerminal) {
      // Rejected/cancelled/expired - stopped at current_stage
      const total = Math.max(request.current_stage + 1, advancedCount + 1);
      return { total, completed: advancedCount };
    }

    // Still pending
    const total = Math.max(request.current_stage + 1, advancedCount + 1);
    return { total, completed: advancedCount };
  }

  let failedCount = $derived(outboxEntries.filter(e => e.status === 'failed').length);
  let displayData = $derived(req ? getDisplay(req.metadata) : null);
  let stages = $derived(req ? deriveStages(auditLogs, req) : { total: 0, completed: 0 });
  let verificationLogs = $derived(auditLogs.filter(l => l.action === 'approved' || l.action === 'rejected'));
</script>

<div>
  {#if isLoading}
    <LoadingSpinner />
  {:else if req}
    <!-- Breadcrumb -->
    <div class="mb-2">
      <a href="#/requests" class="text-sm text-on-surface-variant hover:text-on-surface">Requests</a>
      <span class="text-on-surface-variant/60 mx-1">&gt;</span>
      <span class="text-sm text-on-surface font-mono">{truncateId(id)}</span>
    </div>

    <!-- Header -->
    <section class="flex flex-col md:flex-row md:items-start justify-between gap-6 mb-10">
      <div>
        <div class="flex items-center gap-3 mb-2">
          <h1 class="text-4xl font-extrabold tracking-tight text-on-surface">Request Details</h1>
          <StatusBadge status={req.status} />
        </div>
        <div class="flex flex-wrap items-center gap-4 text-sm text-on-surface-variant">
          <span>Maker: <span class="font-medium text-on-surface">{req.maker_id}</span></span>
          <span>Created: <span class="font-medium text-on-surface">{formatDate(req.created_at)}</span></span>
          <span>Policy: <span class="font-medium text-on-surface">{req.type}</span></span>
        </div>
      </div>
      <div class="flex gap-3">
        <button class="bg-surface-container text-on-surface px-4 py-2 rounded-md font-medium text-sm hover:brightness-95 transition-all">
          Export PDF
        </button>
        <button
          onclick={() => auditTrailEl?.scrollIntoView({ behavior: 'smooth' })}
          class="bg-surface-container text-on-surface px-4 py-2 rounded-md font-medium text-sm hover:brightness-95 transition-all"
        >
          View Audit Log
        </button>
      </div>
    </section>

    <!-- Two-column layout -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-10">
      <!-- Left column (2/3) -->
      <div class="lg:col-span-2 space-y-8">

        <!-- Request Content -->
        <section>
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-xl font-bold tracking-tight text-on-surface">Request Content</h2>
            <span class="text-[10px] font-bold uppercase tracking-widest text-on-surface-variant bg-surface-container-low px-2 py-1 rounded">Manual Submission</span>
          </div>
          <div class="bg-surface-container-lowest rounded-xl shadow-ambient-lg p-6">
            {#if displayData}
              {#if displayData.title}
                <h3 class="text-base font-semibold text-on-surface mb-4">{displayData.title}</h3>
              {/if}
              <div class="space-y-3">
                {#each displayData.fields as field}
                  <div>
                    <span class="text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">{field.label}</span>
                    <p class="text-sm text-on-surface mt-0.5">{field.value}</p>
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
                          <span class="text-[10px] font-bold uppercase tracking-widest text-on-surface-variant min-w-[120px] flex-shrink-0">{field.label}</span>
                          <span class="text-sm text-on-surface">{field.value}</span>
                        </div>
                      {/each}
                    </div>
                  {/each}
                </div>
              {/if}
            {:else}
              <pre class="bg-surface-container-low rounded-md p-4 text-sm font-mono text-on-surface overflow-x-auto">{formatJson(req.payload)}</pre>
            {/if}
          </div>
        </section>

        <!-- Approval Progress -->
        <section>
          <h2 class="text-xl font-bold tracking-tight text-on-surface mb-4">Approval Progress</h2>
          <div class="bg-surface-container-lowest rounded-xl shadow-ambient-lg p-6">
            <div class="flex items-center">
              <!-- START dot -->
              <div class="flex flex-col items-center flex-shrink-0">
                <div class="w-8 h-8 rounded-full bg-emerald-500 flex items-center justify-center">
                  <svg class="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3"><path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                </div>
                <span class="text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mt-2">Start</span>
              </div>

              <!-- Stage dots -->
              {#each Array(stages.total) as _, i}
                <!-- Connector line -->
                <div class="flex-1 h-0.5 mx-2 {i < stages.completed ? 'bg-emerald-500' : 'bg-surface-container'}"></div>
                <!-- Stage dot -->
                <div class="flex flex-col items-center flex-shrink-0">
                  {#if i < stages.completed}
                    <div class="w-8 h-8 rounded-full bg-emerald-500 flex items-center justify-center">
                      <svg class="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3"><path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                    </div>
                  {:else if i === stages.completed && req.status === 'pending'}
                    <div class="w-8 h-8 rounded-full bg-primary-container flex items-center justify-center text-on-primary text-xs font-bold">{i + 1}</div>
                  {:else if req.status === 'rejected' && i === stages.completed}
                    <div class="w-8 h-8 rounded-full bg-red-500 flex items-center justify-center">
                      <svg class="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
                    </div>
                  {:else}
                    <div class="w-8 h-8 rounded-full bg-surface-container flex items-center justify-center text-on-surface-variant text-xs font-bold">{i + 1}</div>
                  {/if}
                  <span class="text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mt-2">Stage {i + 1}</span>
                </div>
              {/each}
            </div>
          </div>
        </section>

        <!-- Verification History -->
        <section>
          <h2 class="text-xl font-bold tracking-tight text-on-surface mb-4">Verification History</h2>
          <div class="bg-surface-container-lowest rounded-xl shadow-ambient-lg overflow-hidden">
            {#if verificationLogs.length === 0}
              <div class="p-6">
                <p class="text-sm text-on-surface-variant">No verification decisions yet.</p>
              </div>
            {:else}
              <table class="min-w-full divide-y divide-outline-variant/15">
                <thead class="bg-surface-container-low">
                  <tr>
                    <th class="px-4 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Checker</th>
                    <th class="px-4 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Decision</th>
                    <th class="px-4 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Stage</th>
                    <th class="px-4 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Timestamp</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-outline-variant/15">
                  {#each verificationLogs as log}
                    <tr class="hover:bg-surface-container-low transition-colors">
                      <td class="px-4 py-3 text-sm text-on-surface font-medium">{log.actor_id}</td>
                      <td class="px-4 py-3">
                        <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium {actionColor(log.action)}">
                          {formatActionLabel(log.action)}
                        </span>
                      </td>
                      <td class="px-4 py-3 text-sm text-on-surface-variant">
                        {#if log.details && log.details.stage !== undefined}
                          Stage {log.details.stage}
                        {:else}
                          --
                        {/if}
                      </td>
                      <td class="px-4 py-3 text-xs text-on-surface-variant">{formatDate(log.created_at)}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            {/if}
          </div>
        </section>

        <!-- Deliveries -->
        {#if outboxEntries.length > 0}
          <section>
            <div class="flex items-center justify-between mb-4">
              <h2 class="text-xl font-bold tracking-tight text-on-surface">Deliveries</h2>
              {#if failedCount > 0}
                <button
                  onclick={retryAllFailed}
                  disabled={retryingAll}
                  class="text-sm bg-status-rejected-bg text-status-rejected-text hover:bg-red-100 border border-status-rejected-text/20 rounded-md px-3 py-1.5 disabled:opacity-50"
                >
                  {retryingAll ? 'Retrying...' : `Retry All Failed (${failedCount})`}
                </button>
              {/if}
            </div>
            <div class="bg-surface-container-lowest shadow-ambient-lg rounded-xl overflow-hidden">
              <table class="min-w-full divide-y divide-outline-variant/15">
                <thead class="bg-surface-container-low">
                  <tr>
                    <th class="px-4 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Webhook URL</th>
                    <th class="px-4 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Status</th>
                    <th class="px-4 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Attempts</th>
                    <th class="px-4 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Last Error</th>
                    <th class="px-4 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Created</th>
                    <th class="px-4 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Actions</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-outline-variant/15">
                  {#each outboxEntries as entry}
                    <tr class="hover:bg-surface-container-low transition-colors">
                      <td class="px-4 py-3 text-xs font-mono text-on-surface max-w-[250px] truncate" title={entry.webhook_url}>{entry.webhook_url}</td>
                      <td class="px-4 py-3">
                        <span class="inline-flex px-2 py-0.5 rounded-full text-xs font-medium {deliveryStatusClass(entry.status)}">{entry.status}</span>
                      </td>
                      <td class="px-4 py-3 text-sm text-on-surface">{entry.attempts} / {entry.max_retries}</td>
                      <td class="px-4 py-3 text-xs text-status-rejected-text max-w-[200px] truncate" title={entry.last_error || ''}>{entry.last_error || '--'}</td>
                      <td class="px-4 py-3 text-xs text-on-surface-variant">{formatDate(entry.created_at)}</td>
                      <td class="px-4 py-3">
                        {#if entry.status === 'failed'}
                          <button
                            onclick={() => retryEntry(entry.id)}
                            disabled={retryingId === entry.id}
                            class="text-xs text-primary-container hover:text-primary disabled:opacity-50 disabled:cursor-not-allowed"
                          >
                            {retryingId === entry.id ? 'Retrying...' : 'Retry'}
                          </button>
                        {:else}
                          <span class="text-xs text-on-surface-variant/60">--</span>
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
                  class="text-sm text-primary-container hover:text-primary border border-outline-variant/40 rounded-md px-4 py-2 disabled:opacity-50"
                >
                  {loadingMore ? 'Loading...' : `Load more (${outboxEntries.length} of ${deliveryTotal})`}
                </button>
              </div>
            {/if}
          </section>
        {/if}
      </div>

      <!-- Right column (1/3) - Audit Trail -->
      <div bind:this={auditTrailEl}>
        <h2 class="text-xl font-bold tracking-tight text-on-surface mb-4">Audit Trail</h2>
        {#if auditLogs.length === 0}
          <p class="text-sm text-on-surface-variant">No audit entries yet.</p>
        {:else}
          <div class="space-y-0">
            {#each auditLogs as log, i}
              <div class="flex gap-4">
                <!-- Timeline dot + line -->
                <div class="flex flex-col items-center">
                  <div class="w-3 h-3 rounded-full {actionDotColor(log.action)} ring-4 ring-surface flex-shrink-0"></div>
                  {#if i < auditLogs.length - 1}
                    <div class="w-0.5 flex-1 bg-surface-container min-h-[40px]"></div>
                  {/if}
                </div>
                <!-- Content -->
                <div class="pb-6">
                  <p class="text-sm font-bold text-on-surface">{formatActionLabel(log.action)}</p>
                  <p class="text-xs text-on-surface-variant">performed by {log.actor_id}</p>
                  {#if log.details && Object.keys(log.details).length > 0}
                    <p class="text-xs text-on-surface-variant/60 mt-1">{formatDetails(log.details, log.action)}</p>
                  {/if}
                  <p class="text-[10px] text-on-surface-variant/60 mt-1">{formatDate(log.created_at)}</p>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>

    <!-- Payload (collapsible) -->
    <section class="mt-8">
      <button
        onclick={() => showPayload = !showPayload}
        class="text-sm text-on-surface-variant hover:text-on-surface flex items-center gap-1"
      >
        Payload (click to {showPayload ? 'collapse' : 'expand'})
      </button>
      {#if showPayload}
        <div class="mt-2 bg-surface-container-lowest rounded-xl shadow-ambient-sm p-6">
          <pre class="bg-on-surface text-emerald-400 rounded-lg p-4 text-sm font-mono overflow-x-auto">{formatJson(req.payload)}</pre>
        </div>
      {/if}
    </section>
  {/if}
</div>
