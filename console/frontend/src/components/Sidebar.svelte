<script lang="ts">
  import { currentUser, selectedTenant, availableTenants } from '../lib/stores';
  import { clearSession } from '../lib/auth';
  import { auth as authApi } from '../lib/api';
  import type { Tenant } from '../lib/types';

  interface NavItem {
    label: string;
    href: string;
    icon: string;
  }

  const navItems: NavItem[] = [
    { label: 'Dashboard', href: '#/', icon: '🏠' },
    { label: 'Policies', href: '#/policies', icon: '📋' },
    { label: 'Webhooks', href: '#/webhooks', icon: '🔗' },
    { label: 'Requests', href: '#/requests', icon: '📨' },
    { label: 'Deliveries', href: '#/deliveries', icon: '📡' },
    { label: 'Audit Log', href: '#/audit', icon: '📝' },
    { label: 'Tenants', href: '#/tenants', icon: '🏢' },
    { label: 'Operators', href: '#/operators', icon: '👤' },
  ];

  let hash = $state(window.location.hash || '#/');

  function onHashChange() {
    hash = window.location.hash || '#/';
  }

  $effect(() => {
    window.addEventListener('hashchange', onHashChange);
    return () => window.removeEventListener('hashchange', onHashChange);
  });

  function isActive(href: string): boolean {
    if (href === '#/') return hash === '#/' || hash === '#';
    return hash.startsWith(href);
  }

  async function handleLogout() {
    try { await authApi.logout(); } catch { /* ignore */ }
    clearSession();
    currentUser.set(null);
    window.location.hash = '#/login';
  }

  let user: ReturnType<typeof $state<import('../lib/types').Operator | null>> = $state(null);
  currentUser.subscribe((v) => { user = v; });

  let tenantsList: Tenant[] = $state([]);
  availableTenants.subscribe((v) => { tenantsList = v; });

  let currentTenant = $state('');
  selectedTenant.subscribe((v) => { currentTenant = v; });

  function handleTenantChange(e: Event) {
    const value = (e.target as HTMLSelectElement).value;
    selectedTenant.set(value);
  }
</script>

<aside class="w-64 bg-gray-900 text-white flex flex-col min-h-screen">
  <div class="px-6 py-5 border-b border-gray-700">
    <h1 class="text-xl font-bold">Quorum</h1>
    <p class="text-gray-400 text-xs mt-1">Admin Console</p>
  </div>

  <!-- Tenant selector -->
  {#if tenantsList.length > 0}
    <div class="px-3 py-3 border-b border-gray-700">
      <label for="tenant-select" class="block text-xs font-medium text-gray-400 mb-1 px-1">Tenant</label>
      <select
        id="tenant-select"
        onchange={handleTenantChange}
        value={currentTenant}
        class="w-full bg-gray-800 text-white text-sm rounded-md border border-gray-600 px-2 py-1.5 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
      >
        <option value="">All Tenants</option>
        {#each tenantsList as t}
          <option value={t.slug}>{t.name} ({t.slug})</option>
        {/each}
      </select>
    </div>
  {/if}

  <nav class="flex-1 px-3 py-4 space-y-1">
    {#each navItems as item}
      <a
        href={item.href}
        class="flex items-center px-3 py-2 text-sm rounded-md transition-colors {isActive(item.href) ? 'bg-gray-800 text-white' : 'text-gray-300 hover:bg-gray-800 hover:text-white'}"
      >
        <span class="mr-3">{item.icon}</span>
        {item.label}
      </a>
    {/each}
  </nav>

  <div class="px-3 py-4 border-t border-gray-700">
    {#if user}
      <div class="px-3 py-2 text-sm text-gray-400">
        <div class="font-medium text-gray-200">{user.display_name || user.username}</div>
        <div class="text-xs text-gray-500">@{user.username}</div>
      </div>
    {/if}
    <button
      onclick={handleLogout}
      class="w-full mt-2 flex items-center px-3 py-2 text-sm text-gray-300 hover:bg-gray-800 hover:text-white rounded-md transition-colors"
    >
      <span class="mr-3">🚪</span>
      Sign out
    </button>
  </div>
</aside>
