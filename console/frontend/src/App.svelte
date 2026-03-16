<script lang="ts">
  import { auth as authApi, tenants as tenantsApi } from './lib/api';
  import { isAuthenticated, getStoredOperator, clearToken } from './lib/auth';
  import { currentUser, availableTenants } from './lib/stores';
  import { operators } from './lib/api';
  import Layout from './components/Layout.svelte';
  import Login from './pages/Login.svelte';
  import Setup from './pages/Setup.svelte';
  import Dashboard from './pages/Dashboard.svelte';
  import Policies from './pages/Policies.svelte';
  import PolicyForm from './pages/PolicyForm.svelte';
  import Webhooks from './pages/Webhooks.svelte';
  import WebhookForm from './pages/WebhookForm.svelte';
  import Requests from './pages/Requests.svelte';
  import RequestDetail from './pages/RequestDetail.svelte';
  import AuditLogs from './pages/AuditLogs.svelte';
  import Operators from './pages/Operators.svelte';
  import Tenants from './pages/Tenants.svelte';
  import Deliveries from './pages/Deliveries.svelte';

  type AppState = 'loading' | 'setup' | 'login' | 'app';

  let state: AppState = $state('loading');
  let hash = $state(window.location.hash || '#/');

  function onHashChange() {
    hash = window.location.hash || '#/';
  }

  $effect(() => {
    window.addEventListener('hashchange', onHashChange);
    return () => window.removeEventListener('hashchange', onHashChange);
  });

  // Initialize app — check setup status and auth
  $effect(() => {
    init();
  });

  async function init() {
    try {
      // Check if setup is needed
      const statusRes = await authApi.status();
      if (statusRes.needs_setup) {
        state = 'setup';
        return;
      }

      // Check if we have a valid token
      if (!isAuthenticated()) {
        state = 'login';
        return;
      }

      // Validate token by fetching current operator
      try {
        const op = await operators.me();
        currentUser.set(op);
        // Fetch available tenants for sidebar selector
        try {
          const tenantsRes = await tenantsApi.list();
          availableTenants.set(tenantsRes.data || []);
        } catch {
          // Non-critical — sidebar will just not show tenant selector
        }
        state = 'app';
        // Redirect away from login hash so the app layout renders
        if (window.location.hash === '#/login' || window.location.hash === '') {
          window.location.hash = '#/';
        }
      } catch {
        clearToken();
        state = 'login';
      }
    } catch {
      // API not available — show login, errors will surface there
      if (isAuthenticated()) {
        const stored = getStoredOperator();
        if (stored) {
          currentUser.set(stored);
          state = 'app';
          return;
        }
      }
      state = 'login';
    }
  }

  // Parse hash route
  function parseRoute(h: string): { path: string; params: Record<string, string> } {
    const raw = h.replace(/^#/, '') || '/';
    const parts = raw.split('/').filter(Boolean);

    if (parts.length === 2 && parts[0] === 'requests') {
      return { path: 'request-detail', params: { id: parts[1] } };
    }
    if (parts.length === 2 && parts[0] === 'policies' && parts[1] === 'new') {
      return { path: 'policy-form', params: {} };
    }
    if (parts.length === 2 && parts[0] === 'policies') {
      return { path: 'policy-form', params: { id: parts[1] } };
    }
    if (parts.length === 2 && parts[0] === 'webhooks' && parts[1] === 'new') {
      return { path: 'webhook-form', params: {} };
    }

    return { path: parts[0] || 'dashboard', params: {} };
  }

  let route = $derived(parseRoute(hash));
</script>

{#if state === 'loading'}
  <div class="min-h-screen bg-gray-50 flex items-center justify-center">
    <div class="text-center">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto"></div>
      <p class="text-gray-500 mt-4">Loading...</p>
    </div>
  </div>
{:else if state === 'setup'}
  <Setup onSuccess={() => init()} />
{:else if state === 'login'}
  <Login onSuccess={() => init()} />
{:else}
  <Layout>
    {#snippet children()}
      {#if route.path === 'dashboard'}
        <Dashboard />
      {:else if route.path === 'policies'}
        <Policies />
      {:else if route.path === 'policy-form'}
        <PolicyForm />
      {:else if route.path === 'webhooks'}
        <Webhooks />
      {:else if route.path === 'webhook-form'}
        <WebhookForm />
      {:else if route.path === 'requests'}
        <Requests />
      {:else if route.path === 'deliveries'}
        <Deliveries />
      {:else if route.path === 'request-detail'}
        <RequestDetail id={route.params.id} />
      {:else if route.path === 'audit'}
        <AuditLogs />
      {:else if route.path === 'tenants'}
        <Tenants />
      {:else if route.path === 'operators'}
        <Operators />
      {:else}
        <Dashboard />
      {/if}
    {/snippet}
  </Layout>
{/if}
