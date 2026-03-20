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

  let newSlug = $state('');
  let newName = $state('');
  let creating = $state(false);
  let createError = $state('');
  let validationErrors: Record<string, string> = $state({});

  $effect(() => {
    loadTenants();
  });

  async function loadTenants() {
    isLoading = true;
    try {
      const res = await tenantsApi.list();
      items = res.data || [];
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
    validationErrors = {};
    showCreateModal = true;
  }

  function validate(): boolean {
    validationErrors = {};

    const slug = newSlug.trim();
    if (slug.length < 2 || slug.length > 50) {
      validationErrors.slug = 'Slug must be between 2 and 50 characters';
    } else if (!/^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/.test(slug)) {
      validationErrors.slug = 'Slug must contain only lowercase letters, numbers, and hyphens, and start/end with an alphanumeric character';
    }

    const name = newName.trim();
    if (name.length < 1 || name.length > 100) {
      validationErrors.name = 'Name must be between 1 and 100 characters';
    }

    return Object.keys(validationErrors).length === 0;
  }

  function clearFieldError(field: string) {
    validationErrors = { ...validationErrors };
    delete validationErrors[field];
  }

  async function handleCreate(e: SubmitEvent) {
    e.preventDefault();
    createError = '';

    if (!validate()) return;

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
    <h1 class="text-2xl font-bold text-on-surface">Tenants</h1>
    <button onclick={openCreateModal} class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-gradient-to-br from-primary to-primary-container rounded-md hover:brightness-110 transition-all">
      Create Tenant
    </button>
  </div>

  {#if isLoading}
    <LoadingSpinner />
  {:else if items.length === 0}
    <EmptyState message="No tenants registered." actionLabel="Create your first tenant" onaction={openCreateModal} />
  {:else}
    <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl overflow-hidden">
      <table class="min-w-full divide-y divide-outline-variant/15">
        <thead class="bg-surface-container-low">
          <tr>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Slug</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Name</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Created</th>
            <th class="px-6 py-3 text-right text-xs font-medium text-on-surface-variant uppercase">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-outline-variant/15">
          {#each items as tenant}
            <tr class="hover:bg-surface-container-low">
              <td class="px-6 py-4 text-sm font-mono font-medium text-on-surface">{tenant.slug}</td>
              <td class="px-6 py-4 text-sm text-on-surface">{tenant.name}</td>
              <td class="px-6 py-4 text-sm text-on-surface-variant">{formatDate(tenant.created_at)}</td>
              <td class="px-6 py-4 text-right text-sm">
                {#if tenant.slug !== 'default'}
                  <button onclick={() => handleDelete(tenant)} class="text-status-rejected-text hover:text-red-800">Delete</button>
                {:else}
                  <span class="text-on-surface-variant/60 text-xs">default</span>
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
      <div class="bg-status-rejected-bg border border-status-rejected-text/20 text-status-rejected-text px-4 py-3 rounded text-sm">{createError}</div>
    {/if}

    <div>
      <label for="newSlug" class="block text-sm font-medium text-on-surface mb-1">Slug</label>
      <input
        id="newSlug"
        type="text"
        bind:value={newSlug}
        oninput={() => clearFieldError('slug')}
        required
        placeholder="e.g. banking, expenses"
        class="w-full px-3 py-2 border rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary {validationErrors.slug ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}"
      />
      {#if validationErrors.slug}
        <p class="mt-1 text-xs text-status-rejected-text">{validationErrors.slug}</p>
      {:else}
        <p class="mt-1 text-xs text-on-surface-variant">Lowercase letters, numbers, and hyphens only. Used as the X-Tenant-ID header value.</p>
      {/if}
    </div>

    <div>
      <label for="newName" class="block text-sm font-medium text-on-surface mb-1">Name</label>
      <input
        id="newName"
        type="text"
        bind:value={newName}
        oninput={() => clearFieldError('name')}
        required
        placeholder="e.g. Banking App"
        class="w-full px-3 py-2 border rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary {validationErrors.name ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}"
      />
      {#if validationErrors.name}
        <p class="mt-1 text-xs text-status-rejected-text">{validationErrors.name}</p>
      {/if}
    </div>

    <div class="flex items-center gap-3 pt-2">
      <button type="submit" disabled={creating} class="px-4 py-2 text-sm font-medium text-white bg-gradient-to-br from-primary to-primary-container rounded-md hover:brightness-110 disabled:opacity-50 transition-all">
        {creating ? 'Creating...' : 'Create Tenant'}
      </button>
      <button type="button" onclick={() => showCreateModal = false} class="px-4 py-2 text-sm text-on-surface-variant hover:text-on-surface">
        Cancel
      </button>
    </div>
  </form>
</Modal>
