<script lang="ts">
  let { size = 'md' }: { size?: 'sm' | 'md' | 'lg' } = $props();

  const sizeClasses = {
    sm: 'h-4 w-4',
    md: 'h-8 w-8',
    lg: 'h-12 w-12',
  };
</script>

<div class="flex items-center justify-center py-8">
  <div class="animate-spin rounded-full border-b-2 border-indigo-600 {sizeClasses[size]}"></div>
</div>
