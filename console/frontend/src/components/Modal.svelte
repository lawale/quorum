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
    <!-- Backdrop (frosted glass) -->
    <div
      class="fixed inset-0 bg-on-surface/40 backdrop-blur-sm"
      onclick={onclose}
      role="button"
      tabindex="-1"
      onkeydown={(e) => e.key === 'Escape' && onclose()}
    ></div>

    <!-- Modal content -->
    <div class="relative bg-surface-container-lowest rounded-xl shadow-ambient-lg max-w-lg w-full mx-4 max-h-[90vh] overflow-y-auto">
      <div class="flex items-center justify-between px-6 py-4">
        <h2 class="text-lg font-semibold text-on-surface">{title}</h2>
        <button
          onclick={onclose}
          class="text-on-surface-variant hover:text-on-surface text-xl leading-none"
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
