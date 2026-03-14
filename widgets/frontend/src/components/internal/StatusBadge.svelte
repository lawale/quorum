<script lang="ts">
  import { STATUS_COLORS, capitalize } from '../../lib/utils';
  import type { RequestStatus } from '../../lib/types';

  let { status }: { status: RequestStatus } = $props();

  let colors = $derived(STATUS_COLORS[status] ?? STATUS_COLORS.pending);
</script>

<span class="badge" style="background:{colors.bg};color:{colors.text}">
  {capitalize(status)}
</span>

<style>
  .badge {
    display: inline-block;
    padding: 2px 8px;
    border-radius: 9999px;
    font-size: 12px;
    font-weight: 500;
    line-height: 1.4;
    white-space: nowrap;
  }
</style>
