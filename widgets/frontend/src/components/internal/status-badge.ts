import { LitElement, css, html } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { classMap } from 'lit/directives/class-map.js';
import { capitalize } from '../../lib/utils';
import type { RequestStatus } from '../../lib/types';

@customElement('quorum-status-badge')
export class QuorumStatusBadge extends LitElement {
  static styles = css`
    :host { display: inline-block; }
    .badge {
      display: inline-block;
      padding: 2px 8px;
      border-radius: 9999px;
      font-size: 12px;
      font-weight: 500;
      line-height: 1.4;
      white-space: nowrap;
      background: var(--quorum-status-bg, #f3f4f6);
      color: var(--quorum-status-fg, #374151);
    }
    .pending { --quorum-status-bg: var(--quorum-status-pending-bg, #fef3c7); --quorum-status-fg: var(--quorum-status-pending-fg, #92400e); }
    .approved { --quorum-status-bg: var(--quorum-status-approved-bg, #d1fae5); --quorum-status-fg: var(--quorum-status-approved-fg, #065f46); }
    .rejected { --quorum-status-bg: var(--quorum-status-rejected-bg, #fee2e2); --quorum-status-fg: var(--quorum-status-rejected-fg, #991b1b); }
    .cancelled { --quorum-status-bg: var(--quorum-status-cancelled-bg, #f3f4f6); --quorum-status-fg: var(--quorum-status-cancelled-fg, #374151); }
    .expired { --quorum-status-bg: var(--quorum-status-expired-bg, #ffedd5); --quorum-status-fg: var(--quorum-status-expired-fg, #9a3412); }
  `;

  @property({ type: String }) status: RequestStatus = 'pending';

  render() {
    const status = this.status || 'pending';
    return html`<span class=${classMap({ badge: true, [status]: true })} part="badge badge-${status}">
      ${capitalize(status)}
    </span>`;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'quorum-status-badge': QuorumStatusBadge;
  }
}
