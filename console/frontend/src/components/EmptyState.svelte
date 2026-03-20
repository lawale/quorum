<script lang="ts">
  let { message, actionLabel, onaction }: {
    message: string;
    actionLabel?: string;
    onaction?: () => void;
  } = $props();
</script>

<div class="text-center py-12">
  <p class="text-on-surface-variant text-sm">{message}</p>
  {#if actionLabel && onaction}
    <button
      onclick={onaction}
      class="mt-4 inline-flex items-center px-4 py-2 text-sm font-medium text-on-primary bg-gradient-to-br from-primary to-primary-container rounded-md hover:brightness-110 transition-all"
    >
      {actionLabel}
    </button>
  {/if}
</div>
