import { LitElement, css, html } from 'lit';
import { tokens } from '../shared';
import { customElement, property } from 'lit/decorators.js';
import { styleMap } from 'lit/directives/style-map.js';
import type { AuditLog } from '../../lib/types';
import { timeAgo, capitalize, formatDetails, ACTION_COLORS } from '../../lib/utils';

@customElement('quorum-audit-timeline')
export class QuorumAuditTimeline extends LitElement {
  static styles = [tokens, css`
    :host { display: block; }
    .timeline { display: flex; flex-direction: column; gap: 0; }
    .entry {
      display: flex;
      gap: 12px;
      padding: 8px 0 8px 16px;
      border-left: 2px solid var(--_track);
      margin-left: 6px;
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
    .content { flex: 1; min-width: 0; }
    .header { display: flex; justify-content: space-between; align-items: baseline; gap: 8px; }
    .action { font-size: 13px; font-weight: 600; }
    .time { font-size: 11px; color: var(--_muted); white-space: nowrap; }
    .actor { font-size: 12px; color: var(--_muted); margin-top: 1px; }
    .details {
      font-size: 11px;
      color: var(--_muted);
      margin-top: 4px;
      font-family: var(--_mono);
      white-space: pre-line;
      word-break: break-all;
    }
    .empty { font-size: 13px; color: var(--_muted); padding: 8px 0; }
  `];

  @property({ attribute: false }) logs: AuditLog[] = [];

  render() {
    if (this.logs.length === 0) {
      return html`<div class="timeline" part="timeline"><div class="empty">No audit entries</div></div>`;
    }
    return html`<div class="timeline" part="timeline">
      ${this.logs.map((log) => {
        // Host pages can override per action, e.g. --quorum-action-approved.
        const color = `var(--quorum-action-${log.action}, ${ACTION_COLORS[log.action] ?? '#6b7280'})`;
        const details = log.details && Object.keys(log.details).length > 0 ? log.details : null;
        return html`<div class="entry">
          <div class="dot" style=${styleMap({ background: color })}></div>
          <div class="content">
            <div class="header">
              <span class="action" style=${styleMap({ color })}>${capitalize(log.action.replace(/_/g, ' '))}</span>
              <span class="time">${timeAgo(log.created_at)}</span>
            </div>
            <div class="actor">by ${log.actor_id}</div>
            ${details ? html`<div class="details">${formatDetails(details, log.action)}</div>` : null}
          </div>
        </div>`;
      })}
    </div>`;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'quorum-audit-timeline': QuorumAuditTimeline;
  }
}
