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

  // Create form state
  let newUsername = $state('');
  let newPassword = $state('');
  let newDisplayName = $state('');
  let creating = $state(false);
  let createError = $state('');

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
    showCreateModal = true;
  }

  async function handleCreate(e: SubmitEvent) {
    e.preventDefault();
    createError = '';

    if (!newUsername.trim() || !newPassword.trim()) {
      createError = 'Username and password are required';
      return;
    }

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
    <h1 class="text-2xl font-bold text-gray-900">Operators</h1>
    <button onclick={openCreateModal} class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-indigo-600 rounded-md hover:bg-indigo-700 transition-colors">
      Create Operator
    </button>
  </div>

  {#if isLoading}
    <LoadingSpinner />
  {:else if items.length === 0}
    <EmptyState message="No operators found." actionLabel="Create your first operator" onaction={openCreateModal} />
  {:else}
    <div class="bg-white shadow-sm rounded-lg border border-gray-200 overflow-hidden">
      <table class="min-w-full divide-y divide-gray-200">
        <thead class="bg-gray-50">
          <tr>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Username</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Display Name</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Created</th>
            <th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200">
          {#each items as op}
            <tr class="hover:bg-gray-50">
              <td class="px-6 py-4 text-sm font-medium text-gray-900">{op.username}</td>
              <td class="px-6 py-4 text-sm text-gray-500">{op.display_name || '—'}</td>
              <td class="px-6 py-4 text-sm text-gray-500">{formatDate(op.created_at)}</td>
              <td class="px-6 py-4 text-right text-sm">
                <button onclick={() => handleDelete(op)} class="text-red-600 hover:text-red-800">Delete</button>
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
      <div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded text-sm">{createError}</div>
    {/if}

    <div>
      <label for="newUsername" class="block text-sm font-medium text-gray-700 mb-1">Username</label>
      <input
        id="newUsername"
        type="text"
        bind:value={newUsername}
        required
        class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
      />
    </div>

    <div>
      <label for="newDisplayName" class="block text-sm font-medium text-gray-700 mb-1">Display Name</label>
      <input
        id="newDisplayName"
        type="text"
        bind:value={newDisplayName}
        class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
      />
    </div>

    <div>
      <label for="newPassword" class="block text-sm font-medium text-gray-700 mb-1">Password</label>
      <input
        id="newPassword"
        type="password"
        bind:value={newPassword}
        required
        class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
      />
    </div>

    <div class="flex items-center gap-3 pt-2">
      <button type="submit" disabled={creating} class="px-4 py-2 text-sm font-medium text-white bg-indigo-600 rounded-md hover:bg-indigo-700 disabled:opacity-50 transition-colors">
        {creating ? 'Creating…' : 'Create Operator'}
      </button>
      <button type="button" onclick={() => showCreateModal = false} class="px-4 py-2 text-sm text-gray-700 hover:text-gray-900">
        Cancel
      </button>
    </div>
  </form>
</Modal>
