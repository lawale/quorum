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
    { label: 'Dashboard', href: '#/', icon: 'dashboard' },
    { label: 'Tenants', href: '#/tenants', icon: 'tenants' },
    { label: 'Policies', href: '#/policies', icon: 'policies' },
    { label: 'Requests', href: '#/requests', icon: 'requests' },
    { label: 'Webhooks', href: '#/webhooks', icon: 'webhooks' },
    { label: 'Deliveries', href: '#/deliveries', icon: 'deliveries' },
    { label: 'Audit Log', href: '#/audit', icon: 'audit' },
    { label: 'Operators', href: '#/operators', icon: 'operators' },
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

<aside class="w-64 bg-sidebar text-white flex flex-col sticky top-0 h-screen overflow-y-auto">
  <!-- Logo -->
  <div class="px-6 py-5 flex items-center gap-3">
    <div class="w-9 h-9 rounded-lg bg-primary flex items-center justify-center shrink-0">
      <svg class="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
        <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6A2.25 2.25 0 016 3.75h2.25A2.25 2.25 0 0110.5 6v2.25a2.25 2.25 0 01-2.25 2.25H6a2.25 2.25 0 01-2.25-2.25V6zM3.75 15.75A2.25 2.25 0 016 13.5h2.25a2.25 2.25 0 012.25 2.25V18a2.25 2.25 0 01-2.25 2.25H6A2.25 2.25 0 013.75 18v-2.25zM13.5 6a2.25 2.25 0 012.25-2.25H18A2.25 2.25 0 0120.25 6v2.25A2.25 2.25 0 0118 10.5h-2.25a2.25 2.25 0 01-2.25-2.25V6zM13.5 15.75a2.25 2.25 0 012.25-2.25H18a2.25 2.25 0 012.25 2.25V18A2.25 2.25 0 0118 20.25h-2.25A2.25 2.25 0 0113.5 18v-2.25z" />
      </svg>
    </div>
    <div>
      <h1 class="text-xl font-bold tracking-tight leading-none">Quorum</h1>
      <p class="text-[10px] font-medium uppercase tracking-[0.15em] text-white/40 mt-0.5">Admin Console</p>
    </div>
  </div>

  <!-- Tenant selector -->
  {#if tenantsList.length > 0}
    <div class="px-3 py-3">
      <label for="tenant-select" class="block text-xs font-medium text-white/40 mb-1 px-1">Tenant</label>
      <select
        id="tenant-select"
        onchange={handleTenantChange}
        value={currentTenant}
        class="w-full bg-[rgba(255,255,255,0.06)] text-white text-sm rounded-md border border-[rgba(255,255,255,0.1)] px-2 py-1.5 focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary"
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
        class="flex items-center gap-3 px-3 py-2.5 text-sm rounded-lg transition-colors {isActive(item.href) ? 'bg-sidebar-active text-white font-medium' : 'text-white/60 hover:bg-[rgba(255,255,255,0.04)] hover:text-white'}"
      >
        <svg class="w-5 h-5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
          {#if item.icon === 'dashboard'}
            <!-- Grid/dashboard icon -->
            <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6A2.25 2.25 0 016 3.75h2.25A2.25 2.25 0 0110.5 6v2.25a2.25 2.25 0 01-2.25 2.25H6a2.25 2.25 0 01-2.25-2.25V6zM3.75 15.75A2.25 2.25 0 016 13.5h2.25a2.25 2.25 0 012.25 2.25V18a2.25 2.25 0 01-2.25 2.25H6A2.25 2.25 0 013.75 18v-2.25zM13.5 6a2.25 2.25 0 012.25-2.25H18A2.25 2.25 0 0120.25 6v2.25A2.25 2.25 0 0118 10.5h-2.25a2.25 2.25 0 01-2.25-2.25V6zM13.5 15.75a2.25 2.25 0 012.25-2.25H18a2.25 2.25 0 012.25 2.25V18A2.25 2.25 0 0118 20.25h-2.25A2.25 2.25 0 0113.5 18v-2.25z" />
          {:else if item.icon === 'tenants'}
            <!-- Building/organization icon -->
            <path stroke-linecap="round" stroke-linejoin="round" d="M2.25 21h19.5m-18-18v18m10.5-18v18m6-13.5V21M6.75 6.75h.75m-.75 3h.75m-.75 3h.75m3-6h.75m-.75 3h.75m-.75 3h.75M6.75 21v-3.375c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21M3 3h12m-.75 4.5H21m-3.75 3h.008v.008h-.008v-.008zm0 3h.008v.008h-.008v-.008zm0 3h.008v.008h-.008v-.008z" />
          {:else if item.icon === 'policies'}
            <!-- Shield/check icon -->
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" />
          {:else if item.icon === 'requests'}
            <!-- Arrow-right-circle / forward icon -->
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 12L3.269 3.126A59.768 59.768 0 0121.485 12 59.77 59.77 0 013.27 20.876L5.999 12zm0 0h7.5" />
          {:else if item.icon === 'webhooks'}
            <!-- Link/chain icon -->
            <path stroke-linecap="round" stroke-linejoin="round" d="M13.19 8.688a4.5 4.5 0 011.242 7.244l-4.5 4.5a4.5 4.5 0 01-6.364-6.364l1.757-1.757m9.856-9.856a4.5 4.5 0 00-6.364 0l-4.5 4.5a4.5 4.5 0 001.242 7.244" />
          {:else if item.icon === 'deliveries'}
            <!-- Truck icon -->
            <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 18.75a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m3 0h6m-9 0H3.375a1.125 1.125 0 01-1.125-1.125V14.25m17.25 4.5a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m3 0h1.125c.621 0 1.129-.504 1.09-1.124a17.902 17.902 0 00-3.213-9.193 2.056 2.056 0 00-1.58-.86H14.25M16.5 18.75h-2.25m0-11.177v-.958c0-.568-.422-1.048-.987-1.106a48.554 48.554 0 00-10.026 0 1.106 1.106 0 00-.987 1.106v7.635m12-6.677v6.677m0 4.5v-4.5m0 0h-12" />
          {:else if item.icon === 'audit'}
            <!-- Clipboard/document-check icon -->
            <path stroke-linecap="round" stroke-linejoin="round" d="M11.35 3.836c-.065.21-.1.433-.1.664 0 .414.336.75.75.75h4.5a.75.75 0 00.75-.75 2.25 2.25 0 00-.1-.664m-5.8 0A2.251 2.251 0 0113.5 2.25H15c1.012 0 1.867.668 2.15 1.586m-5.8 0c-.376.023-.75.05-1.124.08C9.095 4.01 8.25 4.973 8.25 6.108V8.25m8.9-4.414c.376.023.75.05 1.124.08 1.131.094 1.976 1.057 1.976 2.192V16.5A2.25 2.25 0 0118 18.75h-2.25m-7.5-10.5H4.875c-.621 0-1.125.504-1.125 1.125v11.25c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V18.75m-7.5-10.5h6.375c.621 0 1.125.504 1.125 1.125v9.375m-8.25-3l1.5 1.5 3-3.75" />
          {:else if item.icon === 'operators'}
            <!-- Users/people icon -->
            <path stroke-linecap="round" stroke-linejoin="round" d="M15 19.128a9.38 9.38 0 002.625.372 9.337 9.337 0 004.121-.952 4.125 4.125 0 00-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 018.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0111.964-3.07M12 6.375a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zm8.25 2.25a2.625 2.625 0 11-5.25 0 2.625 2.625 0 015.25 0z" />
          {/if}
        </svg>
        {item.label}
      </a>
    {/each}
  </nav>

  <div class="px-3 py-4">
    {#if user}
      <div class="px-3 py-2 text-sm">
        <div class="font-medium text-white/90">{user.display_name || user.username}</div>
        <div class="text-xs text-white/40">@{user.username}</div>
      </div>
    {/if}
    <button
      onclick={handleLogout}
      class="w-full mt-2 flex items-center gap-3 px-3 py-2.5 text-sm text-white/60 hover:bg-[rgba(255,255,255,0.04)] hover:text-white rounded-lg transition-colors"
    >
      <svg class="w-5 h-5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
        <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0013.5 3h-6a2.25 2.25 0 00-2.25 2.25v13.5A2.25 2.25 0 007.5 21h6a2.25 2.25 0 002.25-2.25V15m3 0l3-3m0 0l-3-3m3 3H9" />
      </svg>
      Sign out
    </button>
  </div>
</aside>
