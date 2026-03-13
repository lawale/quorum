<script lang="ts">
  import { policies as policiesApi } from '../lib/api';
  import { addToast } from '../lib/stores';
  import { formatDate } from '../lib/utils';
  import LoadingSpinner from '../components/LoadingSpinner.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import type { Policy } from '../lib/types';

  let items: Policy[] = $state([]);
  let isLoading = $state(true);

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
    <h1 class="text-2xl font-bold text-gray-900">Policies</h1>
    <a href="#/policies/new" class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-indigo-600 rounded-md hover:bg-indigo-700 transition-colors">
      Create Policy
    </a>
  </div>

  {#if isLoading}
    <LoadingSpinner />
  {:else if items.length === 0}
    <EmptyState message="No policies configured yet." actionLabel="Create your first policy" onaction={() => { window.location.hash = '#/policies/new'; }} />
  {:else}
    <div class="bg-white shadow-sm rounded-lg border border-gray-200 overflow-hidden">
      <table class="min-w-full divide-y divide-gray-200">
        <thead class="bg-gray-50">
          <tr>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Name</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Request Type</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Stages</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Created</th>
            <th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200">
          {#each items as policy}
            <tr class="hover:bg-gray-50">
              <td class="px-6 py-4 text-sm font-medium text-gray-900">
                <a href="#/policies/{policy.id}" class="text-indigo-600 hover:text-indigo-800">{policy.name}</a>
              </td>
              <td class="px-6 py-4 text-sm text-gray-500"><code class="bg-gray-100 px-2 py-0.5 rounded text-xs">{policy.request_type}</code></td>
              <td class="px-6 py-4 text-sm text-gray-500">{policy.stages.length} stage{policy.stages.length !== 1 ? 's' : ''}</td>
              <td class="px-6 py-4 text-sm text-gray-500">{formatDate(policy.created_at)}</td>
              <td class="px-6 py-4 text-right text-sm">
                <a href="#/policies/{policy.id}" class="text-indigo-600 hover:text-indigo-800 mr-3">Edit</a>
                <button onclick={() => handleDelete(policy.id, policy.name)} class="text-red-600 hover:text-red-800">Delete</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>
