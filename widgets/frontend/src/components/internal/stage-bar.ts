import { LitElement, css, html, svg } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { classMap } from 'lit/directives/class-map.js';
import { styleMap } from 'lit/directives/style-map.js';
import type { ApprovalStage, Approval } from '../../lib/types';
import { tokens } from '../shared';

type StageState = 'complete' | 'current' | 'upcoming' | 'failed';

const checkIcon = svg`<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><polyline points="20 6 9 17 4 12" /></svg>`;
const crossIcon = svg`<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>`;
const startIcon = svg`<svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor"><circle cx="12" cy="12" r="10" /></svg>`;

@customElement('quorum-stage-bar')
export class QuorumStageBar extends LitElement {
  static styles = [tokens, css`
    :host { display: block; }
    .stage-bar {
      display: flex;
      align-items: center;
      gap: 0;
      width: 100%;
      overflow-x: auto;
      scrollbar-width: thin;
      scrollbar-color: var(--_border) transparent;
    }
    .stage-bar::-webkit-scrollbar { height: 4px; }
    .stage-bar::-webkit-scrollbar-track { background: transparent; }
    .stage-bar::-webkit-scrollbar-thumb { background: var(--_border); border-radius: 2px; }
    .stage {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 4px;
      min-width: 72px;
      flex-shrink: 0;
    }
    .start-stage { min-width: 40px; }
    .indicator {
      width: 28px;
      height: 28px;
      border-radius: 50%;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 12px;
      font-weight: 600;
      border: 2px solid var(--_border);
      color: var(--_muted);
      background: var(--_bg);
      transition: all 0.2s;
    }
    .start-indicator {
      width: 20px;
      height: 20px;
      border: none;
      background: var(--_success);
      color: #fff;
    }
    .complete .indicator { background: var(--_success); border-color: var(--_success); color: #fff; }
    .current .indicator { border-color: var(--_accent); color: var(--_accent); background: color-mix(in srgb, var(--_accent) 8%, transparent); }
    .failed .indicator { border-color: var(--_danger); color: var(--_danger); background: color-mix(in srgb, var(--_danger) 8%, transparent); }
    .number { font-size: 11px; }
    .label { display: flex; flex-direction: column; align-items: center; gap: 1px; }
    .name {
      font-size: 11px;
      color: var(--_fg);
      font-weight: 500;
      text-align: center;
      max-width: 100px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .count { font-size: 10px; color: var(--_muted); }
    .connector {
      flex: 1;
      height: 2px;
      background: var(--_track);
      min-width: 48px;
      flex-shrink: 0;
      margin-bottom: 24px;
      transition: background 0.2s;
      position: relative;
      overflow: hidden;
    }
    .connector.filled { background: var(--_success); }
    .connector-progress {
      position: absolute;
      top: 0;
      left: 0;
      height: 100%;
      background: var(--_success);
      transition: width 0.3s ease;
    }
  `];

  @property({ attribute: false }) stages: ApprovalStage[] = [];
  @property({ attribute: false }) approvals: Approval[] = [];
  @property({ type: Number, attribute: 'current-stage' }) currentStage = 0;
  @property({ type: String }) status = '';

  private approvalsForStage(stageIndex: number): number {
    return this.approvals.filter((a) => a.stage_index === stageIndex && a.decision === 'approved').length;
  }

  private stageState(stageIndex: number): StageState {
    const { status, currentStage } = this;
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

  /** Fill ratio (0..1) of the connector leading into the given stage. */
  private connectorProgress(nextStageIndex: number): number {
    const nextState = this.stageState(nextStageIndex);
    if (nextState === 'complete') return 1;
    if (nextState === 'upcoming') return 0;
    const required = this.stages[nextStageIndex]?.required_approvals ?? 1;
    if (required <= 0) return 1;
    return Math.min(this.approvalsForStage(nextStageIndex) / required, 1);
  }

  private renderConnector(progress: number) {
    const partial = progress > 0 && progress < 1;
    return html`<div class=${classMap({ connector: true, filled: progress === 1 })}>
      ${partial ? html`<div class="connector-progress" style=${styleMap({ width: `${progress * 100}%` })}></div>` : null}
    </div>`;
  }

  private renderIndicator(state: StageState, index: number) {
    if (state === 'complete') return checkIcon;
    if (state === 'failed') return crossIcon;
    return html`<span class="number">${index + 1}</span>`;
  }

  render() {
    const startProgress = this.stages.length > 0 ? this.connectorProgress(0) : 1;
    return html`<div class="stage-bar" part="stage-bar">
      <div class="stage start-stage complete">
        <div class="indicator start-indicator">${startIcon}</div>
        <div class="label"><span class="name">Start</span></div>
      </div>
      ${this.renderConnector(startProgress)}
      ${this.stages.map((stage, i) => {
        const state = this.stageState(i);
        const isLast = i === this.stages.length - 1;
        return html`<div class=${classMap({ stage: true, [state]: true })} part="stage stage-${state}">
            <div class="indicator" part="stage-indicator">${this.renderIndicator(state, i)}</div>
            <div class="label">
              <span class="name">${stage.name || `Stage ${i + 1}`}</span>
              <span class="count">${this.approvalsForStage(i)}/${stage.required_approvals}</span>
            </div>
          </div>
          ${isLast ? null : this.renderConnector(this.connectorProgress(i + 1))}`;
      })}
    </div>`;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'quorum-stage-bar': QuorumStageBar;
  }
}
