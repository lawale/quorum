<script lang="ts">
  import { requests as requestsApi } from '../lib/api';
  import { addToast } from '../lib/stores';
  import { formatDate } from '../lib/utils';
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
      case 'created': return 'bg-blue-100 text-blue-800';
      case 'approved': return 'bg-green-100 text-green-800';
      case 'rejected': return 'bg-red-100 text-red-800';
      case 'cancelled': return 'bg-gray-100 text-gray-800';
      case 'expired': return 'bg-orange-100 text-orange-800';
      default: return 'bg-gray-100 text-gray-600';
    }
  }

  function formatJson(obj: unknown): string {
    return JSON.stringify(obj, null, 2);
  }
</script>

<div>
  <h1 class="text-2xl font-bold text-gray-900 mb-6">Audit Log</h1>

  <!-- Search -->
  <form onsubmit={handleSearch} class="bg-white shadow-sm rounded-lg border border-gray-200 p-4 mb-6">
    <div class="flex items-end gap-4">
      <div class="flex-1">
        <label for="requestId" class="block text-xs font-medium text-gray-500 mb-1">Request ID</label>
        <input
          id="requestId"
          type="text"
          bind:value={requestId}
          placeholder="Enter request UUID…"
          class="w-full px-3 py-1.5 text-sm border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
        />
      </div>
      <button type="submit" class="px-4 py-1.5 text-sm font-medium text-white bg-indigo-600 rounded-md hover:bg-indigo-700 transition-colors">
        Search
      </button>
    </div>
    {#if error}
      <p class="text-sm text-red-600 mt-2">{error}</p>
    {/if}
  </form>

  {#if isLoading}
    <LoadingSpinner />
  {:else if searched}
    {#if auditLogs.length === 0}
      <div class="bg-white shadow-sm rounded-lg border border-gray-200 p-6 text-center">
        <p class="text-sm text-gray-500">No audit entries found for this request.</p>
      </div>
    {:else}
      <div class="space-y-1 mb-4">
        <p class="text-sm text-gray-500">{auditLogs.length} audit {auditLogs.length === 1 ? 'entry' : 'entries'} for request
          <a href="#/requests/{requestId.trim()}" class="text-indigo-600 hover:text-indigo-800 font-mono text-xs">{requestId.trim()}</a>
        </p>
      </div>

      <div class="bg-white shadow-sm rounded-lg border border-gray-200 overflow-hidden">
        <table class="min-w-full divide-y divide-gray-200">
          <thead class="bg-gray-50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Action</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Actor</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Details</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Timestamp</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200">
            {#each auditLogs as log}
              <tr class="hover:bg-gray-50">
                <td class="px-6 py-4 text-sm">
                  <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium {actionColor(log.action)}">
                    {log.action}
                  </span>
                </td>
                <td class="px-6 py-4 text-sm text-gray-700">{log.actor_id}</td>
                <td class="px-6 py-4 text-sm text-gray-500">
                  {#if log.details && Object.keys(log.details).length > 0}
                    <pre class="bg-gray-50 rounded p-2 text-xs font-mono overflow-x-auto max-w-md">{formatJson(log.details)}</pre>
                  {:else}
                    <span class="text-gray-400">—</span>
                  {/if}
                </td>
                <td class="px-6 py-4 text-sm text-gray-500">{formatDate(log.created_at)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  {:else}
    <div class="bg-white shadow-sm rounded-lg border border-gray-200 p-12 text-center">
      <p class="text-gray-400 text-sm">Enter a request ID above to view its audit trail.</p>
    </div>
  {/if}
</div>
