<script lang="ts">
  import { webhooks as webhooksApi, deliveries as deliveriesApi, ApiError } from '../lib/api';
  import { addToast, selectedTenant } from '../lib/stores';
  import { formatDate } from '../lib/utils';
  import LoadingSpinner from '../components/LoadingSpinner.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import Modal from '../components/Modal.svelte';
  import type { Webhook, DeliveryStats } from '../lib/types';

  let items: Webhook[] = $state([]);
  let isLoading = $state(true);
  let totalWebhooks = $state(0);
  let deliveryRate = $state('—');

  const eventColorMap: Record<string, string> = {
    approved: 'bg-status-approved-bg text-status-approved-text',
    rejected: 'bg-status-rejected-bg text-status-rejected-text',
    pending: 'bg-status-pending-bg text-status-pending-text',
    cancelled: 'bg-status-cancelled-bg text-status-cancelled-text',
    expired: 'bg-status-expired-bg text-status-expired-text',
  };

  function eventClasses(event: string): string {
    return eventColorMap[event] ?? 'bg-surface-container text-on-surface-variant';
  }

  // Re-fetch when tenant selection changes
  let currentTenant = $state('');
  selectedTenant.subscribe((v) => { currentTenant = v; loadWebhooks(); });

  $effect(() => { loadWebhooks(); });

  async function loadWebhooks() {
    isLoading = true;
    try {
      const [webhooksRes, stats] = await Promise.all([
        webhooksApi.list(),
        deliveriesApi.stats().catch(() => null),
      ]);
      items = webhooksRes.data || [];
      totalWebhooks = webhooksRes.total ?? items.length;

      if (stats) {
        const total = stats.delivered + stats.failed;
        deliveryRate = total > 0
          ? `${((stats.delivered / total) * 100).toFixed(1)}%`
          : '—';
      } else {
        deliveryRate = '—';
      }
    } catch { addToast('Failed to load webhooks', 'error'); }
    finally { isLoading = false; }
  }

  async function handleDelete(id: string) {
    if (!confirm('Delete this webhook?')) return;
    try {
      await webhooksApi.delete(id);
      items = items.filter((w) => w.id !== id);
      totalWebhooks = Math.max(0, totalWebhooks - 1);
      addToast('Webhook deleted', 'success');
    } catch { addToast('Failed to delete webhook', 'error'); }
  }

  /* ── Create Webhook Modal ── */
  let showCreateModal = $state(false);
  let newUrl = $state('');
  let newSecret = $state('');
  let newRequestType = $state('');
  let newSelectedEvents: Record<string, boolean> = $state({ pending: true, approved: true, rejected: true, cancelled: false, expired: false });
  let showSecret = $state(false);
  let createSubmitting = $state(false);
  let createError = $state('');

  const availableEvents = ['pending', 'approved', 'rejected', 'cancelled', 'expired'];

  function openCreateModal() {
    newUrl = '';
    newSecret = '';
    newRequestType = '';
    newSelectedEvents = { pending: true, approved: true, rejected: true, cancelled: false, expired: false };
    showSecret = false;
    createError = '';
    showCreateModal = true;
  }

  function closeCreateModal() {
    showCreateModal = false;
  }

  async function handleCreate(e: SubmitEvent) {
    e.preventDefault();
    createError = '';

    const events = Object.entries(newSelectedEvents).filter(([, v]) => v).map(([k]) => k);
    if (events.length === 0) { createError = 'Select at least one event'; return; }

    createSubmitting = true;
    try {
      await webhooksApi.create({
        url: newUrl,
        secret: newSecret,
        events,
        request_type: newRequestType || undefined,
      });
      addToast('Webhook created', 'success');
      closeCreateModal();
      loadWebhooks();
    } catch (err) {
      createError = err instanceof ApiError ? err.message : 'Failed to create webhook';
    } finally { createSubmitting = false; }
  }
</script>

<div>
  <!-- Header -->
  <section class="flex flex-col md:flex-row md:items-end justify-between gap-6 mb-10">
    <div>
      <p class="text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mb-2">Integrations</p>
      <h1 class="text-4xl font-extrabold tracking-tight text-on-surface mb-2">Webhooks</h1>
      <p class="text-on-surface-variant max-w-lg">Manage external service integrations and real-time event notifications.</p>
    </div>
    <button onclick={openCreateModal} class="bg-gradient-to-br from-primary to-primary-container text-on-primary px-5 py-2.5 rounded-md font-semibold text-sm shadow-lg shadow-primary/20 hover:brightness-110 transition-all flex items-center gap-2 shrink-0">
      + Create Webhook
    </button>
  </section>

  <!-- Summary Cards -->
  <section class="grid grid-cols-1 sm:grid-cols-3 gap-6 mb-10">
    <div class="bg-surface-container-lowest p-6 rounded-xl shadow-ambient-lg">
      <p class="text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mb-1">Active Subscriptions</p>
      <div class="flex items-baseline gap-2">
        <p class="text-[44px] font-black tracking-tighter text-on-surface leading-none">{totalWebhooks}</p>
      </div>
      <p class="text-on-surface-variant font-medium mt-1">Webhooks</p>
    </div>
    <div class="bg-surface-container-lowest p-6 rounded-xl shadow-ambient-lg">
      <p class="text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mb-1">Delivery Rate</p>
      <div class="flex items-baseline gap-2">
        <p class="text-[44px] font-black tracking-tighter text-on-surface leading-none">{deliveryRate}</p>
      </div>
      <p class="text-on-surface-variant font-medium mt-1">Success</p>
    </div>
    <div class="bg-surface-container-lowest p-6 rounded-xl shadow-ambient-lg">
      <p class="text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mb-1">Avg. Latency</p>
      <div class="flex items-baseline gap-2">
        <p class="text-[44px] font-black tracking-tighter text-on-surface leading-none">—</p>
      </div>
      <p class="text-on-surface-variant font-medium mt-1">Response Time</p>
    </div>
  </section>

  {#if isLoading}
    <LoadingSpinner />
  {:else if items.length === 0}
    <EmptyState message="No webhooks configured yet." actionLabel="Create your first webhook" onaction={openCreateModal} />
  {:else}
    <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl overflow-hidden">
      <table class="min-w-full divide-y divide-outline-variant/15">
        <thead class="bg-surface-container-low">
          <tr>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant border-b border-outline-variant/15">URL</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant border-b border-outline-variant/15">Events</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant border-b border-outline-variant/15">Request Type</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant border-b border-outline-variant/15">Status</th>
            <th class="px-6 py-3 text-left text-[10px] font-bold uppercase tracking-widest text-on-surface-variant border-b border-outline-variant/15">Created</th>
            <th class="px-6 py-3 text-right text-[10px] font-bold uppercase tracking-widest text-on-surface-variant border-b border-outline-variant/15">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-outline-variant/15">
          {#each items as webhook}
            <tr class="hover:bg-surface-container-low transition-colors">
              <td class="px-6 py-5 text-xs text-on-surface font-mono">{webhook.url}</td>
              <td class="px-6 py-5">
                {#each webhook.events as event}
                  <span class="inline-flex mr-1 mb-1 px-2 py-0.5 rounded text-xs font-medium {eventClasses(event)}">{event}</span>
                {/each}
              </td>
              <td class="px-6 py-5 text-sm text-on-surface-variant">{webhook.request_type || 'All'}</td>
              <td class="px-6 py-5">
                <span class="inline-flex px-2 py-0.5 rounded text-xs font-bold uppercase tracking-wider bg-status-approved-bg text-status-approved-text">Active</span>
              </td>
              <td class="px-6 py-5 text-sm text-on-surface-variant">{formatDate(webhook.created_at)}</td>
              <td class="px-6 py-5 text-right text-sm">
                <button onclick={() => handleDelete(webhook.id)} class="text-status-rejected-text hover:text-red-800 transition-colors">Delete</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <p class="text-xs text-on-surface-variant mt-4">Showing 1–{items.length} of {totalWebhooks} webhooks</p>
  {/if}
</div>

<!-- Create Webhook Modal -->
<Modal open={showCreateModal} title="Create Webhook" onclose={closeCreateModal}>
  {#snippet children()}
    <form onsubmit={handleCreate} class="space-y-5">
      {#if createError}
        <div class="bg-status-rejected-bg border border-status-rejected-text/20 text-status-rejected-text px-4 py-3 rounded-lg text-sm">{createError}</div>
      {/if}

      <div>
        <label for="wh-url" class="block text-sm font-medium text-on-surface mb-1.5">URL</label>
        <input id="wh-url" type="url" bind:value={newUrl} required placeholder="https://your-app.com/webhooks" class="w-full px-3 py-2.5 text-sm border border-outline-variant/40 rounded-lg bg-surface-container-lowest focus:outline-none focus:ring-2 focus:ring-primary" />
      </div>

      <div>
        <label class="block text-sm font-medium text-on-surface mb-2">Events</label>
        <div class="grid grid-cols-3 gap-x-4 gap-y-2">
          {#each availableEvents as event}
            <label class="flex items-center gap-2 text-sm text-on-surface-variant">
              <input type="checkbox" bind:checked={newSelectedEvents[event]} class="rounded border-outline-variant/40 text-primary focus:ring-primary" />
              {event}
            </label>
          {/each}
        </div>
      </div>

      <div>
        <label for="wh-reqType" class="block text-sm font-medium text-on-surface mb-1.5">Request Type</label>
        <input id="wh-reqType" type="text" bind:value={newRequestType} placeholder="e.g. wire_transfer" class="w-full px-3 py-2.5 text-sm border border-outline-variant/40 rounded-lg bg-surface-container-lowest focus:outline-none focus:ring-2 focus:ring-primary" />
        <p class="mt-1 text-xs text-on-surface-variant/60">Leave empty to receive events for all request types</p>
      </div>

      <div>
        <label for="wh-secret" class="block text-sm font-medium text-on-surface mb-1.5">Signing Secret</label>
        <div class="relative">
          <input id="wh-secret" type={showSecret ? 'text' : 'password'} bind:value={newSecret} required class="w-full px-3 py-2.5 text-sm border border-outline-variant/40 rounded-lg bg-surface-container-lowest focus:outline-none focus:ring-2 focus:ring-primary pr-10" />
          <button type="button" onclick={() => showSecret = !showSecret} class="absolute right-3 top-1/2 -translate-y-1/2 text-on-surface-variant hover:text-on-surface transition-colors" aria-label={showSecret ? 'Hide secret' : 'Show secret'}>
            {#if showSecret}
              <svg class="w-4.5 h-4.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path d="M3.98 8.223A10.477 10.477 0 001.934 12C3.226 16.338 7.244 19.5 12 19.5c.993 0 1.953-.138 2.863-.395M6.228 6.228A10.45 10.45 0 0112 4.5c4.756 0 8.773 3.162 10.065 7.498a10.523 10.523 0 01-4.293 5.774M6.228 6.228L3 3m3.228 3.228l3.65 3.65m7.894 7.894L21 21m-3.228-3.228l-3.65-3.65m0 0a3 3 0 10-4.243-4.243m4.242 4.242L9.88 9.88" />
              </svg>
            {:else}
              <svg class="w-4.5 h-4.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path d="M2.036 12.322a1.012 1.012 0 010-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178z" />
                <path d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
              </svg>
            {/if}
          </button>
        </div>
        <p class="mt-1 text-xs text-on-surface-variant/60">Used to sign payloads with HMAC-SHA256</p>
      </div>

      <div class="flex items-center justify-end gap-3 pt-3 border-t border-outline-variant/15">
        <button type="button" onclick={closeCreateModal} class="px-4 py-2.5 text-sm font-medium text-on-surface-variant hover:text-on-surface transition-colors">Cancel</button>
        <button type="submit" disabled={createSubmitting} class="bg-gradient-to-br from-primary to-primary-container text-on-primary px-5 py-2.5 rounded-md font-semibold text-sm shadow-lg shadow-primary/20 hover:brightness-110 disabled:opacity-50 transition-all">
          {createSubmitting ? 'Creating...' : 'Create'}
        </button>
      </div>
    </form>
  {/snippet}
</Modal>
