<script lang="ts">
  import { webhooks as webhooksApi } from '../lib/api';
  import { addToast, selectedTenant } from '../lib/stores';
  import { formatDate } from '../lib/utils';
  import LoadingSpinner from '../components/LoadingSpinner.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import type { Webhook } from '../lib/types';

  let items: Webhook[] = $state([]);
  let isLoading = $state(true);

  // Re-fetch when tenant selection changes
  let currentTenant = $state('');
  selectedTenant.subscribe((v) => { currentTenant = v; loadWebhooks(); });

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
    <h1 class="text-2xl font-bold text-on-surface">Webhooks</h1>
    <a href="#/webhooks/new" class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-gradient-to-br from-primary to-primary-container rounded-md hover:brightness-110 transition-all">
      Create Webhook
    </a>
  </div>

  {#if isLoading}
    <LoadingSpinner />
  {:else if items.length === 0}
    <EmptyState message="No webhooks configured yet." actionLabel="Create your first webhook" onaction={() => { window.location.hash = '#/webhooks/new'; }} />
  {:else}
    <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl overflow-hidden">
      <table class="min-w-full divide-y divide-outline-variant/15">
        <thead class="bg-surface-container-low">
          <tr>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">URL</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Tenant</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Events</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Type Filter</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Created</th>
            <th class="px-6 py-3 text-right text-xs font-medium text-on-surface-variant uppercase">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-outline-variant/15">
          {#each items as webhook}
            <tr class="hover:bg-surface-container-low">
              <td class="px-6 py-4 text-sm text-on-surface font-mono text-xs">{webhook.url}</td>
              <td class="px-6 py-4 text-sm text-on-surface-variant"><code class="bg-surface-container px-2 py-0.5 rounded text-xs">{webhook.tenant_id}</code></td>
              <td class="px-6 py-4 text-sm text-on-surface-variant">
                {#each webhook.events as event}
                  <span class="inline-flex mr-1 mb-1 px-2 py-0.5 bg-surface-container rounded text-xs">{event}</span>
                {/each}
              </td>
              <td class="px-6 py-4 text-sm text-on-surface-variant">{webhook.request_type || 'All'}</td>
              <td class="px-6 py-4 text-sm text-on-surface-variant">{formatDate(webhook.created_at)}</td>
              <td class="px-6 py-4 text-right text-sm">
                <button onclick={() => handleDelete(webhook.id)} class="text-status-rejected-text hover:text-red-800">Delete</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>
