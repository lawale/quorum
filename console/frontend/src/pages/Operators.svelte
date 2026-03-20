<script lang="ts">
  import { operators as operatorsApi } from '../lib/api';
  import { addToast } from '../lib/stores';
  import { formatDate } from '../lib/utils';
  import LoadingSpinner from '../components/LoadingSpinner.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import Modal from '../components/Modal.svelte';
  import type { Operator } from '../lib/types';

  let items: Operator[] = $state([]);
  let isLoading = $state(true);
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
      const res = await operatorsApi.list();
      items = res.data || [];
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
  <div class="flex items-center justify-between mb-6">
    <h1 class="text-2xl font-bold text-on-surface">Operators</h1>
    <button onclick={openCreateModal} class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-gradient-to-br from-primary to-primary-container rounded-md hover:brightness-110 transition-all">
      Create Operator
    </button>
  </div>

  {#if isLoading}
    <LoadingSpinner />
  {:else if items.length === 0}
    <EmptyState message="No operators found." actionLabel="Create your first operator" onaction={openCreateModal} />
  {:else}
    <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl overflow-hidden">
      <table class="min-w-full divide-y divide-outline-variant/15">
        <thead class="bg-surface-container-low">
          <tr>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Username</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Display Name</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Created</th>
            <th class="px-6 py-3 text-right text-xs font-medium text-on-surface-variant uppercase">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-outline-variant/15">
          {#each items as op}
            <tr class="hover:bg-surface-container-low">
              <td class="px-6 py-4 text-sm font-medium text-on-surface">{op.username}</td>
              <td class="px-6 py-4 text-sm text-on-surface-variant">{op.display_name || '—'}</td>
              <td class="px-6 py-4 text-sm text-on-surface-variant">{formatDate(op.created_at)}</td>
              <td class="px-6 py-4 text-right text-sm">
                <button onclick={() => handleDelete(op)} class="text-status-rejected-text hover:text-red-800">Delete</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
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
