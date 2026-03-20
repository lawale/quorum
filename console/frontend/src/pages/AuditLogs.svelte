<script lang="ts">
  import { requests as requestsApi } from '../lib/api';
  import { addToast } from '../lib/stores';
  import { formatDate, formatDetails } from '../lib/utils';
  import LoadingSpinner from '../components/LoadingSpinner.svelte';
  import type { AuditLog } from '../lib/types';

  let requestId = $state('');
  let auditLogs: AuditLog[] = $state([]);
  let isLoading = $state(false);
  let searched = $state(false);
  let error = $state('');

  async function handleSearch(e: SubmitEvent) {
    e.preventDefault();
    error = '';

    const trimmed = requestId.trim();
    if (!trimmed) {
      error = 'Enter a request ID';
      return;
    }

    isLoading = true;
    searched = true;
    try {
      const res = await requestsApi.audit(trimmed);
      auditLogs = res.data || [];
    } catch {
      addToast('Failed to load audit logs', 'error');
      auditLogs = [];
    } finally {
      isLoading = false;
    }
  }

  function actionColor(action: string): string {
    switch (action) {
      case 'created':
      case 'webhook_sent':
      case 'webhook_failed': return 'bg-surface-container text-on-surface-variant';
      case 'approved':
      case 'stage_advanced': return 'bg-status-approved-bg text-status-approved-text';
      case 'rejected': return 'bg-status-rejected-bg text-status-rejected-text';
      case 'cancelled':
      case 'expired': return 'bg-surface-container text-on-surface-variant';
      default: return 'bg-surface-container text-on-surface-variant';
    }
  }

</script>

<div>
  <h1 class="text-2xl font-bold text-on-surface mb-6">Audit Log</h1>

  <!-- Search -->
  <form onsubmit={handleSearch} class="bg-surface-container-lowest shadow-ambient-sm rounded-xl p-4 mb-6">
    <div class="flex items-end gap-4">
      <div class="flex-1">
        <label for="requestId" class="block text-xs font-medium text-on-surface-variant mb-1">Request ID</label>
        <input
          id="requestId"
          type="text"
          bind:value={requestId}
          placeholder="Enter request UUID…"
          class="w-full px-3 py-1.5 text-sm border border-outline-variant/40 rounded-md focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary"
        />
      </div>
      <button type="submit" class="px-4 py-1.5 text-sm font-medium text-white bg-gradient-to-br from-primary to-primary-container rounded-md hover:brightness-110 transition-all">
        Search
      </button>
    </div>
    {#if error}
      <p class="text-sm text-status-rejected-text mt-2">{error}</p>
    {/if}
  </form>

  {#if isLoading}
    <LoadingSpinner />
  {:else if searched}
    {#if auditLogs.length === 0}
      <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl p-6 text-center">
        <p class="text-sm text-on-surface-variant">No audit entries found for this request.</p>
      </div>
    {:else}
      <div class="space-y-1 mb-4">
        <p class="text-sm text-on-surface-variant">{auditLogs.length} audit {auditLogs.length === 1 ? 'entry' : 'entries'} for request
          <a href="#/requests/{requestId.trim()}" class="text-primary-container hover:text-primary font-mono text-xs">{requestId.trim()}</a>
        </p>
      </div>

      <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl overflow-hidden">
        <table class="min-w-full divide-y divide-outline-variant/15">
          <thead class="bg-surface-container-low">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Action</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Actor</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Details</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-on-surface-variant uppercase">Timestamp</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-outline-variant/15">
            {#each auditLogs as log}
              <tr class="hover:bg-surface-container-low">
                <td class="px-6 py-4 text-sm">
                  <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium {actionColor(log.action)}">
                    {log.action}
                  </span>
                </td>
                <td class="px-6 py-4 text-sm text-on-surface">{log.actor_id}</td>
                <td class="px-6 py-4 text-sm text-on-surface-variant">
                  {#if log.details && Object.keys(log.details).length > 0}
                    <span class="text-xs text-on-surface whitespace-pre-line">{formatDetails(log.details, log.action)}</span>
                  {:else}
                    <span class="text-on-surface-variant/60">—</span>
                  {/if}
                </td>
                <td class="px-6 py-4 text-sm text-on-surface-variant">{formatDate(log.created_at)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  {:else}
    <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl p-12 text-center">
      <p class="text-on-surface-variant/60 text-sm">Enter a request ID above to view its audit trail.</p>
    </div>
  {/if}
</div>
