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
  <section class="flex flex-col md:flex-row md:items-end justify-between gap-6 mb-10">
    <div>
      <h1 class="text-4xl font-extrabold tracking-tight text-on-surface mb-2">Tenants</h1>
      <p class="text-on-surface-variant max-w-lg">Manage infrastructure isolation and organization units.</p>
    </div>
    <button onclick={openCreateModal} class="bg-gradient-to-br from-primary to-primary-container text-on-primary px-5 py-2.5 rounded-md font-semibold text-sm shadow-lg shadow-primary/20 hover:brightness-110 transition-all flex items-center gap-2">
      + Create Tenant
    </button>
  </section>

  {#if isLoading}
    <LoadingSpinner />
  {:else if items.length === 0}
    <EmptyState message="No tenants registered." actionLabel="Create your first tenant" onaction={openCreateModal} />
  {:else}
    <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl overflow-hidden">
      <table class="min-w-full">
        <thead>
          <tr class="border-b border-outline-variant/15">
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Slug</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Name</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Created</th>
            <th class="px-6 py-3 text-right text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-outline-variant/15">
          {#each items as tenant}
            <tr class="hover:bg-surface-container-low transition-colors">
              <td class="px-6 py-5">
                <a href="#/tenants/{tenant.slug}" class="text-primary-container hover:text-primary font-mono text-sm transition-colors">{tenant.slug}</a>
              </td>
              <td class="px-6 py-5 font-bold text-on-surface text-sm">{tenant.name}</td>
              <td class="px-6 py-5 text-sm text-on-surface-variant">{formatDate(tenant.created_at)}</td>
              <td class="px-6 py-5 text-right text-sm">
                {#if tenant.slug !== 'default'}
                  <button onclick={() => handleDelete(tenant)} class="text-on-surface-variant/40 hover:text-status-rejected-text transition-colors" title="Delete tenant">
                    <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
                      <path fill-rule="evenodd" d="M9 2a1 1 0 00-.894.553L7.382 4H4a1 1 0 000 2v10a2 2 0 002 2h8a2 2 0 002-2V6a1 1 0 100-2h-3.382l-.724-1.447A1 1 0 0011 2H9zM7 8a1 1 0 012 0v6a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v6a1 1 0 102 0V8a1 1 0 00-1-1z" clip-rule="evenodd" />
                    </svg>
                  </button>
                {:else}
                  <span class="inline-flex px-2 py-0.5 bg-surface-container-low text-on-surface-variant rounded text-[10px] font-bold uppercase tracking-wider">Default</span>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <p class="text-xs text-on-surface-variant mt-4">Showing {items.length} of {items.length} tenants</p>
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
