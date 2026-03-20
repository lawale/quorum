<script lang="ts">
  import { deliveries as deliveriesApi } from '../lib/api';
  import { selectedTenant, addToast } from '../lib/stores';
  import { formatDate, copyToClipboard } from '../lib/utils';
  import LoadingSpinner from '../components/LoadingSpinner.svelte';
  import type { OutboxEntry } from '../lib/types';

  let entries: OutboxEntry[] = $state([]);
  let total = $state(0);
  let page = $state(1);
  let isLoading = $state(true);
  let statusFilter = $state('');
  let eventFilter = $state('');
  let retryingId: string | null = $state(null);
  let retryingAll = $state(false);
  let failedCount = $state(0);
  let copiedId: string | null = $state(null);

  async function handleCopyId(id: string) {
    if (await copyToClipboard(id)) {
      copiedId = id;
      setTimeout(() => { copiedId = null; }, 1500);
    }
  }

  selectedTenant.subscribe(() => { page = 1; loadEntries(); });

  $effect(() => {
    loadEntries();
  });

  async function loadEntries() {
    isLoading = true;
    try {
      const [res, stats] = await Promise.all([
        deliveriesApi.list({
          page,
          per_page: 20,
          status: statusFilter || undefined,
          event: eventFilter || undefined,
        }),
        deliveriesApi.stats().catch(() => ({} as Record<string, number>)),
      ]);
      entries = res.data || [];
      total = res.total ?? 0;
      failedCount = (stats as Record<string, number>).failed ?? 0;
    } catch {
      addToast('Failed to load deliveries', 'error');
    } finally {
      isLoading = false;
    }
  }

  async function retryEntry(id: string) {
    retryingId = id;
    try {
      await deliveriesApi.retry(id);
      addToast('Delivery queued for retry', 'success');
      await loadEntries();
    } catch {
      addToast('Failed to retry delivery', 'error');
    } finally {
      retryingId = null;
    }
  }

  async function retryAllFailed() {
    retryingAll = true;
    try {
      const res = await deliveriesApi.retryAllFailed();
      addToast(`Retried ${res.reset} failed deliveries`, 'success');
      await loadEntries();
    } catch {
      addToast('Failed to retry deliveries', 'error');
    } finally {
      retryingAll = false;
    }
  }

  function statusBadgeClass(status: string): string {
    switch (status) {
      case 'delivered': return 'bg-status-approved-bg text-status-approved-text';
      case 'failed': return 'bg-status-rejected-bg text-status-rejected-text';
      case 'processing': return 'bg-surface-container text-on-surface-variant';
      case 'pending': return 'bg-status-pending-bg text-status-pending-text';
      default: return 'bg-surface-container text-on-surface-variant';
    }
  }

  function truncateId(id: string): string {
    return id.length > 8 ? id.slice(0, 8) + '…' : id;
  }

  function truncateUrl(url: string): string {
    return url.length > 50 ? url.slice(0, 50) + '…' : url;
  }

  let totalPages = $derived(Math.max(1, Math.ceil(total / 20)));
</script>

<div>
  <!-- Page Header -->
  <section class="flex flex-col md:flex-row md:items-end justify-between gap-6 mb-10">
    <div>
      <h1 class="text-4xl font-extrabold tracking-tight text-on-surface mb-2">Deliveries</h1>
      <p class="text-on-surface-variant max-w-lg">Track webhook delivery status and manage retries for failed notifications.</p>
    </div>
  </section>

  <!-- Filter Bar -->
  <div class="flex items-center gap-4 mb-6">
    <select
      bind:value={statusFilter}
      onchange={() => { page = 1; loadEntries(); }}
      class="px-4 py-2.5 text-sm border border-outline-variant/40 rounded-lg bg-surface-container-lowest focus:outline-none focus:ring-2 focus:ring-primary"
    >
      <option value="">All Statuses</option>
      <option value="pending">Pending</option>
      <option value="processing">Processing</option>
      <option value="delivered">Delivered</option>
      <option value="failed">Failed</option>
    </select>
    <input
      type="text"
      placeholder="Filter by event..."
      bind:value={eventFilter}
      onchange={() => { page = 1; loadEntries(); }}
      class="px-4 py-2.5 text-sm border border-outline-variant/40 rounded-lg bg-surface-container-lowest focus:outline-none focus:ring-2 focus:ring-primary"
    />
    <div class="flex-1"></div>
    <button
      onclick={retryAllFailed}
      disabled={retryingAll || failedCount === 0}
      class="flex items-center gap-2 px-4 py-2.5 text-sm font-medium text-white bg-gradient-to-br from-primary to-primary-container rounded-lg hover:brightness-110 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
    >
      {#if retryingAll}
        Retrying...
      {:else}
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
        Retry All Failed
      {/if}
    </button>
  </div>

  {#if isLoading}
    <LoadingSpinner />
  {:else if entries.length === 0}
    <div class="bg-surface-container-lowest shadow-ambient-lg rounded-xl p-8 text-center">
      <p class="text-on-surface-variant">No delivery entries found.</p>
    </div>
  {:else}
    <div class="bg-surface-container-lowest shadow-ambient-lg rounded-xl overflow-hidden">
      <table class="min-w-full">
        <thead>
          <tr class="border-b border-outline-variant/15">
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Webhook URL</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Event</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Request ID</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Status</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Attempts</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Next Retry</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Created At</th>
            <th class="px-6 py-3 text-right text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-outline-variant/15">
          {#each entries as entry}
            <tr class="hover:bg-surface-container-low transition-colors">
              <td class="px-6 py-5 text-xs font-mono text-on-surface" title={entry.webhook_url}>{truncateUrl(entry.webhook_url)}</td>
              <td class="px-6 py-5 text-sm text-on-surface-variant">{entry.event_type || '—'}</td>
              <td class="px-6 py-5 text-xs font-mono">
                <span class="inline-flex items-center gap-1.5">
                  <a href="#/requests/{entry.request_id}" class="text-primary-container hover:text-primary" title={entry.request_id}>{truncateId(entry.request_id)}</a>
                  <button onclick={() => handleCopyId(entry.request_id)} class="text-on-surface-variant/40 hover:text-on-surface transition-colors" title="Copy ID">
                    {#if copiedId === entry.request_id}
                      <svg class="w-3.5 h-3.5 text-status-approved-text" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" /></svg>
                    {:else}
                      <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" /></svg>
                    {/if}
                  </button>
                </span>
              </td>
              <td class="px-6 py-5">
                <span class="inline-flex px-2.5 py-1 rounded-full text-xs font-semibold {statusBadgeClass(entry.status)}">{entry.status}</span>
              </td>
              <td class="px-6 py-5 text-sm text-on-surface">{entry.attempts} / {entry.max_retries}</td>
              <td class="px-6 py-5 text-xs text-on-surface-variant">{entry.next_retry_at ? formatDate(entry.next_retry_at) : '—'}</td>
              <td class="px-6 py-5 text-xs text-on-surface-variant">{formatDate(entry.created_at)}</td>
              <td class="px-6 py-5 text-right">
                {#if entry.status === 'failed'}
                  <button
                    onclick={() => retryEntry(entry.id)}
                    disabled={retryingId === entry.id}
                    class="text-xs font-medium text-primary-container hover:text-primary disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                  >
                    {retryingId === entry.id ? 'Retrying...' : 'Retry'}
                  </button>
                {:else}
                  <span class="text-xs text-on-surface-variant/40">—</span>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <!-- Pagination -->
    <div class="flex items-center justify-between mt-6">
      <span class="text-sm text-on-surface-variant">Showing {(page - 1) * 20 + 1}–{Math.min(page * 20, total)} of {total} deliveries</span>
      {#if totalPages > 1}
        <div class="flex items-center gap-3">
          <span class="text-sm text-on-surface-variant">Page {page} of {totalPages}</span>
          <div class="flex gap-1">
            <button
              onclick={() => { page = Math.max(1, page - 1); loadEntries(); }}
              disabled={page <= 1}
              class="px-2.5 py-1.5 text-sm border border-outline-variant/40 rounded-md hover:bg-surface-container-low disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              aria-label="Previous page"
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" />
              </svg>
            </button>
            <button
              onclick={() => { page = Math.min(totalPages, page + 1); loadEntries(); }}
              disabled={page >= totalPages}
              class="px-2.5 py-1.5 text-sm border border-outline-variant/40 rounded-md hover:bg-surface-container-low disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              aria-label="Next page"
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
              </svg>
            </button>
          </div>
        </div>
      {/if}
    </div>
  {/if}
</div>
