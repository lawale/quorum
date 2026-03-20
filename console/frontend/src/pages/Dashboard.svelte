<script lang="ts">
  import { policies as policiesApi, webhooks as webhooksApi, requests as requestsApi, deliveries as deliveriesApi, tenants as tenantsApi, operators as operatorsApi } from '../lib/api';
  import { selectedTenant } from '../lib/stores';
  import { formatDate, timeAgo } from '../lib/utils';
  import StatusBadge from '../components/StatusBadge.svelte';
  import LoadingSpinner from '../components/LoadingSpinner.svelte';
  import type { Policy, Webhook, Request } from '../lib/types';

  let isLoading = $state(true);
  let policyCount = $state(0);
  let webhookCount = $state(0);
  let pendingCount = $state(0);
  let totalRequests = $state(0);
  let tenantCount = $state(0);
  let operatorCount = $state(0);
  let failedDeliveries = $state(0);
  let pendingDeliveries = $state(0);
  let deliveredCount = $state(0);
  let deliveryRate = $state(0);
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
      const [policiesRes, webhooksRes, pendingRes, allRes, statsRes, tenantsRes, operatorsRes] = await Promise.all([
        policiesApi.list(),
        webhooksApi.list(),
        requestsApi.list({ page: 1, per_page: 5, status: 'pending' }),
        requestsApi.list({ page: 1, per_page: 5 }),
        deliveriesApi.stats().catch(() => ({ failed: 0, pending: 0, delivered: 0 })),
        tenantsApi.list().catch(() => ({ data: [], total: 0 })),
        operatorsApi.list().catch(() => ({ data: [], total: 0 })),
      ]);

      policies = policiesRes.data || [];
      webhooks = webhooksRes.data || [];
      policyCount = policiesRes.total ?? policies.length;
      webhookCount = webhooksRes.total ?? webhooks.length;
      pendingCount = pendingRes.total ?? 0;
      totalRequests = allRes.total ?? 0;
      recentRequests = allRes.data || [];
      tenantCount = tenantsRes.total ?? (tenantsRes.data || []).length;
      operatorCount = operatorsRes.total ?? (operatorsRes.data || []).length;

      const stats = statsRes as Record<string, number>;
      failedDeliveries = stats.failed ?? 0;
      pendingDeliveries = stats.pending ?? 0;
      deliveredCount = stats.delivered ?? 0;
      const processingCount = stats.processing ?? 0;
      const totalDeliveries = deliveredCount + failedDeliveries + pendingDeliveries + processingCount;
      deliveryRate = totalDeliveries > 0 ? Math.round((deliveredCount / totalDeliveries) * 1000) / 10 : 100;
    } catch {
      // silently fail — cards show 0
    } finally {
      isLoading = false;
    }
  }

  function truncateId(id: string): string {
    return id.length > 8 ? id.slice(0, 8) + '\u2026' : id;
  }
</script>

<div>
  <!-- Header -->
  <section class="flex flex-col md:flex-row md:items-end justify-between gap-6 mb-10">
    <div>
      <h1 class="text-4xl font-extrabold tracking-tight text-on-surface mb-2">Dashboard</h1>
      <p class="text-on-surface-variant max-w-lg">Overview of cross-tenant approval activity and delivery health across all active policy engines.</p>
    </div>
    <div class="flex gap-3">
      <a href="#/requests" class="bg-surface-container-high text-on-surface px-4 py-2 rounded-md font-medium text-sm hover:brightness-95 transition-all flex items-center gap-2">
        View Requests
      </a>
      <a href="#/policies/new" class="bg-gradient-to-br from-primary to-primary-container text-on-primary px-5 py-2 rounded-md font-semibold text-sm shadow-lg shadow-primary/20 hover:brightness-110 transition-all flex items-center gap-2">
        New Policy
      </a>
    </div>
  </section>

  {#if isLoading}
    <LoadingSpinner />
  {:else}
    <!-- Summary Stat Cards (Bento Style) -->
    <section class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6 mb-12">
      <!-- Pending Requests -->
      <a href="#/requests" class="bg-surface-container-lowest p-6 rounded-xl shadow-ambient-lg group hover:scale-[1.02] transition-transform duration-300">
        <div class="flex justify-between items-start mb-4">
          <div class="w-12 h-12 rounded-lg bg-amber-50 flex items-center justify-center">
            <svg class="w-6 h-6 text-amber-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
          </div>
          {#if pendingCount > 0}
            <span class="text-amber-600 text-[10px] font-bold uppercase tracking-widest bg-amber-50 px-2 py-1 rounded">Action Required</span>
          {/if}
        </div>
        <p class="text-[44px] font-black tracking-tighter text-on-surface leading-none">{pendingCount}</p>
        <p class="text-on-surface-variant font-medium mt-1">Pending Requests</p>
        <p class="text-[10px] text-on-surface-variant/60 mt-0.5">{totalRequests} total</p>
      </a>

      <!-- Total Policies -->
      <a href="#/policies" class="bg-surface-container-lowest p-6 rounded-xl shadow-ambient-lg group hover:scale-[1.02] transition-transform duration-300">
        <div class="flex justify-between items-start mb-4">
          <div class="w-12 h-12 rounded-lg bg-indigo-50 flex items-center justify-center">
            <svg class="w-6 h-6 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" /></svg>
          </div>
          <span class="text-primary-container text-[10px] font-bold uppercase tracking-widest bg-indigo-50 px-2 py-1 rounded">Stable</span>
        </div>
        <p class="text-[44px] font-black tracking-tighter text-on-surface leading-none">{policyCount}</p>
        <p class="text-on-surface-variant font-medium mt-1">Total Policies</p>
      </a>

      <!-- Active Tenants -->
      <a href="#/tenants" class="bg-surface-container-lowest p-6 rounded-xl shadow-ambient-lg group hover:scale-[1.02] transition-transform duration-300">
        <div class="flex justify-between items-start mb-4">
          <div class="w-12 h-12 rounded-lg bg-emerald-50 flex items-center justify-center">
            <svg class="w-6 h-6 text-emerald-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M3.75 21h16.5M4.5 3h15M5.25 3v18m13.5-18v18M9 6.75h1.5m-1.5 3h1.5m-1.5 3h1.5m3-6H15m-1.5 3H15m-1.5 3H15M9 21v-3.375c0-.621.504-1.125 1.125-1.125h3.75c.621 0 1.125.504 1.125 1.125V21" /></svg>
          </div>
          <span class="text-emerald-600 text-[10px] font-bold uppercase tracking-widest bg-emerald-50 px-2 py-1 rounded">Live</span>
        </div>
        <p class="text-[44px] font-black tracking-tighter text-on-surface leading-none">{tenantCount}</p>
        <p class="text-on-surface-variant font-medium mt-1">Active Tenants</p>
      </a>

      <!-- Operators -->
      <a href="#/operators" class="bg-surface-container-lowest p-6 rounded-xl shadow-ambient-lg group hover:scale-[1.02] transition-transform duration-300">
        <div class="flex justify-between items-start mb-4">
          <div class="w-12 h-12 rounded-lg bg-purple-50 flex items-center justify-center">
            <svg class="w-6 h-6 text-purple-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M15 19.128a9.38 9.38 0 002.625.372 9.337 9.337 0 004.121-.952 4.125 4.125 0 00-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 018.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0111.964-3.07M12 6.375a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zm8.25 2.25a2.625 2.625 0 11-5.25 0 2.625 2.625 0 015.25 0z" /></svg>
          </div>
          <span class="text-purple-600 text-[10px] font-bold uppercase tracking-widest bg-purple-50 px-2 py-1 rounded">Online</span>
        </div>
        <p class="text-[44px] font-black tracking-tighter text-on-surface leading-none">{operatorCount}</p>
        <p class="text-on-surface-variant font-medium mt-1">Active Operators</p>
      </a>
    </section>

    <!-- Recent Requests + Delivery Health -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-12">
      <!-- Recent Requests Table (2/3 width) -->
      <section class="lg:col-span-2 space-y-6">
        <div class="flex items-center justify-between">
          <h3 class="text-xl font-bold tracking-tight text-on-surface">Recent Requests</h3>
          <a href="#/requests" class="text-primary font-semibold text-sm hover:underline">View all activity</a>
        </div>
        <div class="bg-surface-container-lowest rounded-xl shadow-ambient-lg overflow-hidden">
          {#if recentRequests.length === 0}
            <div class="px-6 py-12 text-center">
              <p class="text-sm text-on-surface-variant">No requests yet.</p>
              <p class="text-xs text-on-surface-variant/60 mt-1">Requests will appear here once created.</p>
            </div>
          {:else}
            <table class="w-full text-left border-collapse">
              <thead>
                <tr class="bg-surface-container-low">
                  <th class="px-6 py-4 text-[10px] text-on-surface-variant font-bold uppercase tracking-widest border-b border-outline-variant/15">Type</th>
                  <th class="px-6 py-4 text-[10px] text-on-surface-variant font-bold uppercase tracking-widest border-b border-outline-variant/15">Maker</th>
                  <th class="px-6 py-4 text-[10px] text-on-surface-variant font-bold uppercase tracking-widest border-b border-outline-variant/15">Created</th>
                  <th class="px-6 py-4 text-[10px] text-on-surface-variant font-bold uppercase tracking-widest border-b border-outline-variant/15">Status</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-outline-variant/10">
                {#each recentRequests as req}
                  <tr class="hover:bg-surface-container-low transition-colors cursor-pointer group" onclick={() => window.location.hash = `#/requests/${req.id}`}>
                    <td class="px-6 py-5">
                      <span class="text-sm font-semibold text-on-surface">{req.type}</span>
                    </td>
                    <td class="px-6 py-5">
                      <div class="flex flex-col">
                        <span class="text-sm font-bold text-on-surface">{req.maker_id}</span>
                        <span class="text-[10px] font-mono text-on-surface-variant">{truncateId(req.id)}</span>
                      </div>
                    </td>
                    <td class="px-6 py-5 text-sm text-on-surface-variant">{timeAgo(req.created_at)}</td>
                    <td class="px-6 py-5">
                      <StatusBadge status={req.status} />
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {/if}
        </div>
      </section>

      <!-- Delivery Health & Quick Info (1/3 width) -->
      <section class="space-y-8">
        <!-- Delivery Health Card -->
        <div class="space-y-4">
          <h3 class="text-xl font-bold tracking-tight text-on-surface">Delivery Health</h3>
          <div class="bg-surface-container-lowest p-6 rounded-xl shadow-ambient-lg border-l-4 {failedDeliveries > 0 ? 'border-status-rejected-text' : 'border-emerald-500'}">
            <div class="flex justify-between items-center mb-6">
              <p class="text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Webhook Deliveries</p>
              {#if failedDeliveries === 0}
                <svg class="w-5 h-5 text-emerald-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
              {:else}
                <svg class="w-5 h-5 text-status-rejected-text" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" /></svg>
              {/if}
            </div>
            <div class="space-y-4">
              <div>
                <div class="flex justify-between text-xs font-bold mb-1">
                  <span class="text-on-surface">Delivered</span>
                  <span class="text-emerald-600">{deliveryRate}%</span>
                </div>
                <div class="w-full h-1.5 bg-surface-container rounded-full overflow-hidden">
                  <div class="bg-emerald-500 h-full transition-all duration-500" style="width: {deliveryRate}%"></div>
                </div>
              </div>
              <div class="grid grid-cols-2 gap-4 pt-2">
                <div class="bg-surface-container-low p-3 rounded-lg">
                  <p class="text-[10px] font-bold text-status-rejected-text uppercase">Failed</p>
                  <p class="text-lg font-black text-on-surface leading-tight">{failedDeliveries}</p>
                </div>
                <div class="bg-surface-container-low p-3 rounded-lg">
                  <p class="text-[10px] font-bold text-status-pending-text uppercase">Pending</p>
                  <p class="text-lg font-black text-on-surface leading-tight">{pendingDeliveries}</p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Policies Quick List -->
        <div class="bg-surface-container-lowest p-6 rounded-xl shadow-ambient-lg">
          <div class="flex items-center justify-between mb-4">
            <h4 class="text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Active Policies</h4>
            <a href="#/policies" class="text-xs text-primary font-semibold hover:underline">Manage</a>
          </div>
          {#if policies.length === 0}
            <p class="text-sm text-on-surface-variant">No policies configured.</p>
          {:else}
            <div class="space-y-3">
              {#each policies.slice(0, 4) as policy}
                <a href="#/policies/{policy.id}" class="flex items-center justify-between group">
                  <div>
                    <p class="text-sm font-bold text-on-surface group-hover:text-primary transition-colors">{policy.name}</p>
                    <p class="text-[10px] text-on-surface-variant">{policy.request_type}</p>
                  </div>
                  <span class="text-[10px] font-medium text-on-surface-variant bg-surface-container-low px-2 py-0.5 rounded">{policy.stages.length} {policy.stages.length === 1 ? 'stage' : 'stages'}</span>
                </a>
              {/each}
            </div>
          {/if}
        </div>

        <!-- Webhooks Quick List -->
        <div class="bg-surface-container-lowest p-6 rounded-xl shadow-ambient-lg">
          <div class="flex items-center justify-between mb-4">
            <h4 class="text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">Webhooks</h4>
            <a href="#/webhooks" class="text-xs text-primary font-semibold hover:underline">Manage</a>
          </div>
          {#if webhooks.length === 0}
            <p class="text-sm text-on-surface-variant">No webhooks configured.</p>
          {:else}
            <div class="space-y-3">
              {#each webhooks.slice(0, 4) as webhook}
                <div>
                  <p class="text-xs font-mono text-on-surface truncate">{webhook.url}</p>
                  <div class="flex gap-1 mt-1">
                    {#each webhook.events as event}
                      <span class="inline-flex px-1.5 py-0.5 bg-surface-container rounded text-[10px] text-on-surface-variant">{event}</span>
                    {/each}
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      </section>
    </div>
  {/if}
</div>
