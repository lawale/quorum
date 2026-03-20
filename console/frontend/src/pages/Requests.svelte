<script lang="ts">
  import { requests as requestsApi, policies as policiesApi } from '../lib/api';
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
  let searchQuery = $state('');
  let requestTypes: string[] = $state([]);
  let searchTimeout: ReturnType<typeof setTimeout> | null = null;

  let totalPages = $derived(Math.ceil(total / perPage));

  // Re-fetch when tenant selection changes
  let currentTenant = $state('');
  selectedTenant.subscribe((v) => { currentTenant = v; page = 1; loadRequests(); loadRequestTypes(); });

  $effect(() => {
    loadRequests();
    loadRequestTypes();
  });

  async function loadRequestTypes() {
    try {
      const res = await policiesApi.requestTypes();
      requestTypes = res.data || [];
    } catch {
      // silently fail
    }
  }

  async function loadRequests() {
    isLoading = true;
    try {
      const res = await requestsApi.list({
        page,
        per_page: perPage,
        status: statusFilter || undefined,
        type: typeFilter || undefined,
        search: searchQuery || undefined,
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

  function onSearchInput() {
    if (searchTimeout) clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      page = 1;
      loadRequests();
    }, 300);
  }

  function clearFilters() {
    statusFilter = '';
    typeFilter = '';
    searchQuery = '';
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
  <!-- Page Header -->
  <section class="flex flex-col md:flex-row md:items-end justify-between gap-6 mb-10">
    <div>
      <h1 class="text-4xl font-extrabold tracking-tight text-on-surface mb-2">Requests</h1>
      <p class="text-on-surface-variant max-w-lg">Monitor and manage access requests across all organizational units.</p>
    </div>
  </section>

  <!-- Filter Bar -->
  <div class="flex items-center gap-4 mb-6">
    <div class="flex-1 relative">
      <input
        type="text"
        placeholder="Search by ID, type, or maker..."
        bind:value={searchQuery}
        oninput={onSearchInput}
        class="w-full px-4 py-2.5 text-sm border border-outline-variant/40 rounded-lg bg-surface-container-lowest focus:outline-none focus:ring-2 focus:ring-primary"
      />
    </div>
    <select
      bind:value={statusFilter}
      onchange={applyFilters}
      class="px-4 py-2.5 text-sm border border-outline-variant/40 rounded-lg bg-surface-container-lowest focus:outline-none focus:ring-2 focus:ring-primary"
    >
      <option value="">All Statuses</option>
      <option value="pending">Pending</option>
      <option value="approved">Approved</option>
      <option value="rejected">Rejected</option>
      <option value="cancelled">Cancelled</option>
      <option value="expired">Expired</option>
    </select>
    <select
      bind:value={typeFilter}
      onchange={applyFilters}
      class="px-4 py-2.5 text-sm border border-outline-variant/40 rounded-lg bg-surface-container-lowest focus:outline-none focus:ring-2 focus:ring-primary"
    >
      <option value="">All Types</option>
      {#each requestTypes as rt}
        <option value={rt}>{rt}</option>
      {/each}
    </select>
    <button
      onclick={() => { if (statusFilter || typeFilter || searchQuery) clearFilters(); }}
      class="flex items-center gap-2 px-4 py-2.5 text-sm font-medium border border-outline-variant/40 rounded-lg hover:bg-surface-container-low transition-colors"
    >
      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
        <path stroke-linecap="round" stroke-linejoin="round" d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
      </svg>
      Filters
    </button>
  </div>

  {#if isLoading}
    <LoadingSpinner />
  {:else if items.length === 0}
    <EmptyState message="No requests found." />
  {:else}
    <div class="bg-surface-container-lowest shadow-ambient-lg rounded-xl overflow-hidden">
      <table class="min-w-full">
        <thead>
          <tr class="border-b border-outline-variant/15">
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">ID</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Type</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Status</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Maker</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Stage</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Created</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Expires</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-outline-variant/15">
          {#each items as req}
            <tr class="hover:bg-surface-container-low transition-colors">
              <td class="px-6 py-5 text-sm font-mono text-xs text-on-surface">
                <a href="#/requests/{req.id}" class="text-primary-container hover:text-primary" title={req.id}>{truncateId(req.id)}</a>
              </td>
              <td class="px-6 py-5 text-sm text-on-surface">{req.type}</td>
              <td class="px-6 py-5 text-sm"><StatusBadge status={req.status} /></td>
              <td class="px-6 py-5 text-sm text-on-surface-variant">{req.maker_id}</td>
              <td class="px-6 py-5 text-sm text-on-surface-variant">{req.current_stage + 1}{req.total_stages ? ` / ${req.total_stages}` : ''}</td>
              <td class="px-6 py-5 text-sm text-on-surface-variant">{formatDate(req.created_at)}</td>
              <td class="px-6 py-5 text-sm text-on-surface-variant">{req.expires_at ? formatDate(req.expires_at) : '—'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <!-- Pagination -->
    {#if totalPages > 1}
      <div class="flex items-center justify-between mt-6">
        <div class="flex items-center gap-3">
          <span class="text-sm text-on-surface-variant">Page {page} of {totalPages}</span>
          <div class="flex gap-1">
            <button
              onclick={() => goToPage(page - 1)}
              disabled={page <= 1}
              class="px-2.5 py-1.5 text-sm border border-outline-variant/40 rounded-md hover:bg-surface-container-low disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              aria-label="Previous page"
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" />
              </svg>
            </button>
            <button
              onclick={() => goToPage(page + 1)}
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
        <span class="text-sm text-on-surface-variant">Show {perPage} per page</span>
      </div>
    {/if}
  {/if}
</div>
