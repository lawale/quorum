<script lang="ts">
  import { policies as policiesApi, webhooks as webhooksApi, requests as requestsApi, deliveries as deliveriesApi } from '../lib/api';
  import { selectedTenant } from '../lib/stores';
  import { formatDate } from '../lib/utils';
  import StatusBadge from '../components/StatusBadge.svelte';
  import LoadingSpinner from '../components/LoadingSpinner.svelte';
  import type { Policy, Webhook, Request } from '../lib/types';

  let isLoading = $state(true);
  let policyCount = $state(0);
  let webhookCount = $state(0);
  let pendingCount = $state(0);
  let totalRequests = $state(0);
  let failedDeliveries = $state(0);
  let recentRequests: Request[] = $state([]);
  let policies: Policy[] = $state([]);
  let webhooks: Webhook[] = $state([]);

  // Re-fetch when tenant selection changes
  let currentTenant = $state('');
  selectedTenant.subscribe((v) => { currentTenant = v; loadDashboard(); });

  $effect(() => {
    loadDashboard();
  });

  async function loadDashboard() {
    isLoading = true;
    try {
      const [policiesRes, webhooksRes, pendingRes, allRes, statsRes] = await Promise.all([
        policiesApi.list(),
        webhooksApi.list(),
        requestsApi.list({ page: 1, per_page: 5, status: 'pending' }),
        requestsApi.list({ page: 1, per_page: 5 }),
        deliveriesApi.stats().catch(() => ({ failed: 0 })),
      ]);

      policies = policiesRes.data || [];
      webhooks = webhooksRes.data || [];
      policyCount = policies.length;
      webhookCount = webhooks.length;
      pendingCount = pendingRes.total ?? 0;
      totalRequests = allRes.total ?? 0;
      recentRequests = allRes.data || [];
      failedDeliveries = (statsRes as Record<string, number>).failed ?? 0;
    } catch {
      // silently fail — cards show 0
    } finally {
      isLoading = false;
    }
  }

  function truncateId(id: string): string {
    return id.length > 8 ? id.slice(0, 8) + '…' : id;
  }
</script>

<div>
  <h1 class="text-2xl font-bold text-gray-900 mb-6">Dashboard</h1>

  {#if isLoading}
    <LoadingSpinner />
  {:else}
    <!-- Stats cards -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4 mb-8">
      <a href="#/requests" class="bg-white shadow-sm rounded-lg border border-gray-200 p-5 hover:shadow-md transition-shadow">
        <p class="text-sm font-medium text-gray-500">Total Requests</p>
        <p class="text-3xl font-bold text-gray-900 mt-1">{totalRequests}</p>
      </a>
      <a href="#/requests" class="bg-white shadow-sm rounded-lg border border-gray-200 p-5 hover:shadow-md transition-shadow">
        <p class="text-sm font-medium text-gray-500">Pending Approval</p>
        <p class="text-3xl font-bold text-yellow-600 mt-1">{pendingCount}</p>
      </a>
      <a href="#/deliveries" class="bg-white shadow-sm rounded-lg border {failedDeliveries > 0 ? 'border-red-300' : 'border-gray-200'} p-5 hover:shadow-md transition-shadow">
        <p class="text-sm font-medium text-gray-500">Failed Deliveries</p>
        <p class="text-3xl font-bold {failedDeliveries > 0 ? 'text-red-600' : 'text-gray-900'} mt-1">{failedDeliveries}</p>
      </a>
      <a href="#/policies" class="bg-white shadow-sm rounded-lg border border-gray-200 p-5 hover:shadow-md transition-shadow">
        <p class="text-sm font-medium text-gray-500">Policies</p>
        <p class="text-3xl font-bold text-gray-900 mt-1">{policyCount}</p>
      </a>
      <a href="#/webhooks" class="bg-white shadow-sm rounded-lg border border-gray-200 p-5 hover:shadow-md transition-shadow">
        <p class="text-sm font-medium text-gray-500">Webhooks</p>
        <p class="text-3xl font-bold text-gray-900 mt-1">{webhookCount}</p>
      </a>
    </div>

    <!-- Recent requests + Quick info -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <!-- Recent requests -->
      <div class="lg:col-span-2">
        <div class="bg-white shadow-sm rounded-lg border border-gray-200">
          <div class="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
            <h2 class="text-sm font-semibold text-gray-900">Recent Requests</h2>
            <a href="#/requests" class="text-xs text-indigo-600 hover:text-indigo-800">View all</a>
          </div>
          {#if recentRequests.length === 0}
            <div class="px-6 py-8 text-center">
              <p class="text-sm text-gray-400">No requests yet.</p>
            </div>
          {:else}
            <ul class="divide-y divide-gray-200">
              {#each recentRequests as req}
                <li>
                  <a href="#/requests/{req.id}" class="flex items-center justify-between px-6 py-3 hover:bg-gray-50 transition-colors">
                    <div class="flex items-center gap-3">
                      <StatusBadge status={req.status} />
                      <div>
                        <p class="text-sm font-medium text-gray-900">{req.type}</p>
                        <p class="text-xs text-gray-500 font-mono">{truncateId(req.id)}</p>
                      </div>
                    </div>
                    <div class="text-right">
                      <p class="text-xs text-gray-500">by {req.maker_id}</p>
                      <p class="text-xs text-gray-400">{formatDate(req.created_at)}</p>
                    </div>
                  </a>
                </li>
              {/each}
            </ul>
          {/if}
        </div>
      </div>

      <!-- Quick info sidebar -->
      <div class="space-y-6">
        <!-- Policies list -->
        <div class="bg-white shadow-sm rounded-lg border border-gray-200">
          <div class="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
            <h2 class="text-sm font-semibold text-gray-900">Policies</h2>
            <a href="#/policies" class="text-xs text-indigo-600 hover:text-indigo-800">Manage</a>
          </div>
          {#if policies.length === 0}
            <div class="px-6 py-4 text-center">
              <p class="text-sm text-gray-400">No policies configured.</p>
              <a href="#/policies/new" class="text-xs text-indigo-600 hover:text-indigo-800">Create one</a>
            </div>
          {:else}
            <ul class="divide-y divide-gray-200">
              {#each policies.slice(0, 5) as policy}
                <li>
                  <a href="#/policies/{policy.id}" class="flex items-center justify-between px-6 py-3 hover:bg-gray-50 transition-colors">
                    <div>
                      <p class="text-sm font-medium text-gray-900">{policy.name}</p>
                      <p class="text-xs text-gray-500">{policy.request_type}</p>
                    </div>
                    <span class="text-xs text-gray-400">{policy.stages.length} {policy.stages.length === 1 ? 'stage' : 'stages'}</span>
                  </a>
                </li>
              {/each}
            </ul>
          {/if}
        </div>

        <!-- Webhooks list -->
        <div class="bg-white shadow-sm rounded-lg border border-gray-200">
          <div class="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
            <h2 class="text-sm font-semibold text-gray-900">Webhooks</h2>
            <a href="#/webhooks" class="text-xs text-indigo-600 hover:text-indigo-800">Manage</a>
          </div>
          {#if webhooks.length === 0}
            <div class="px-6 py-4 text-center">
              <p class="text-sm text-gray-400">No webhooks configured.</p>
              <a href="#/webhooks/new" class="text-xs text-indigo-600 hover:text-indigo-800">Create one</a>
            </div>
          {:else}
            <ul class="divide-y divide-gray-200">
              {#each webhooks.slice(0, 5) as webhook}
                <li class="px-6 py-3">
                  <p class="text-sm font-mono text-xs text-gray-900 truncate">{webhook.url}</p>
                  <div class="flex gap-1 mt-1">
                    {#each webhook.events as event}
                      <span class="inline-flex px-1.5 py-0.5 bg-gray-100 rounded text-xs text-gray-600">{event}</span>
                    {/each}
                  </div>
                </li>
              {/each}
            </ul>
          {/if}
        </div>
      </div>
    </div>
  {/if}
</div>
