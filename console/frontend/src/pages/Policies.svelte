<script lang="ts">
  import { policies as policiesApi } from '../lib/api';
  import { addToast, selectedTenant } from '../lib/stores';
  import { formatDate } from '../lib/utils';
  import LoadingSpinner from '../components/LoadingSpinner.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import type { Policy } from '../lib/types';

  let items: Policy[] = $state([]);
  let isLoading = $state(true);

  // Re-fetch when tenant selection changes
  let currentTenant = $state('');
  selectedTenant.subscribe((v) => { currentTenant = v; loadPolicies(); });

  $effect(() => { loadPolicies(); });

  async function loadPolicies() {
    isLoading = true;
    try {
      const res = await policiesApi.list();
      items = res.data || [];
    } catch { addToast('Failed to load policies', 'error'); }
    finally { isLoading = false; }
  }

  async function handleDelete(id: string, name: string) {
    if (!confirm(`Delete policy "${name}"?`)) return;
    try {
      await policiesApi.delete(id);
      items = items.filter((p) => p.id !== id);
      addToast('Policy deleted', 'success');
    } catch { addToast('Failed to delete policy', 'error'); }
  }
</script>

<div>
  <div class="flex items-center justify-between mb-6">
    <h1 class="text-2xl font-bold text-on-surface">Policies</h1>
    <a href="#/policies/new" class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-gradient-to-br from-primary to-primary-container rounded-md hover:brightness-110 transition-all">
      Create Policy
    </a>
  </div>

  {#if isLoading}
    <LoadingSpinner />
  {:else if items.length === 0}
    <EmptyState message="No policies configured yet." actionLabel="Create your first policy" onaction={() => { window.location.hash = '#/policies/new'; }} />
  {:else}
    <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl overflow-hidden">
      <table class="min-w-full divide-y divide-outline-variant/15">
        <thead class="bg-surface-container-low">
          <tr>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Name</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Tenant</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Request Type</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Stages</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Created</th>
            <th class="px-6 py-3 text-right text-xs font-medium text-on-surface-variant uppercase">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-outline-variant/15">
          {#each items as policy}
            <tr class="hover:bg-surface-container-low">
              <td class="px-6 py-4 text-sm font-medium text-on-surface">
                <a href="#/policies/{policy.id}" class="text-primary-container hover:text-primary">{policy.name}</a>
              </td>
              <td class="px-6 py-4 text-sm text-on-surface-variant"><code class="bg-surface-container px-2 py-0.5 rounded text-xs">{policy.tenant_id}</code></td>
              <td class="px-6 py-4 text-sm text-on-surface-variant"><code class="bg-surface-container px-2 py-0.5 rounded text-xs">{policy.request_type}</code></td>
              <td class="px-6 py-4 text-sm text-on-surface-variant">{policy.stages.length} stage{policy.stages.length !== 1 ? 's' : ''}</td>
              <td class="px-6 py-4 text-sm text-on-surface-variant">{formatDate(policy.created_at)}</td>
              <td class="px-6 py-4 text-right text-sm">
                <a href="#/policies/{policy.id}" class="text-primary-container hover:text-primary mr-3">Edit</a>
                <button onclick={() => handleDelete(policy.id, policy.name)} class="text-status-rejected-text hover:text-red-800">Delete</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>
