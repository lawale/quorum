<script lang="ts">
  import type { AuditLog } from '../../lib/types';
  import { timeAgo, capitalize, formatDetails, ACTION_COLORS } from '../../lib/utils';

  let { logs }: { logs: AuditLog[] } = $props();
</script>

<div class="timeline">
  {#each logs as log}
    {@const color = ACTION_COLORS[log.action] ?? '#6b7280'}
    <div class="entry">
      <div class="dot" style="background:{color}"></div>
      <div class="content">
        <div class="header">
          <span class="action" style="color:{color}">{capitalize(log.action.replace(/_/g, ' '))}</span>
          <span class="time">{timeAgo(log.created_at)}</span>
        </div>
        <div class="actor">by {log.actor_id}</div>
        {#if log.details && Object.keys(log.details).length > 0}
          <div class="details">{formatDetails(log.details, log.action)}</div>
        {/if}
      </div>
    </div>
  {/each}
  {#if logs.length === 0}
    <div class="empty">No audit entries</div>
  {/if}
</div>

<style>
  .timeline {
    display: flex;
    flex-direction: column;
    gap: 0;
  }
  .entry {
    display: flex;
    gap: 12px;
    padding: 8px 0;
    border-left: 2px solid #e5e7eb;
    margin-left: 6px;
    padding-left: 16px;
    position: relative;
  }
  .dot {
    position: absolute;
    left: -5px;
    top: 12px;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }
  .content {
    flex: 1;
    min-width: 0;
  }
  .header {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    gap: 8px;
  }
  .action {
    font-size: 13px;
    font-weight: 600;
  }
  .time {
    font-size: 11px;
    color: #9ca3af;
    white-space: nowrap;
  }
  .actor {
    font-size: 12px;
    color: #6b7280;
    margin-top: 1px;
  }
  .details {
    font-size: 11px;
    color: #9ca3af;
    margin-top: 4px;
    font-family: monospace;
    white-space: pre-line;
    word-break: break-all;
  }
  .empty {
    font-size: 13px;
    color: #9ca3af;
    padding: 8px 0;
  }
</style>
