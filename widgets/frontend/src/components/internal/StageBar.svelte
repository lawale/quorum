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

  function stageState(stageIndex: number): 'complete' | 'current' | 'upcoming' | 'failed' {
    if (status === 'approved') {
      return stageIndex <= currentStage ? 'complete' : 'upcoming';
    }
    if (status === 'rejected' || status === 'cancelled' || status === 'expired') {
      if (stageIndex < currentStage) return 'complete';
      if (stageIndex === currentStage) return 'failed';
      return 'upcoming';
    }
    if (stageIndex < currentStage) return 'complete';
    if (stageIndex === currentStage) return 'current';
    return 'upcoming';
  }

  function connectorProgress(nextStageIndex: number): number {
    const nextState = stageState(nextStageIndex);
    if (nextState === 'complete') return 1;
    if (nextState === 'upcoming') return 0;
    // current or failed — show partial progress based on actual approvals
    const required = stages[nextStageIndex]?.required_approvals ?? 1;
    const obtained = approvalsForStage(nextStageIndex);
    if (required <= 0) return 1;
    return Math.min(obtained / required, 1);
  }

  /** Progress for the connector from Start into Stage 0. */
  let startProgress = $derived(stages.length > 0 ? connectorProgress(0) : 1);
</script>

<div class="stage-bar">
  <!-- Start node -->
  <div class="stage start-stage" class:complete={true}>
    <div class="indicator start-indicator">
      <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor">
        <circle cx="12" cy="12" r="10" />
      </svg>
    </div>
    <div class="label">
      <span class="name">Start</span>
    </div>
  </div>
  <div class="connector" class:filled={startProgress === 1}>
    {#if startProgress > 0 && startProgress < 1}
      <div class="connector-progress" style="width: {startProgress * 100}%"></div>
    {/if}
  </div>

  {#each stages as stage, i}
    {@const state = stageState(i)}
    {@const count = approvalsForStage(i)}
    <div class="stage" class:complete={state === 'complete'} class:current={state === 'current'} class:upcoming={state === 'upcoming'} class:failed={state === 'failed'}>
      <div class="indicator">
        {#if state === 'complete'}
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
            <polyline points="20 6 9 17 4 12" />
          </svg>
        {:else if state === 'failed'}
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
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
      {@const nextProgress = connectorProgress(i + 1)}
      <div class="connector" class:filled={nextProgress === 1}>
        {#if nextProgress > 0 && nextProgress < 1}
          <div class="connector-progress" style="width: {nextProgress * 100}%"></div>
        {/if}
      </div>
    {/if}
  {/each}
</div>

<style>
  .stage-bar {
    display: flex;
    align-items: center;
    gap: 0;
    width: 100%;
    overflow-x: auto;
    scrollbar-width: thin;
    scrollbar-color: #d1d5db transparent;
  }
  .stage-bar::-webkit-scrollbar { height: 4px; }
  .stage-bar::-webkit-scrollbar-track { background: transparent; }
  .stage-bar::-webkit-scrollbar-thumb { background: #d1d5db; border-radius: 2px; }
  .stage {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    min-width: 72px;
    flex-shrink: 0;
  }
  .start-stage {
    min-width: 40px;
    flex-shrink: 0;
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
  .start-indicator {
    width: 20px;
    height: 20px;
    border: none;
    background: #10b981;
    color: #fff;
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
  .failed .indicator {
    border-color: #ef4444;
    color: #ef4444;
    background: #fef2f2;
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
    max-width: 100px;
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
    min-width: 48px;
    flex-shrink: 0;
    margin-bottom: 24px;
    transition: background 0.2s;
    position: relative;
    overflow: hidden;
  }
  .connector.filled {
    background: #10b981;
  }
  .connector-progress {
    position: absolute;
    top: 0;
    left: 0;
    height: 100%;
    background: #10b981;
    transition: width 0.3s ease;
  }
</style>
