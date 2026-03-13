<script lang="ts">
  import { webhooks as webhooksApi } from '../lib/api';
  import { addToast } from '../lib/stores';
  import { formatDate } from '../lib/utils';
  import LoadingSpinner from '../components/LoadingSpinner.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import type { Webhook } from '../lib/types';

  let items: Webhook[] = $state([]);
  let isLoading = $state(true);

  $effect(() => { loadWebhooks(); });

  async function loadWebhooks() {
    isLoading = true;
    try {
      const res = await webhooksApi.list();
      items = res.data || [];
    } catch { addToast('Failed to load webhooks', 'error'); }
    finally { isLoading = false; }
  }

  async function handleDelete(id: string) {
    if (!confirm('Delete this webhook?')) return;
    try {
      await webhooksApi.delete(id);
      items = items.filter((w) => w.id !== id);
      addToast('Webhook deleted', 'success');
    } catch { addToast('Failed to delete webhook', 'error'); }
  }
</script>

<div>
  <div class="flex items-center justify-between mb-6">
    <h1 class="text-2xl font-bold text-gray-900">Webhooks</h1>
    <a href="#/webhooks/new" class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-indigo-600 rounded-md hover:bg-indigo-700 transition-colors">
      Create Webhook
    </a>
  </div>

  {#if isLoading}
    <LoadingSpinner />
  {:else if items.length === 0}
    <EmptyState message="No webhooks configured yet." actionLabel="Create your first webhook" onaction={() => { window.location.hash = '#/webhooks/new'; }} />
  {:else}
    <div class="bg-white shadow-sm rounded-lg border border-gray-200 overflow-hidden">
      <table class="min-w-full divide-y divide-gray-200">
        <thead class="bg-gray-50">
          <tr>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">URL</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Events</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Type Filter</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Created</th>
            <th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200">
          {#each items as webhook}
            <tr class="hover:bg-gray-50">
              <td class="px-6 py-4 text-sm text-gray-900 font-mono text-xs">{webhook.url}</td>
              <td class="px-6 py-4 text-sm text-gray-500">
                {#each webhook.events as event}
                  <span class="inline-flex mr-1 mb-1 px-2 py-0.5 bg-gray-100 rounded text-xs">{event}</span>
                {/each}
              </td>
              <td class="px-6 py-4 text-sm text-gray-500">{webhook.request_type || 'All'}</td>
              <td class="px-6 py-4 text-sm text-gray-500">{formatDate(webhook.created_at)}</td>
              <td class="px-6 py-4 text-right text-sm">
                <button onclick={() => handleDelete(webhook.id)} class="text-red-600 hover:text-red-800">Delete</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>
