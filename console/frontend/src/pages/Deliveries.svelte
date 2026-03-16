<script lang="ts">
  import { deliveries as deliveriesApi } from '../lib/api';
  import { selectedTenant, addToast } from '../lib/stores';
  import { formatDate } from '../lib/utils';
  import LoadingSpinner from '../components/LoadingSpinner.svelte';
  import type { OutboxEntry } from '../lib/types';

  let entries: OutboxEntry[] = $state([]);
  let total = $state(0);
  let page = $state(1);
  let isLoading = $state(true);
  let statusFilter = $state('');
  let retryingId: string | null = $state(null);

  selectedTenant.subscribe(() => { page = 1; loadEntries(); });

  $effect(() => {
    loadEntries();
  });

  async function loadEntries() {
    isLoading = true;
    try {
      const res = await deliveriesApi.list({
        page,
        per_page: 20,
        status: statusFilter || undefined,
      });
      entries = res.data || [];
      total = res.total ?? 0;
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

  function statusBadgeClass(status: string): string {
    switch (status) {
      case 'delivered': return 'bg-green-100 text-green-800';
      case 'failed': return 'bg-red-100 text-red-800';
      case 'processing': return 'bg-blue-100 text-blue-800';
      case 'pending': return 'bg-yellow-100 text-yellow-800';
      default: return 'bg-gray-100 text-gray-600';
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
  <div class="flex items-center justify-between mb-6">
    <h1 class="text-2xl font-bold text-gray-900">Deliveries</h1>
    <div class="flex items-center gap-3">
      <select
        bind:value={statusFilter}
        onchange={() => { page = 1; loadEntries(); }}
        class="text-sm border border-gray-300 rounded-md px-3 py-1.5 focus:outline-none focus:ring-2 focus:ring-indigo-500"
      >
        <option value="">All statuses</option>
        <option value="pending">Pending</option>
        <option value="processing">Processing</option>
        <option value="delivered">Delivered</option>
        <option value="failed">Failed</option>
      </select>
    </div>
  </div>

  {#if isLoading}
    <LoadingSpinner />
  {:else if entries.length === 0}
    <div class="bg-white shadow-sm rounded-lg border border-gray-200 p-8 text-center">
      <p class="text-gray-500">No delivery entries found.</p>
    </div>
  {:else}
    <div class="bg-white shadow-sm rounded-lg border border-gray-200 overflow-hidden">
      <table class="min-w-full divide-y divide-gray-200">
        <thead class="bg-gray-50">
          <tr>
            <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">ID</th>
            <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Webhook URL</th>
            <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
            <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Attempts</th>
            <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Last Error</th>
            <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Created</th>
            <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200">
          {#each entries as entry}
            <tr class="hover:bg-gray-50">
              <td class="px-4 py-3">
                <a href="#/requests/{entry.request_id}" class="text-xs font-mono text-indigo-600 hover:text-indigo-800">{truncateId(entry.id)}</a>
              </td>
              <td class="px-4 py-3 text-xs font-mono text-gray-700" title={entry.webhook_url}>{truncateUrl(entry.webhook_url)}</td>
              <td class="px-4 py-3">
                <span class="inline-flex px-2 py-0.5 rounded-full text-xs font-medium {statusBadgeClass(entry.status)}">{entry.status}</span>
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

    <!-- Pagination -->
    {#if totalPages > 1}
      <div class="flex items-center justify-between mt-4">
        <p class="text-sm text-gray-500">{total} total entries</p>
        <div class="flex gap-2">
          <button
            onclick={() => { page = Math.max(1, page - 1); loadEntries(); }}
            disabled={page <= 1}
            class="px-3 py-1 text-sm border rounded-md disabled:opacity-50"
          >Prev</button>
          <span class="px-3 py-1 text-sm text-gray-600">Page {page} of {totalPages}</span>
          <button
            onclick={() => { page = Math.min(totalPages, page + 1); loadEntries(); }}
            disabled={page >= totalPages}
            class="px-3 py-1 text-sm border rounded-md disabled:opacity-50"
          >Next</button>
        </div>
      </div>
    {/if}
  {/if}
</div>
