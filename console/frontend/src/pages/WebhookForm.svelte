<script lang="ts">
  import { webhooks as webhooksApi, ApiError } from '../lib/api';
  import { addToast } from '../lib/stores';

  let url = $state('');
  let secret = $state('');
  let requestType = $state('');
  let selectedEvents: Record<string, boolean> = $state({ approved: true, rejected: true });
  let submitting = $state(false);
  let error = $state('');

  const availableEvents = ['approved', 'rejected', 'cancelled', 'expired'];

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    error = '';

    const events = Object.entries(selectedEvents).filter(([, v]) => v).map(([k]) => k);
    if (events.length === 0) { error = 'Select at least one event'; return; }

    submitting = true;
    try {
      await webhooksApi.create({
        url,
        secret,
        events,
        request_type: requestType || undefined,
      });
      addToast('Webhook created', 'success');
      window.location.hash = '#/webhooks';
    } catch (err) {
      error = err instanceof ApiError ? err.message : 'Failed to create webhook';
    } finally { submitting = false; }
  }
</script>

<div>
  <div class="mb-6">
    <a href="#/webhooks" class="text-sm text-on-surface-variant hover:text-on-surface">&larr; Back to webhooks</a>
    <h1 class="text-2xl font-bold text-on-surface mt-2">Create Webhook</h1>
  </div>

  <form onsubmit={handleSubmit} class="bg-surface-container-lowest shadow-ambient-sm rounded-xl p-6 space-y-6 max-w-2xl">
    {#if error}
      <div class="bg-status-rejected-bg border-status-rejected-text/20 text-status-rejected-text px-4 py-3 rounded text-sm">{error}</div>
    {/if}

    <div>
      <label for="url" class="block text-sm font-medium text-on-surface mb-1">URL</label>
      <input id="url" type="url" bind:value={url} required placeholder="https://example.com/webhook" class="w-full px-3 py-2 border border-outline-variant/40 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary" />
    </div>

    <div>
      <label for="secret" class="block text-sm font-medium text-on-surface mb-1">Secret</label>
      <input id="secret" type="text" bind:value={secret} required class="w-full px-3 py-2 border border-outline-variant/40 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary" />
    </div>

    <div>
      <label class="block text-sm font-medium text-on-surface mb-2">Events</label>
      <div class="flex gap-4">
        {#each availableEvents as event}
          <label class="flex items-center gap-2 text-sm">
            <input type="checkbox" bind:checked={selectedEvents[event]} class="rounded border-outline-variant/40 text-primary-container focus:ring-primary" />
            {event}
          </label>
        {/each}
      </div>
    </div>

    <div>
      <label for="reqType" class="block text-sm font-medium text-on-surface mb-1">Request Type Filter <span class="text-on-surface-variant/60">(optional, blank = all types)</span></label>
      <input id="reqType" type="text" bind:value={requestType} placeholder="transfer" class="w-full px-3 py-2 border border-outline-variant/40 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary" />
    </div>

    <div class="flex items-center gap-3 pt-4 border-t border-outline-variant/15">
      <button type="submit" disabled={submitting} class="px-4 py-2 text-sm font-medium text-white bg-gradient-to-br from-primary to-primary-container rounded-md hover:brightness-110 disabled:opacity-50 transition-all">
        {submitting ? 'Creating...' : 'Create Webhook'}
      </button>
      <a href="#/webhooks" class="px-4 py-2 text-sm text-on-surface-variant hover:text-on-surface">Cancel</a>
    </div>
  </form>
</div>
