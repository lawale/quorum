import { css, html, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { classMap } from 'lit/directives/class-map.js';
import { ApiError } from '../lib/api';
import type { Request, Policy, AuditLog } from '../lib/types';
import { formatDate, timeAgo, getDisplay } from '../lib/utils';
import { QuorumElement, LiveUpdater, defaultTrueConverter, presenceConverter, tokens } from './shared';
import './internal/status-badge';
import './internal/stage-bar';
import './internal/audit-timeline';

type Tab = 'details' | 'payload' | 'audit';

/**
 * `<quorum-approval-panel>` — full detail view for a single request with
 * stage progress, payload, audit trail, and approve/reject actions.
 *
 * Events (bubbling, composed):
 * - `quorum:approved` / `quorum:rejected` — `{ requestId, request }`
 * - `quorum:error` — `{ action, message, status }`
 */
@customElement('quorum-approval-panel')
export class QuorumApprovalPanel extends QuorumElement {
  static styles = [tokens, css`
    :host { display: block; }
    .panel {
      font-family: var(--_font);
      box-shadow: var(--_shadow);
      border-radius: var(--_radius);
      padding: 16px;
      background: var(--_bg);
      color: var(--_fg);
      max-width: var(--quorum-panel-max-width, 600px);
    }
    .loading { font-size: 13px; color: var(--_muted); padding: 16px 0; text-align: center; }
    .error-box {
      font-size: 13px;
      color: var(--quorum-status-rejected-fg, #991b1b);
      padding: 8px 12px;
      background: var(--quorum-status-rejected-bg, #fee2e2);
      border-radius: var(--_radius-sm);
    }
    .header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
    .header-left { display: flex; align-items: center; gap: 8px; }
    .type { font-size: 13px; color: var(--_muted); }
    .id { font-size: 11px; color: var(--_muted); font-family: var(--_mono); }
    .meta { display: flex; gap: 16px; font-size: 12px; color: var(--_muted); margin-bottom: 16px; flex-wrap: wrap; }
    .meta strong { color: var(--_fg); }
    .section { margin-bottom: 16px; }
    .tabs { display: flex; border-bottom: 1px solid var(--_divider); margin-bottom: 12px; }
    .tab {
      padding: 8px 16px;
      font-size: 13px;
      font-weight: 500;
      background: none;
      border: none;
      border-bottom: 2px solid transparent;
      color: var(--_muted);
      cursor: pointer;
      font-family: inherit;
      transition: all 0.15s;
    }
    .tab:hover { color: var(--_fg); }
    .tab.active { color: var(--_accent); border-bottom-color: var(--_accent); }
    .tab-content { min-height: 80px; }
    .detail-grid { display: flex; flex-direction: column; gap: 8px; }
    .detail-row { display: flex; align-items: center; gap: 8px; }
    .detail-label { font-size: 12px; color: var(--_muted); min-width: 100px; }
    .detail-value { font-size: 13px; color: var(--_fg); }
    .mono { font-family: var(--_mono); font-size: 12px; }
    .payload {
      font-size: 12px;
      font-family: var(--_mono);
      background: var(--_surface);
      border: 1px solid var(--_divider);
      border-radius: var(--_radius-sm);
      padding: 12px;
      overflow-x: auto;
      white-space: pre-wrap;
      word-break: break-word;
      margin: 0;
      color: var(--_fg);
    }
    .actions { margin-top: 16px; padding-top: 16px; border-top: 1px solid var(--_divider); }
    .action-error { font-size: 12px; color: var(--_danger); margin-bottom: 8px; }
    .comment-input {
      width: 100%;
      padding: 8px 10px;
      font-size: 13px;
      border: 1px solid var(--_border);
      border-radius: var(--_radius-sm);
      resize: vertical;
      font-family: inherit;
      box-sizing: border-box;
      margin-bottom: 8px;
      background: var(--_bg);
      color: var(--_fg);
    }
    .comment-input:focus {
      outline: none;
      border-color: var(--_accent);
      box-shadow: 0 0 0 2px color-mix(in srgb, var(--_accent) 15%, transparent);
    }
    .action-buttons { display: flex; gap: 8px; }
    .btn {
      flex: 1;
      padding: 8px 16px;
      font-size: 13px;
      font-weight: 500;
      border: none;
      border-radius: var(--_radius-sm);
      cursor: pointer;
      font-family: inherit;
      color: #fff;
      transition: background 0.15s;
    }
    .btn:disabled { opacity: 0.6; cursor: not-allowed; }
    .btn-approve { background: var(--_success); }
    .btn-approve:hover:not(:disabled) { background: var(--_success-hover); }
    .btn-reject { background: var(--_danger); }
    .btn-reject:hover:not(:disabled) { background: var(--_danger-hover); }
    .display-title { font-size: 15px; font-weight: 600; color: var(--_fg); margin: 0 0 12px 0; }
    .display-fields { display: flex; flex-direction: column; gap: 6px; }
    .display-row { display: flex; align-items: baseline; gap: 8px; }
    .display-row-sm { padding-left: 8px; }
    .display-label { font-size: 12px; color: var(--_muted); min-width: 120px; flex-shrink: 0; }
    .display-value { font-size: 13px; color: var(--_fg); }
    .display-items { margin-top: 12px; display: flex; flex-direction: column; gap: 8px; }
    .display-item {
      padding: 8px 12px;
      background: var(--_surface);
      border: 1px solid var(--_divider);
      border-radius: var(--_radius-sm);
    }
    .display-item-title { font-size: 13px; font-weight: 600; color: var(--_fg); margin-bottom: 4px; }
    .raw-toggle {
      margin-top: 10px;
      padding: 4px 10px;
      font-size: 11px;
      background: none;
      border: 1px solid var(--_border);
      border-radius: 4px;
      color: var(--_muted);
      cursor: pointer;
      font-family: inherit;
    }
    .raw-toggle:hover { background: var(--_surface); color: var(--_fg); }
  `];

  @property({ type: String, attribute: 'request-id' }) requestId = '';
  @property({ type: Number, attribute: 'poll-interval' }) pollInterval = 30000;
  @property({ attribute: 'sse', converter: defaultTrueConverter }) sse = true;
  @property({ attribute: 'suppress-errors', converter: presenceConverter }) suppressErrors = false;

  @state() private req: Request | null = null;
  @state() private policy: Policy | null = null;
  @state() private auditLogs: AuditLog[] = [];
  @state() private error: string | null = null;
  @state() private loading = true;
  @state() private actionLoading = false;
  @state() private comment = '';
  @state() private activeTab: Tab = 'details';
  @state() private showRawPayload = false;

  private live = new LiveUpdater(() => this.load());
  private wasPending = false;

  disconnectedCallback(): void {
    super.disconnectedCallback();
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

  private get isPending(): boolean {
    return this.req?.status === 'pending';
  }

  /** Start or stop SSE/polling to match the request's pending state. */
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
      const [req, logs] = await Promise.all([client.getRequest(this.requestId), client.getAudit(this.requestId)]);
      this.req = req;
      this.auditLogs = logs;
      this.policy = await client.getPolicyByType(req.type);
      if (this.isPending !== this.wasPending) {
        this.wasPending = this.isPending;
        this.syncLive();
      }
    } catch (e) {
      this.error = e instanceof ApiError ? e.message : 'Failed to load';
      this.emit('quorum:error', { action: 'load', message: this.error, status: e instanceof ApiError ? e.status : 0 });
    } finally {
      this.loading = false;
    }
  }

  private async handleAction(action: 'approve' | 'reject'): Promise<void> {
    if (!this.req) return;
    this.actionLoading = true;
    try {
      const client = this.client();
      const updated = await (action === 'approve'
        ? client.approve(this.req.id, this.comment || undefined)
        : client.reject(this.req.id, this.comment || undefined));
      this.req = updated;
      this.comment = '';
      this.emit(action === 'approve' ? 'quorum:approved' : 'quorum:rejected', { requestId: updated.id, request: updated });
      await this.load();
    } catch (e) {
      const message = e instanceof ApiError ? e.message : 'Action failed';
      this.error = message;
      this.emit('quorum:error', { action, message, status: e instanceof ApiError ? e.status : 0 });
    } finally {
      this.actionLoading = false;
    }
  }

  render() {
    const { req, error, suppressErrors } = this;
    if (this.loading && !req) return html`<div class="panel" part="panel"><div class="loading">Loading request...</div></div>`;
    if (error && !req && !suppressErrors) return html`<div class="panel" part="panel"><div class="error-box" part="error">${error}</div></div>`;
    if (!req) return html`<div class="panel" part="panel"><div class="error-box" part="error">Request not found</div></div>`;

    return html`<div class="panel" part="panel">
      <div class="header" part="header">
        <div class="header-left">
          <quorum-status-badge status=${req.status} exportparts="badge"></quorum-status-badge>
          <span class="type">${this.policy?.name ?? req.type}</span>
        </div>
        <span class="id" title=${req.id}>${req.id.slice(0, 8)}...</span>
      </div>

      <div class="meta">
        <span>Maker: <strong>${req.maker_id}</strong></span>
        <span>Created: ${timeAgo(req.created_at)}</span>
        ${req.expires_at ? html`<span>Expires: ${formatDate(req.expires_at)}</span>` : nothing}
      </div>

      ${this.policy
        ? html`<div class="section">
            <quorum-stage-bar
              exportparts="stage-bar, stage, stage-indicator"
              .stages=${this.policy.stages}
              .approvals=${req.approvals ?? []}
              current-stage=${req.current_stage}
              status=${req.status}
            ></quorum-stage-bar>
          </div>`
        : nothing}

      <div class="tabs" part="tabs">${this.renderTab('details', 'Details')}${this.renderTab('payload', 'Payload')}${this.renderTab('audit', 'Audit Trail')}</div>
      <div class="tab-content" part="tab-content">${this.renderTabContent(req)}</div>

      ${req.viewer_can_act ? this.renderActions() : nothing}
    </div>`;
  }

  private renderTab(tab: Tab, label: string) {
    return html`<button class=${classMap({ tab: true, active: this.activeTab === tab })} part="tab" @click=${() => (this.activeTab = tab)}>
      ${label}
    </button>`;
  }

  private renderTabContent(req: Request) {
    switch (this.activeTab) {
      case 'details':
        return html`<div class="detail-grid">
          <div class="detail-row">
            <span class="detail-label">Status</span>
            <quorum-status-badge status=${req.status} exportparts="badge"></quorum-status-badge>
          </div>
          <div class="detail-row">
            <span class="detail-label">Type</span>
            <span class="detail-value">${req.type}</span>
          </div>
          <div class="detail-row">
            <span class="detail-label">Stage</span>
            <span class="detail-value">${req.current_stage + 1} of ${this.policy?.stages.length ?? '?'}</span>
          </div>
          ${req.idempotency_key
            ? html`<div class="detail-row">
                <span class="detail-label">Idempotency Key</span>
                <span class="detail-value mono">${req.idempotency_key}</span>
              </div>`
            : nothing}
        </div>`;
      case 'payload':
        return this.renderPayload(req);
      case 'audit':
        return html`<quorum-audit-timeline .logs=${this.auditLogs} exportparts="timeline"></quorum-audit-timeline>`;
    }
  }

  private renderPayload(req: Request) {
    const display = getDisplay(req.metadata);
    if (display && !this.showRawPayload) {
      return html`<div class="display-view">
        ${display.title ? html`<h3 class="display-title">${display.title}</h3>` : nothing}
        <div class="display-fields">
          ${display.fields.map(
            (f) => html`<div class="display-row">
              <span class="display-label">${f.label}</span>
              <span class="display-value">${f.value}</span>
            </div>`,
          )}
        </div>
        ${display.items && display.items.length > 0
          ? html`<div class="display-items">
              ${display.items.map(
                (item) => html`<div class="display-item">
                  <div class="display-item-title">${item.title}</div>
                  ${item.fields.map(
                    (f) => html`<div class="display-row display-row-sm">
                      <span class="display-label">${f.label}</span>
                      <span class="display-value">${f.value}</span>
                    </div>`,
                  )}
                </div>`,
              )}
            </div>`
          : nothing}
        <button class="raw-toggle" @click=${() => (this.showRawPayload = true)}>Show raw payload</button>
      </div>`;
    }
    return html`<pre class="payload" part="payload">${JSON.stringify(req.payload, null, 2)}</pre>
      ${display ? html`<button class="raw-toggle" @click=${() => (this.showRawPayload = false)}>Show formatted view</button>` : nothing}`;
  }

  private renderActions() {
    const busy = this.actionLoading;
    return html`<div class="actions" part="actions">
      ${this.error && !this.suppressErrors ? html`<div class="action-error">${this.error}</div>` : nothing}
      <textarea
        class="comment-input"
        part="comment"
        placeholder="Add a comment (optional)"
        rows="2"
        .value=${this.comment}
        @input=${(e: Event) => (this.comment = (e.target as HTMLTextAreaElement).value)}
      ></textarea>
      <div class="action-buttons">
        <button class="btn btn-approve" part="button approve-button" ?disabled=${busy} @click=${() => this.handleAction('approve')}>
          ${busy ? 'Processing...' : 'Approve'}
        </button>
        <button class="btn btn-reject" part="button reject-button" ?disabled=${busy} @click=${() => this.handleAction('reject')}>
          ${busy ? 'Processing...' : 'Reject'}
        </button>
      </div>
    </div>`;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'quorum-approval-panel': QuorumApprovalPanel;
  }
}
