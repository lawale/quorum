<script lang="ts">
  import type { Snippet } from 'svelte';

  let { open, title, onclose, children }: {
    open: boolean;
    title: string;
    onclose: () => void;
    children: Snippet;
  } = $props();
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center">
    <!-- Backdrop -->
    <div
      class="fixed inset-0 bg-black/50"
      onclick={onclose}
      role="button"
      tabindex="-1"
      onkeydown={(e) => e.key === 'Escape' && onclose()}
    ></div>

    <!-- Modal content -->
    <div class="relative bg-white rounded-lg shadow-xl max-w-lg w-full mx-4 max-h-[90vh] overflow-y-auto">
      <div class="flex items-center justify-between px-6 py-4 border-b border-gray-200">
        <h2 class="text-lg font-semibold text-gray-900">{title}</h2>
        <button
          onclick={onclose}
          class="text-gray-400 hover:text-gray-600 text-xl leading-none"
        >
          &times;
        </button>
      </div>
      <div class="px-6 py-4">
        {@render children()}
      </div>
    </div>
  </div>
{/if}
