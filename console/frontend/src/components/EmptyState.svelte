<script lang="ts">
  let { message, actionLabel, onaction }: {
    message: string;
    actionLabel?: string;
    onaction?: () => void;
  } = $props();
</script>

<div class="text-center py-12">
  <p class="text-gray-500 text-sm">{message}</p>
  {#if actionLabel && onaction}
    <button
      onclick={onaction}
      class="mt-4 inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-indigo-600 rounded-md hover:bg-indigo-700 transition-colors"
    >
      {actionLabel}
    </button>
  {/if}
</div>
