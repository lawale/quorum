<script lang="ts">
  import type { ApprovalStage, Approval } from '../../lib/types';

  let { stages, currentStage, approvals, status }: {
    stages: ApprovalStage[];
    currentStage: number;
    approvals: Approval[];
    status: string;
  } = $props();

  function approvalsForStage(stageIndex: number): number {
    return approvals.filter(
      (a) => a.stage_index === stageIndex && a.decision === 'approved'
    ).length;
  }

  function stageState(stageIndex: number): 'complete' | 'current' | 'upcoming' {
    if (status !== 'pending') {
      return stageIndex <= currentStage ? 'complete' : 'upcoming';
    }
    if (stageIndex < currentStage) return 'complete';
    if (stageIndex === currentStage) return 'current';
    return 'upcoming';
  }
</script>

<div class="stage-bar">
  {#each stages as stage, i}
    {@const state = stageState(i)}
    {@const count = approvalsForStage(i)}
    <div class="stage" class:complete={state === 'complete'} class:current={state === 'current'} class:upcoming={state === 'upcoming'}>
      <div class="indicator">
        {#if state === 'complete'}
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
            <polyline points="20 6 9 17 4 12" />
          </svg>
        {:else}
          <span class="number">{i + 1}</span>
        {/if}
      </div>
      <div class="label">
        <span class="name">{stage.name || `Stage ${i + 1}`}</span>
        <span class="count">{count}/{stage.required_approvals}</span>
      </div>
    </div>
    {#if i < stages.length - 1}
      <div class="connector" class:filled={state === 'complete'}></div>
    {/if}
  {/each}
</div>

<style>
  .stage-bar {
    display: flex;
    align-items: center;
    gap: 0;
    width: 100%;
  }
  .stage {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    min-width: 60px;
  }
  .indicator {
    width: 28px;
    height: 28px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    font-weight: 600;
    border: 2px solid #d1d5db;
    color: #9ca3af;
    background: #fff;
    transition: all 0.2s;
  }
  .complete .indicator {
    background: #10b981;
    border-color: #10b981;
    color: #fff;
  }
  .current .indicator {
    border-color: #6366f1;
    color: #6366f1;
    background: #eef2ff;
  }
  .number { font-size: 11px; }
  .label {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1px;
  }
  .name {
    font-size: 11px;
    color: #374151;
    font-weight: 500;
    text-align: center;
    max-width: 80px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .count {
    font-size: 10px;
    color: #9ca3af;
  }
  .connector {
    flex: 1;
    height: 2px;
    background: #e5e7eb;
    min-width: 20px;
    margin-bottom: 24px;
    transition: background 0.2s;
  }
  .connector.filled {
    background: #10b981;
  }
</style>
