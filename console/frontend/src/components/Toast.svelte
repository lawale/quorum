<script lang="ts">
  import { toasts } from '../lib/stores';

  let items: import('../lib/stores').Toast[] = $state([]);
  toasts.subscribe((v) => { items = v; });

  function colorClasses(type: string): string {
    switch (type) {
      case 'success': return 'bg-status-approved-bg border-outline-variant/15 text-status-approved-text';
      case 'error': return 'bg-status-rejected-bg border-outline-variant/15 text-status-rejected-text';
      default: return 'bg-surface-container-lowest border-outline-variant/15 text-on-surface';
    }
  }

  function dismiss(id: number) {
    toasts.update((t) => t.filter((x) => x.id !== id));
  }
</script>

{#if items.length > 0}
  <div class="fixed bottom-4 right-4 z-50 space-y-2">
    {#each items as toast (toast.id)}
      <div
        class="border rounded-xl px-4 py-3 shadow-ambient-md text-sm flex items-center gap-3 min-w-[300px] {colorClasses(toast.type)}"
      >
        <span class="flex-1">{toast.message}</span>
        <button
          onclick={() => dismiss(toast.id)}
          class="text-current opacity-50 hover:opacity-100"
        >
          &times;
        </button>
      </div>
    {/each}
  </div>
{/if}
