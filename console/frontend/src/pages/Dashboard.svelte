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
  <h1 class="text-2xl font-bold text-on-surface mb-6">Dashboard</h1>

  {#if isLoading}
    <LoadingSpinner />
  {:else}
    <!-- Stats cards -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4 mb-8">
      <a href="#/requests" class="bg-surface-container-lowest shadow-ambient-sm rounded-xl p-5 hover:shadow-ambient-md transition-shadow">
        <p class="text-sm font-medium text-on-surface-variant">Total Requests</p>
        <p class="text-3xl font-bold text-on-surface tracking-tight mt-1">{totalRequests}</p>
      </a>
      <a href="#/requests" class="bg-surface-container-lowest shadow-ambient-sm rounded-xl p-5 hover:shadow-ambient-md transition-shadow">
        <p class="text-sm font-medium text-on-surface-variant">Pending Approval</p>
        <p class="text-3xl font-bold text-status-pending-text tracking-tight mt-1">{pendingCount}</p>
      </a>
      <a href="#/deliveries" class="bg-surface-container-lowest shadow-ambient-sm rounded-xl {failedDeliveries > 0 ? 'border border-status-rejected-text/30' : ''} p-5 hover:shadow-ambient-md transition-shadow">
        <p class="text-sm font-medium text-on-surface-variant">Failed Deliveries</p>
        <p class="text-3xl font-bold {failedDeliveries > 0 ? 'text-status-rejected-text' : 'text-on-surface'} tracking-tight mt-1">{failedDeliveries}</p>
      </a>
      <a href="#/policies" class="bg-surface-container-lowest shadow-ambient-sm rounded-xl p-5 hover:shadow-ambient-md transition-shadow">
        <p class="text-sm font-medium text-on-surface-variant">Policies</p>
        <p class="text-3xl font-bold text-on-surface tracking-tight mt-1">{policyCount}</p>
      </a>
      <a href="#/webhooks" class="bg-surface-container-lowest shadow-ambient-sm rounded-xl p-5 hover:shadow-ambient-md transition-shadow">
        <p class="text-sm font-medium text-on-surface-variant">Webhooks</p>
        <p class="text-3xl font-bold text-on-surface tracking-tight mt-1">{webhookCount}</p>
      </a>
    </div>

    <!-- Recent requests + Quick info -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <!-- Recent requests -->
      <div class="lg:col-span-2">
        <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl">
          <div class="px-6 py-4 border-b border-outline-variant/15 flex items-center justify-between">
            <h2 class="text-sm font-semibold text-on-surface">Recent Requests</h2>
            <a href="#/requests" class="text-xs text-primary-container hover:text-primary">View all</a>
          </div>
          {#if recentRequests.length === 0}
            <div class="px-6 py-8 text-center">
              <p class="text-sm text-on-surface-variant/60">No requests yet.</p>
            </div>
          {:else}
            <ul class="divide-y divide-outline-variant/15">
              {#each recentRequests as req}
                <li>
                  <a href="#/requests/{req.id}" class="flex items-center justify-between px-6 py-3 hover:bg-surface-container-low transition-colors">
                    <div class="flex items-center gap-3">
                      <StatusBadge status={req.status} />
                      <div>
                        <p class="text-sm font-medium text-on-surface">{req.type}</p>
                        <p class="text-xs text-on-surface-variant font-mono">{truncateId(req.id)}</p>
                      </div>
                    </div>
                    <div class="text-right">
                      <p class="text-xs text-on-surface-variant">by {req.maker_id}</p>
                      <p class="text-xs text-on-surface-variant/60">{formatDate(req.created_at)}</p>
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
        <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl">
          <div class="px-6 py-4 border-b border-outline-variant/15 flex items-center justify-between">
            <h2 class="text-sm font-semibold text-on-surface">Policies</h2>
            <a href="#/policies" class="text-xs text-primary-container hover:text-primary">Manage</a>
          </div>
          {#if policies.length === 0}
            <div class="px-6 py-4 text-center">
              <p class="text-sm text-on-surface-variant/60">No policies configured.</p>
              <a href="#/policies/new" class="text-xs text-primary-container hover:text-primary">Create one</a>
            </div>
          {:else}
            <ul class="divide-y divide-outline-variant/15">
              {#each policies.slice(0, 5) as policy}
                <li>
                  <a href="#/policies/{policy.id}" class="flex items-center justify-between px-6 py-3 hover:bg-surface-container-low transition-colors">
                    <div>
                      <p class="text-sm font-medium text-on-surface">{policy.name}</p>
                      <p class="text-xs text-on-surface-variant">{policy.request_type}</p>
                    </div>
                    <span class="text-xs text-on-surface-variant/60">{policy.stages.length} {policy.stages.length === 1 ? 'stage' : 'stages'}</span>
                  </a>
                </li>
              {/each}
            </ul>
          {/if}
        </div>

        <!-- Webhooks list -->
        <div class="bg-surface-container-lowest shadow-ambient-sm rounded-xl">
          <div class="px-6 py-4 border-b border-outline-variant/15 flex items-center justify-between">
            <h2 class="text-sm font-semibold text-on-surface">Webhooks</h2>
            <a href="#/webhooks" class="text-xs text-primary-container hover:text-primary">Manage</a>
          </div>
          {#if webhooks.length === 0}
            <div class="px-6 py-4 text-center">
              <p class="text-sm text-on-surface-variant/60">No webhooks configured.</p>
              <a href="#/webhooks/new" class="text-xs text-primary-container hover:text-primary">Create one</a>
            </div>
          {:else}
            <ul class="divide-y divide-outline-variant/15">
              {#each webhooks.slice(0, 5) as webhook}
                <li class="px-6 py-3">
                  <p class="text-sm font-mono text-xs text-on-surface truncate">{webhook.url}</p>
                  <div class="flex gap-1 mt-1">
                    {#each webhook.events as event}
                      <span class="inline-flex px-1.5 py-0.5 bg-surface-container rounded text-xs text-on-surface-variant">{event}</span>
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
