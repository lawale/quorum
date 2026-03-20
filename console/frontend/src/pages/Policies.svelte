<script lang="ts">
  import { policies as policiesApi, requests as requestsApi } from '../lib/api';
  import { addToast, selectedTenant, availableTenants } from '../lib/stores';
  import { formatDate } from '../lib/utils';
  import LoadingSpinner from '../components/LoadingSpinner.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import type { Policy } from '../lib/types';

  let items: Policy[] = $state([]);
  let isLoading = $state(true);
  let totalPolicies = $state(0);
  let pendingRequests = $state(0);
  let page = $state(1);
  const perPage = 20;
  let totalPages = $derived(Math.max(1, Math.ceil(totalPolicies / perPage)));

  let tenantMap: Record<string, string> = $state({});
  availableTenants.subscribe((tenants) => {
    const map: Record<string, string> = {};
    for (const t of tenants) { map[t.id] = t.slug; }
    tenantMap = map;
  });

  // Re-fetch when tenant selection changes
  let currentTenant = $state('');
  selectedTenant.subscribe((v) => { currentTenant = v; page = 1; loadPolicies(); });

  $effect(() => { loadPolicies(); });

  async function loadPolicies() {
    isLoading = true;
    try {
      const [policiesRes, requestsRes] = await Promise.all([
        policiesApi.list({ page, per_page: perPage }),
        requestsApi.list({ status: 'pending' }),
      ]);
      items = policiesRes.data || [];
      totalPolicies = policiesRes.total ?? items.length;
      pendingRequests = requestsRes.total ?? (requestsRes.data || []).length;
    } catch { addToast('Failed to load policies', 'error'); }
    finally { isLoading = false; }
  }

  async function handleDelete(id: string, name: string) {
    if (!confirm(`Delete policy "${name}"?`)) return;
    try {
      await policiesApi.delete(id);
      items = items.filter((p) => p.id !== id);
      totalPolicies = Math.max(0, totalPolicies - 1);
      addToast('Policy deleted', 'success');
    } catch { addToast('Failed to delete policy', 'error'); }
  }
</script>

<div>
  <!-- Header -->
  <section class="flex flex-col md:flex-row md:items-end justify-between gap-6 mb-10">
    <div>
      <p class="text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mb-2">Governance</p>
      <h1 class="text-4xl font-extrabold tracking-tight text-on-surface mb-2">Policies</h1>
      <p class="text-on-surface-variant max-w-lg">Define and manage quorum requirements, execution stages, and automated expiry rules across your distributed network.</p>
    </div>
    <a href="#/policies/new" class="bg-gradient-to-br from-primary to-primary-container text-on-primary px-5 py-2.5 rounded-md font-semibold text-sm shadow-lg shadow-primary/20 hover:brightness-110 transition-all flex items-center gap-2 shrink-0">
      + Create Policy
    </a>
  </section>

  <!-- Summary Cards -->
  <section class="grid grid-cols-1 sm:grid-cols-2 gap-6 mb-10">
    <div class="bg-surface-container-lowest p-6 rounded-xl shadow-ambient-lg">
      <p class="text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mb-1">Active Fleet</p>
      <div class="flex items-baseline gap-2">
        <p class="text-[44px] font-black tracking-tighter text-on-surface leading-none">{totalPolicies}</p>
      </div>
      <p class="text-on-surface-variant font-medium mt-1">Policies</p>
    </div>
    <div class="bg-surface-container-lowest p-6 rounded-xl shadow-ambient-lg">
      <p class="text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mb-1">Pending Approval</p>
      <div class="flex items-baseline gap-2">
        <p class="text-[44px] font-black tracking-tighter text-on-surface leading-none">{pendingRequests}</p>
      </div>
      <p class="text-on-surface-variant font-medium mt-1">Requests</p>
    </div>
  </section>

  {#if isLoading}
    <LoadingSpinner />
  {:else if items.length === 0}
    <EmptyState message="No policies configured yet." actionLabel="Create your first policy" onaction={() => { window.location.hash = '#/policies/new'; }} />
  {:else}
    <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl overflow-hidden">
      <table class="min-w-full divide-y divide-outline-variant/15">
        <thead class="bg-surface-container-low">
          <tr>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant border-b border-outline-variant/15">Name</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant border-b border-outline-variant/15">Request Type</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant border-b border-outline-variant/15">Stages</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant border-b border-outline-variant/15">Auto Expire</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant border-b border-outline-variant/15">Tenant</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant border-b border-outline-variant/15">Created At</th>
            <th class="px-6 py-3 text-right text-[10px] font-bold uppercase tracking-widest text-on-surface-variant border-b border-outline-variant/15">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-outline-variant/15">
          {#each items as policy}
            <tr class="hover:bg-surface-container-low">
              <td class="px-6 py-5 text-sm font-medium text-on-surface">
                <a href="#/policies/{policy.id}" class="text-primary-container hover:text-primary">{policy.name}</a>
              </td>
              <td class="px-6 py-5 text-sm text-on-surface-variant"><code class="bg-surface-container px-2 py-0.5 rounded text-xs">{policy.request_type}</code></td>
              <td class="px-6 py-5 text-sm text-on-surface-variant">{policy.stages.length} stage{policy.stages.length !== 1 ? 's' : ''}</td>
              <td class="px-6 py-5 text-sm text-on-surface-variant">{policy.auto_expire_duration || '—'}</td>
              <td class="px-6 py-5 text-sm text-on-surface-variant"><code class="bg-surface-container px-2 py-0.5 rounded text-xs">{tenantMap[policy.tenant_id] || policy.tenant_id}</code></td>
              <td class="px-6 py-5 text-sm text-on-surface-variant">{formatDate(policy.created_at)}</td>
              <td class="px-6 py-5 text-right text-sm">
                <a href="#/policies/{policy.id}" class="text-primary-container hover:text-primary mr-3">Edit</a>
                <button onclick={() => handleDelete(policy.id, policy.name)} class="text-status-rejected-text hover:text-red-800">Delete</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <div class="flex items-center justify-between mt-6">
      <span class="text-sm text-on-surface-variant">Showing {(page - 1) * perPage + 1}–{Math.min(page * perPage, totalPolicies)} of {totalPolicies} policies</span>
      {#if totalPages > 1}
        <div class="flex items-center gap-3">
          <span class="text-sm text-on-surface-variant">Page {page} of {totalPages}</span>
          <div class="flex gap-1">
            <button onclick={() => { page = Math.max(1, page - 1); loadPolicies(); }} disabled={page <= 1} class="px-2.5 py-1.5 text-sm border border-outline-variant/40 rounded-md hover:bg-surface-container-low disabled:opacity-50 disabled:cursor-not-allowed transition-colors" aria-label="Previous page">
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" /></svg>
            </button>
            <button onclick={() => { page = Math.min(totalPages, page + 1); loadPolicies(); }} disabled={page >= totalPages} class="px-2.5 py-1.5 text-sm border border-outline-variant/40 rounded-md hover:bg-surface-container-low disabled:opacity-50 disabled:cursor-not-allowed transition-colors" aria-label="Next page">
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" /></svg>
            </button>
          </div>
        </div>
      {/if}
    </div>
  {/if}
</div>
