<script lang="ts">
  import { statusColor, capitalize } from '../lib/utils';

  let { status }: { status: string } = $props();
</script>

<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {statusColor(status)}">
  {capitalize(status)}
</span>
