<script lang="ts">
  import { operators as operatorsApi } from '../lib/api';
  import { addToast, currentUser } from '../lib/stores';
  import { formatDate } from '../lib/utils';
  import LoadingSpinner from '../components/LoadingSpinner.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import Modal from '../components/Modal.svelte';
  import type { Operator } from '../lib/types';

  let items: Operator[] = $state([]);
  let isLoading = $state(true);
  let total = $state(0);
  let page = $state(1);
  const perPage = 20;
  let totalPages = $derived(Math.max(1, Math.ceil(total / perPage)));
  let showCreateModal = $state(false);

  let newUsername = $state('');
  let newPassword = $state('');
  let newDisplayName = $state('');
  let creating = $state(false);
  let createError = $state('');
  let validationErrors: Record<string, string> = $state({});

  $effect(() => {
    loadOperators();
  });

  async function loadOperators() {
    isLoading = true;
    try {
      const res = await operatorsApi.list({ page, per_page: perPage });
      items = res.data || [];
      total = res.total ?? items.length;
    } catch {
      addToast('Failed to load operators', 'error');
    } finally {
      isLoading = false;
    }
  }

  function openCreateModal() {
    newUsername = '';
    newPassword = '';
    newDisplayName = '';
    createError = '';
    validationErrors = {};
    showCreateModal = true;
  }

  function validate(): boolean {
    validationErrors = {};

    if (newUsername.trim().length < 3) {
      validationErrors.username = 'Username must be at least 3 characters';
    } else if (!/^[a-zA-Z0-9_]+$/.test(newUsername.trim())) {
      validationErrors.username = 'Username can only contain letters, numbers, and underscores';
    }

    if (newPassword.length < 8) {
      validationErrors.password = 'Password must be at least 8 characters';
    } else if (!/[A-Z]/.test(newPassword)) {
      validationErrors.password = 'Password must contain at least one uppercase letter';
    } else if (!/[0-9]/.test(newPassword)) {
      validationErrors.password = 'Password must contain at least one digit';
    }

    if (newDisplayName.trim().length > 0 && newDisplayName.trim().length < 1) {
      validationErrors.displayName = 'Display name must not be empty';
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
      await operatorsApi.create(newUsername.trim(), newPassword, newDisplayName.trim());
      addToast('Operator created', 'success');
      showCreateModal = false;
      await loadOperators();
    } catch (err) {
      createError = err instanceof Error ? err.message : 'Failed to create operator';
    } finally {
      creating = false;
    }
  }

  async function handleDelete(op: Operator) {
    if (!confirm(`Delete operator "${op.username}"? This cannot be undone.`)) return;
    try {
      await operatorsApi.delete(op.id);
      items = items.filter((o) => o.id !== op.id);
      addToast('Operator deleted', 'success');
    } catch {
      addToast('Failed to delete operator', 'error');
    }
  }
</script>

<div>
  <section class="flex flex-col md:flex-row md:items-end justify-between gap-6 mb-10">
    <div>
      <h1 class="text-4xl font-extrabold tracking-tight text-on-surface mb-2">Operators</h1>
      <p class="text-on-surface-variant max-w-lg">Manage console users and their access credentials.</p>
    </div>
    <button onclick={openCreateModal} class="bg-gradient-to-br from-primary to-primary-container text-on-primary px-5 py-2.5 rounded-md font-semibold text-sm shadow-lg shadow-primary/20 hover:brightness-110 transition-all flex items-center gap-2">
      + Create Operator
    </button>
  </section>

  {#if isLoading}
    <LoadingSpinner />
  {:else if items.length === 0}
    <EmptyState message="No operators found." actionLabel="Create your first operator" onaction={openCreateModal} />
  {:else}
    <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl overflow-hidden">
      <table class="min-w-full">
        <thead>
          <tr class="border-b border-outline-variant/15">
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Username</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Display Name</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Created</th>
            <th class="px-6 py-3 text-right text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-outline-variant/15">
          {#each items as op}
            <tr class="hover:bg-surface-container-low transition-colors">
              <td class="px-6 py-5 text-sm">
                <span class="font-bold text-on-surface">{op.username}</span>
                {#if $currentUser?.username === op.username}
                  <span class="inline-flex px-2 py-0.5 bg-primary-container/10 text-primary-container rounded text-[10px] font-bold uppercase tracking-wider ml-2">You</span>
                {/if}
              </td>
              <td class="px-6 py-5 text-sm text-on-surface-variant">{op.display_name || '—'}</td>
              <td class="px-6 py-5 text-sm text-on-surface-variant">{formatDate(op.created_at)}</td>
              <td class="px-6 py-5 text-right text-sm">
                <button onclick={() => handleDelete(op)} class="text-on-surface-variant/40 hover:text-status-rejected-text transition-colors" title="Delete operator">
                  <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
                    <path fill-rule="evenodd" d="M9 2a1 1 0 00-.894.553L7.382 4H4a1 1 0 000 2v10a2 2 0 002 2h8a2 2 0 002-2V6a1 1 0 100-2h-3.382l-.724-1.447A1 1 0 0011 2H9zM7 8a1 1 0 012 0v6a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v6a1 1 0 102 0V8a1 1 0 00-1-1z" clip-rule="evenodd" />
                  </svg>
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <div class="flex items-center justify-between mt-6">
      <span class="text-sm text-on-surface-variant">Showing {(page - 1) * perPage + 1}–{Math.min(page * perPage, total)} of {total} operators</span>
      {#if totalPages > 1}
        <div class="flex items-center gap-3">
          <span class="text-sm text-on-surface-variant">Page {page} of {totalPages}</span>
          <div class="flex gap-1">
            <button onclick={() => { page = Math.max(1, page - 1); loadOperators(); }} disabled={page <= 1} class="px-2.5 py-1.5 text-sm border border-outline-variant/40 rounded-md hover:bg-surface-container-low disabled:opacity-50 disabled:cursor-not-allowed transition-colors" aria-label="Previous page">
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" /></svg>
            </button>
            <button onclick={() => { page = Math.min(totalPages, page + 1); loadOperators(); }} disabled={page >= totalPages} class="px-2.5 py-1.5 text-sm border border-outline-variant/40 rounded-md hover:bg-surface-container-low disabled:opacity-50 disabled:cursor-not-allowed transition-colors" aria-label="Next page">
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" /></svg>
            </button>
          </div>
        </div>
      {/if}
    </div>
  {/if}
</div>

<!-- Create Operator Modal -->
<Modal open={showCreateModal} title="Create Operator" onclose={() => showCreateModal = false}>
  <form onsubmit={handleCreate} class="space-y-4">
    {#if createError}
      <div class="bg-status-rejected-bg border border-status-rejected-text/20 text-status-rejected-text px-4 py-3 rounded text-sm">{createError}</div>
    {/if}

    <div>
      <label for="newUsername" class="block text-sm font-medium text-on-surface mb-1">Username</label>
      <input
        id="newUsername"
        type="text"
        bind:value={newUsername}
        oninput={() => clearFieldError('username')}
        required
        class="w-full px-3 py-2 border rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary {validationErrors.username ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}"
      />
      {#if validationErrors.username}
        <p class="mt-1 text-xs text-status-rejected-text">{validationErrors.username}</p>
      {/if}
    </div>

    <div>
      <label for="newDisplayName" class="block text-sm font-medium text-on-surface mb-1">Display Name</label>
      <input
        id="newDisplayName"
        type="text"
        bind:value={newDisplayName}
        oninput={() => clearFieldError('displayName')}
        class="w-full px-3 py-2 border rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary {validationErrors.displayName ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}"
      />
      {#if validationErrors.displayName}
        <p class="mt-1 text-xs text-status-rejected-text">{validationErrors.displayName}</p>
      {/if}
    </div>

    <div>
      <label for="newPassword" class="block text-sm font-medium text-on-surface mb-1">Password</label>
      <input
        id="newPassword"
        type="password"
        bind:value={newPassword}
        oninput={() => clearFieldError('password')}
        required
        class="w-full px-3 py-2 border rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary {validationErrors.password ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}"
      />
      {#if validationErrors.password}
        <p class="mt-1 text-xs text-status-rejected-text">{validationErrors.password}</p>
      {/if}
    </div>

    <div class="flex items-center gap-3 pt-2">
      <button type="submit" disabled={creating} class="px-4 py-2 text-sm font-medium text-white bg-gradient-to-br from-primary to-primary-container rounded-md hover:brightness-110 disabled:opacity-50 transition-all">
        {creating ? 'Creating…' : 'Create Operator'}
      </button>
      <button type="button" onclick={() => showCreateModal = false} class="px-4 py-2 text-sm text-on-surface-variant hover:text-on-surface">
        Cancel
      </button>
    </div>
  </form>
</Modal>
