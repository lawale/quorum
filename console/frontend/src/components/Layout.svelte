<script lang="ts">
  import Sidebar from './Sidebar.svelte';
  import Toast from './Toast.svelte';
  import type { Snippet } from 'svelte';

  let { children }: { children: Snippet } = $props();
</script>

<div class="flex min-h-screen">
  <Sidebar />
  <main class="flex-1 bg-surface">
    <div class="p-8">
      {@render children()}
    </div>
  </main>
  <Toast />
</div>
