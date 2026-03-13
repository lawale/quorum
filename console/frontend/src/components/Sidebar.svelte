<script lang="ts">
  import { currentUser } from '../lib/stores';
  import { clearToken } from '../lib/auth';

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
    { label: 'Audit Log', href: '#/audit', icon: '📝' },
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

  function handleLogout() {
    clearToken();
    currentUser.set(null);
    window.location.hash = '#/login';
  }

  let user: ReturnType<typeof $state<import('../lib/types').Operator | null>> = $state(null);
  currentUser.subscribe((v) => { user = v; });
</script>

<aside class="w-64 bg-gray-900 text-white flex flex-col min-h-screen">
  <div class="px-6 py-5 border-b border-gray-700">
    <h1 class="text-xl font-bold">Quorum</h1>
    <p class="text-gray-400 text-xs mt-1">Admin Console</p>
  </div>

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
