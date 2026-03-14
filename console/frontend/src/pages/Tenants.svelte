<script lang="ts">
  import { tenants as tenantsApi } from '../lib/api';
  import { addToast, availableTenants } from '../lib/stores';
  import { formatDate } from '../lib/utils';
  import LoadingSpinner from '../components/LoadingSpinner.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import Modal from '../components/Modal.svelte';
  import type { Tenant } from '../lib/types';

  let items: Tenant[] = $state([]);
  let isLoading = $state(true);
  let showCreateModal = $state(false);

  // Create form state
  let newSlug = $state('');
  let newName = $state('');
  let creating = $state(false);
  let createError = $state('');

  $effect(() => {
    loadTenants();
  });

  async function loadTenants() {
    isLoading = true;
    try {
      const res = await tenantsApi.list();
      items = res.data || [];
      // Keep the global store in sync
      availableTenants.set(items);
    } catch {
      addToast('Failed to load tenants', 'error');
    } finally {
      isLoading = false;
    }
  }

  function openCreateModal() {
    newSlug = '';
    newName = '';
    createError = '';
    showCreateModal = true;
  }

  async function handleCreate(e: SubmitEvent) {
    e.preventDefault();
    createError = '';

    if (!newSlug.trim() || !newName.trim()) {
      createError = 'Slug and name are required';
      return;
    }

    creating = true;
    try {
      await tenantsApi.create(newSlug.trim(), newName.trim());
      addToast('Tenant created', 'success');
      showCreateModal = false;
      await loadTenants();
    } catch (err) {
      createError = err instanceof Error ? err.message : 'Failed to create tenant';
    } finally {
      creating = false;
    }
  }

  async function handleDelete(tenant: Tenant) {
    if (tenant.slug === 'default') {
      addToast('Cannot delete the default tenant', 'error');
      return;
    }
    if (!confirm(`Delete tenant "${tenant.name}" (${tenant.slug})? This cannot be undone.`)) return;
    try {
      await tenantsApi.delete(tenant.id);
      items = items.filter((t) => t.id !== tenant.id);
      availableTenants.set(items);
      addToast('Tenant deleted', 'success');
    } catch {
      addToast('Failed to delete tenant', 'error');
    }
  }
</script>

<div>
  <div class="flex items-center justify-between mb-6">
    <h1 class="text-2xl font-bold text-gray-900">Tenants</h1>
    <button onclick={openCreateModal} class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-indigo-600 rounded-md hover:bg-indigo-700 transition-colors">
      Create Tenant
    </button>
  </div>

  {#if isLoading}
    <LoadingSpinner />
  {:else if items.length === 0}
    <EmptyState message="No tenants registered." actionLabel="Create your first tenant" onaction={openCreateModal} />
  {:else}
    <div class="bg-white shadow-sm rounded-lg border border-gray-200 overflow-hidden">
      <table class="min-w-full divide-y divide-gray-200">
        <thead class="bg-gray-50">
          <tr>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Slug</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Name</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Created</th>
            <th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200">
          {#each items as tenant}
            <tr class="hover:bg-gray-50">
              <td class="px-6 py-4 text-sm font-mono font-medium text-gray-900">{tenant.slug}</td>
              <td class="px-6 py-4 text-sm text-gray-700">{tenant.name}</td>
              <td class="px-6 py-4 text-sm text-gray-500">{formatDate(tenant.created_at)}</td>
              <td class="px-6 py-4 text-right text-sm">
                {#if tenant.slug !== 'default'}
                  <button onclick={() => handleDelete(tenant)} class="text-red-600 hover:text-red-800">Delete</button>
                {:else}
                  <span class="text-gray-400 text-xs">default</span>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<!-- Create Tenant Modal -->
<Modal open={showCreateModal} title="Create Tenant" onclose={() => showCreateModal = false}>
  <form onsubmit={handleCreate} class="space-y-4">
    {#if createError}
      <div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded text-sm">{createError}</div>
    {/if}

    <div>
      <label for="newSlug" class="block text-sm font-medium text-gray-700 mb-1">Slug</label>
      <input
        id="newSlug"
        type="text"
        bind:value={newSlug}
        required
        placeholder="e.g. banking, expenses"
        class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
      />
      <p class="mt-1 text-xs text-gray-500">Lowercase letters, numbers, and hyphens only. Used as the X-Tenant-ID header value.</p>
    </div>

    <div>
      <label for="newName" class="block text-sm font-medium text-gray-700 mb-1">Name</label>
      <input
        id="newName"
        type="text"
        bind:value={newName}
        required
        placeholder="e.g. Banking App"
        class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
      />
    </div>

    <div class="flex items-center gap-3 pt-2">
      <button type="submit" disabled={creating} class="px-4 py-2 text-sm font-medium text-white bg-indigo-600 rounded-md hover:bg-indigo-700 disabled:opacity-50 transition-colors">
        {creating ? 'Creating...' : 'Create Tenant'}
      </button>
      <button type="button" onclick={() => showCreateModal = false} class="px-4 py-2 text-sm text-gray-700 hover:text-gray-900">
        Cancel
      </button>
    </div>
  </form>
</Modal>
