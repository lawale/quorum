<script lang="ts">
  import { toasts } from '../lib/stores';

  let items: import('../lib/stores').Toast[] = $state([]);
  toasts.subscribe((v) => { items = v; });

  function colorClasses(type: string): string {
    switch (type) {
      case 'success': return 'bg-green-50 border-green-200 text-green-800';
      case 'error': return 'bg-red-50 border-red-200 text-red-800';
      default: return 'bg-blue-50 border-blue-200 text-blue-800';
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
        class="border rounded-lg px-4 py-3 shadow-lg text-sm flex items-center gap-3 min-w-[300px] {colorClasses(toast.type)}"
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
