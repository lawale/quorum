import { css, html, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { ApiError } from '../lib/api';
import type { Request } from '../lib/types';
import { timeAgo, getDisplay } from '../lib/utils';
import { QuorumElement, tokens } from './shared';
import './internal/status-badge';

/**
 * `<quorum-request-list>` — paginated, filterable list of approval requests.
 *
 * Events (bubbling, composed):
 * - `quorum:select` — `{ requestId, request }` when a row is clicked
 * - `quorum:error` — `{ message, status }`
 */
@customElement('quorum-request-list')
export class QuorumRequestList extends QuorumElement {
  static styles = [tokens, css`
    :host { display: block; }
    .list-container {
      font-family: var(--_font);
      box-shadow: var(--_shadow);
      border-radius: var(--_radius);
      background: var(--_bg);
      color: var(--_fg);
      overflow: hidden;
    }
    .loading, .error-box, .empty { font-size: 13px; padding: 24px 16px; text-align: center; }
    .loading { color: var(--_muted); }
    .error-box { color: var(--_danger); }
    .empty { color: var(--_muted); }
    .table { width: 100%; border-collapse: collapse; font-size: 13px; }
    th {
      text-align: left;
      padding: 8px 12px;
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--_muted);
      background: var(--_surface);
      border-bottom: 1px solid var(--_divider);
    }
    td { padding: 10px 12px; border-bottom: 1px solid var(--_surface); }
    .row { cursor: pointer; transition: background 0.1s; }
    .row:hover { background: var(--_surface); }
    .type-cell { font-family: var(--_mono); font-size: 12px; }
    .maker-cell { color: var(--_fg); }
    .stage-cell { color: var(--_muted); text-align: center; }
    .time-cell { color: var(--_muted); white-space: nowrap; font-size: 12px; }
    .pagination {
      display: flex;
      justify-content: center;
      align-items: center;
      gap: 12px;
      padding: 10px 12px;
      border-top: 1px solid var(--_divider);
    }
    .page-btn {
      padding: 4px 12px;
      font-size: 12px;
      border: 1px solid var(--_border);
      border-radius: 4px;
      background: var(--_bg);
      color: var(--_fg);
      cursor: pointer;
      font-family: inherit;
    }
    .page-btn:hover:not(:disabled) { background: var(--_surface); }
    .page-btn:disabled { opacity: 0.4; cursor: not-allowed; }
    .page-info { font-size: 12px; color: var(--_muted); }
  `];

  @property({ type: String }) status = '';
  @property({ type: String }) type = '';
  @property({ type: Number, attribute: 'page-size' }) pageSize = 10;

  @state() private requests: Request[] = [];
  @state() private total = 0;
  @state() private page = 1;
  @state() private error: string | null = null;
  @state() private loading = true;

  private get totalPages(): number {
    const size = this.pageSize > 0 ? this.pageSize : 10;
    return Math.ceil(this.total / size);
  }

  updated(changed: Map<PropertyKey, unknown>): void {
    const filtersChanged =
      this.connectionChanged(changed) || changed.has('status') || changed.has('type') || changed.has('pageSize');
    if (filtersChanged && this.page !== 1) {
      // Resetting the page triggers another update cycle, which performs the load.
      this.page = 1;
      return;
    }
    if ((filtersChanged || changed.has('page')) && this.canConnect()) {
      this.load();
    }
  }

  protected onGlobalConfigured(): void {
    if (this.page !== 1) {
      this.page = 1;
    } else {
      this.load();
    }
  }

  private async load(): Promise<void> {
    if (!this.canConnect()) return;
    this.loading = true;
    this.error = null;
    try {
      const res = await this.client().listRequests({
        status: this.status || undefined,
        type: this.type || undefined,
        page: this.page,
        per_page: this.pageSize > 0 ? this.pageSize : 10,
      });
      this.requests = res.data;
      this.total = res.total;
    } catch (e) {
      this.error = e instanceof ApiError ? e.message : 'Failed to load';
      this.emit('quorum:error', { message: this.error, status: e instanceof ApiError ? e.status : 0 });
    } finally {
      this.loading = false;
    }
  }

  private select(req: Request): void {
    this.emit('quorum:select', { requestId: req.id, request: req });
  }

  render() {
    return html`<div class="list-container" part="list">${this.renderBody()}</div>`;
  }

  private renderBody() {
    const { requests } = this;
    if (this.loading && requests.length === 0) return html`<div class="loading">Loading requests...</div>`;
    if (this.error && requests.length === 0) return html`<div class="error-box" part="error">${this.error}</div>`;
    if (requests.length === 0) return html`<div class="empty">No requests found</div>`;

    return html`<table class="table" part="table">
        <thead>
          <tr><th>Type</th><th>Status</th><th>Maker</th><th>Stage</th><th>Created</th></tr>
        </thead>
        <tbody>
          ${requests.map(
            (req) => html`<tr class="row" part="row" @click=${() => this.select(req)}>
              <td class="type-cell">${getDisplay(req.metadata)?.title ?? req.type}</td>
              <td><quorum-status-badge status=${req.status} exportparts="badge"></quorum-status-badge></td>
              <td class="maker-cell">${req.maker_id}</td>
              <td class="stage-cell">${req.current_stage + 1}</td>
              <td class="time-cell">${timeAgo(req.created_at)}</td>
            </tr>`,
          )}
        </tbody>
      </table>
      ${this.totalPages > 1
        ? html`<div class="pagination" part="pagination">
            <button class="page-btn" ?disabled=${this.page <= 1} @click=${() => this.page--}>Prev</button>
            <span class="page-info">${this.page} / ${this.totalPages}</span>
            <button class="page-btn" ?disabled=${this.page >= this.totalPages} @click=${() => this.page++}>Next</button>
          </div>`
        : nothing}`;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'quorum-request-list': QuorumRequestList;
  }
}
