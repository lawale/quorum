<script lang="ts">
  import { requests as requestsApi } from '../lib/api';
  import { addToast, selectedTenant } from '../lib/stores';
  import { formatDate } from '../lib/utils';
  import StatusBadge from '../components/StatusBadge.svelte';
  import LoadingSpinner from '../components/LoadingSpinner.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import type { Request } from '../lib/types';

  let items: Request[] = $state([]);
  let isLoading = $state(true);
  let total = $state(0);
  let page = $state(1);
  let perPage = $state(20);
  let statusFilter = $state('');
  let typeFilter = $state('');

  let totalPages = $derived(Math.ceil(total / perPage));

  // Re-fetch when tenant selection changes
  let currentTenant = $state('');
  selectedTenant.subscribe((v) => { currentTenant = v; page = 1; loadRequests(); });

  $effect(() => {
    loadRequests();
  });

  async function loadRequests() {
    isLoading = true;
    try {
      const res = await requestsApi.list({
        page,
        per_page: perPage,
        status: statusFilter || undefined,
        type: typeFilter || undefined,
      });
      items = res.data || [];
      total = res.total ?? 0;
    } catch {
      addToast('Failed to load requests', 'error');
    } finally {
      isLoading = false;
    }
  }

  function applyFilters() {
    page = 1;
    loadRequests();
  }

  function clearFilters() {
    statusFilter = '';
    typeFilter = '';
    page = 1;
    loadRequests();
  }

  function goToPage(p: number) {
    page = p;
    loadRequests();
  }

  function truncateId(id: string): string {
    return id.length > 8 ? id.slice(0, 8) + '…' : id;
  }
</script>

<div>
  <div class="flex items-center justify-between mb-6">
    <h1 class="text-2xl font-bold text-on-surface">Requests</h1>
  </div>

  <!-- Filters -->
  <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl p-4 mb-4">
    <div class="flex items-end gap-4">
      <div>
        <label for="statusFilter" class="block text-xs font-medium text-on-surface-variant mb-1">Status</label>
        <select id="statusFilter" bind:value={statusFilter} class="px-3 py-1.5 text-sm border border-outline-variant/40 rounded-md">
          <option value="">All</option>
          <option value="pending">Pending</option>
          <option value="approved">Approved</option>
          <option value="rejected">Rejected</option>
          <option value="cancelled">Cancelled</option>
          <option value="expired">Expired</option>
        </select>
      </div>
      <div>
        <label for="typeFilter" class="block text-xs font-medium text-on-surface-variant mb-1">Type</label>
        <input id="typeFilter" type="text" bind:value={typeFilter} placeholder="e.g. transfer" class="px-3 py-1.5 text-sm border border-outline-variant/40 rounded-md" />
      </div>
      <button onclick={applyFilters} class="px-3 py-1.5 text-sm font-medium text-white bg-gradient-to-br from-primary to-primary-container rounded-md hover:brightness-110 transition-all">
        Filter
      </button>
      {#if statusFilter || typeFilter}
        <button onclick={clearFilters} class="px-3 py-1.5 text-sm text-on-surface-variant hover:text-on-surface">
          Clear
        </button>
      {/if}
    </div>
  </div>

  {#if isLoading}
    <LoadingSpinner />
  {:else if items.length === 0}
    <EmptyState message="No requests found." />
  {:else}
    <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl overflow-hidden">
      <table class="min-w-full divide-y divide-outline-variant/15">
        <thead class="bg-surface-container-low">
          <tr>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">ID</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Tenant</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Type</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Status</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Maker</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Stage</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Created</th>
            <th class="px-6 py-3 text-right text-xs font-medium text-on-surface-variant uppercase">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-outline-variant/15">
          {#each items as req}
            <tr class="hover:bg-surface-container-low">
              <td class="px-6 py-4 text-sm font-mono text-xs text-on-surface">
                <a href="#/requests/{req.id}" class="text-primary-container hover:text-primary" title={req.id}>{truncateId(req.id)}</a>
              </td>
              <td class="px-6 py-4 text-sm text-on-surface-variant"><code class="bg-surface-container px-2 py-0.5 rounded text-xs">{req.tenant_id}</code></td>
              <td class="px-6 py-4 text-sm text-on-surface">{req.type}</td>
              <td class="px-6 py-4 text-sm"><StatusBadge status={req.status} /></td>
              <td class="px-6 py-4 text-sm text-on-surface-variant">{req.maker_id}</td>
              <td class="px-6 py-4 text-sm text-on-surface-variant">{req.current_stage}</td>
              <td class="px-6 py-4 text-sm text-on-surface-variant">{formatDate(req.created_at)}</td>
              <td class="px-6 py-4 text-right text-sm">
                <a href="#/requests/{req.id}" class="text-primary-container hover:text-primary">View</a>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <!-- Pagination -->
    {#if totalPages > 1}
      <div class="flex items-center justify-between mt-4">
        <p class="text-sm text-on-surface-variant">
          Showing {(page - 1) * perPage + 1}–{Math.min(page * perPage, total)} of {total}
        </p>
        <div class="flex gap-1">
          <button
            onclick={() => goToPage(page - 1)}
            disabled={page <= 1}
            class="px-3 py-1.5 text-sm border border-outline-variant/40 rounded-md hover:bg-surface-container-low disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Previous
          </button>
          {#each Array.from({ length: totalPages }, (_, i) => i + 1) as p}
            {#if totalPages <= 7 || p === 1 || p === totalPages || Math.abs(p - page) <= 1}
              <button
                onclick={() => goToPage(p)}
                class="px-3 py-1.5 text-sm border rounded-md {p === page ? 'bg-gradient-to-br from-primary to-primary-container text-white border-primary-container' : 'border-outline-variant/40 hover:bg-surface-container-low'}"
              >
                {p}
              </button>
            {:else if p === 2 || p === totalPages - 1}
              <span class="px-2 py-1.5 text-sm text-on-surface-variant/60">…</span>
            {/if}
          {/each}
          <button
            onclick={() => goToPage(page + 1)}
            disabled={page >= totalPages}
            class="px-3 py-1.5 text-sm border border-outline-variant/40 rounded-md hover:bg-surface-container-low disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Next
          </button>
        </div>
      </div>
    {/if}
  {/if}
</div>
