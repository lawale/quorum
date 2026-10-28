import { css, html } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { ApiError } from '../lib/api';
import type { Request, Policy } from '../lib/types';
import { QuorumElement, LiveUpdater, defaultTrueConverter, tokens } from './shared';
import './internal/stage-bar';
import './internal/status-badge';

/**
 * `<quorum-stage-progress>` — compact stage indicator for a single request.
 * Refreshes on SSE events (with polling fallback) and whenever a
 * `<quorum-approval-panel>` on the same page approves or rejects it.
 */
@customElement('quorum-stage-progress')
export class QuorumStageProgress extends QuorumElement {
  static styles = [tokens, css`
    :host { display: block; }
    .container { font-family: var(--_font); color: var(--_fg); padding: 12px; }
    .header { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; }
    .type { font-size: 13px; color: var(--_muted); }
    .loading, .error { font-size: 13px; padding: 8px 0; }
    .loading { color: var(--_muted); }
    .error { color: var(--_danger); }
  `];

  @property({ type: String, attribute: 'request-id' }) requestId = '';
  @property({ type: Number, attribute: 'poll-interval' }) pollInterval = 30000;
  @property({ attribute: 'sse', converter: defaultTrueConverter }) sse = true;

  @state() private req: Request | null = null;
  @state() private policy: Policy | null = null;
  @state() private error: string | null = null;
  @state() private loading = true;

  private live = new LiveUpdater(() => this.load());
  private wasPending = false;

  connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener('quorum:approved', this.onDecision);
    document.addEventListener('quorum:rejected', this.onDecision);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('quorum:approved', this.onDecision);
    document.removeEventListener('quorum:rejected', this.onDecision);
    this.live.stop();
  }

  updated(changed: Map<PropertyKey, unknown>): void {
    const sourceChanged = changed.has('requestId') || this.connectionChanged(changed);
    if (sourceChanged && this.requestId && this.canConnect()) {
      this.load();
    }
    if (sourceChanged || changed.has('sse') || changed.has('pollInterval')) {
      this.syncLive();
    }
  }

  protected onGlobalConfigured(): void {
    if (this.requestId) this.load();
  }

  private onDecision = (e: Event) => {
    const detail = (e as CustomEvent).detail;
    if (detail?.requestId === this.requestId) this.load();
  };

  private get isPending(): boolean {
    return this.req?.status === 'pending';
  }

  private syncLive(): void {
    if (!this.isPending || !this.requestId || !this.canConnect()) {
      this.live.stop();
      return;
    }
    this.live.start({
      sse: this.sse,
      client: this.client(),
      requestId: this.requestId,
      pollInterval: this.pollInterval,
    });
  }

  private async load(): Promise<void> {
    if (!this.requestId || !this.canConnect()) return;
    this.loading = true;
    this.error = null;
    try {
      const client = this.client();
      const req = await client.getRequest(this.requestId);
      this.req = req;
      this.policy = await client.getPolicyByType(req.type);
      if (this.isPending !== this.wasPending) {
        this.wasPending = this.isPending;
        this.syncLive();
      }
    } catch (e) {
      this.error = e instanceof ApiError ? e.message : 'Failed to load';
    } finally {
      this.loading = false;
    }
  }

  render() {
    return html`<div class="container" part="progress">${this.renderBody()}</div>`;
  }

  private renderBody() {
    if (this.loading) return html`<div class="loading">Loading...</div>`;
    if (this.error) return html`<div class="error">${this.error}</div>`;
    const { req, policy } = this;
    if (!req || !policy) return html`<div class="error">Request or policy not found</div>`;
    return html`<div class="header" part="header">
        <quorum-status-badge status=${req.status} exportparts="badge"></quorum-status-badge>
        <span class="type">${policy.name}</span>
      </div>
      <quorum-stage-bar
        exportparts="stage-bar, stage, stage-indicator"
        .stages=${policy.stages}
        .approvals=${req.approvals ?? []}
        current-stage=${req.current_stage}
        status=${req.status}
      ></quorum-stage-bar>`;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'quorum-stage-progress': QuorumStageProgress;
  }
}
