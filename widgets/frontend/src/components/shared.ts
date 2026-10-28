import { LitElement, css } from 'lit';
import { property } from 'lit/decorators.js';
import { createClient, type QuorumClient, type SSEConnection } from '../lib/api';
import { getGlobalConfig, hasGlobalApiUrl } from '../lib/config';

/** Names of the reactive properties that affect which API/credentials a widget uses. */
export const CONNECTION_KEYS = ['apiUrl', 'token', 'tenantId', 'authHeaders'] as const;

/**
 * Base class for every Quorum widget. Provides the shared connection
 * attributes (`api-url`, `token`, `tenant-id`, `auth-headers`), a client
 * factory that overlays them on the global config, event dispatch, and a
 * hook that fires when `configure()` is called after the element mounted.
 */
export abstract class QuorumElement extends LitElement {
  @property({ type: String, attribute: 'api-url' }) apiUrl = '';
  @property({ type: String }) token = '';
  @property({ type: String, attribute: 'tenant-id' }) tenantId = '';
  @property({ type: String, attribute: 'auth-headers' }) authHeaders = '';

  connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener('quorum:configured', this.handleConfigured);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('quorum:configured', this.handleConfigured);
  }

  /** Called when `configure()` runs and this element has no `api-url` of its own. */
  protected onGlobalConfigured(): void {}

  private handleConfigured = () => {
    if (!this.apiUrl && hasGlobalApiUrl()) this.onGlobalConfigured();
  };

  /** True when the element can reach an API, either via its own attribute or the global config. */
  protected canConnect(): boolean {
    return !!this.apiUrl || hasGlobalApiUrl();
  }

  /** True when any connection-related property is in the changed set. */
  protected connectionChanged(changed: Map<PropertyKey, unknown>): boolean {
    return CONNECTION_KEYS.some((k) => changed.has(k));
  }

  /** Builds a client from global config overlaid with this element's attributes. */
  protected client(): QuorumClient {
    let authHeaders: Record<string, string> | undefined;
    if (this.authHeaders) {
      try {
        authHeaders = JSON.parse(this.authHeaders);
      } catch {
        /* ignore malformed JSON */
      }
    }
    const global = getGlobalConfig();
    return createClient({
      ...global,
      apiUrl: this.apiUrl || global.apiUrl || '',
      ...(this.token ? { token: this.token } : {}),
      ...(this.tenantId ? { tenantId: this.tenantId } : {}),
      ...(authHeaders ? { authHeaders } : {}),
    });
  }

  /** Dispatches a bubbling, composed custom event from the host element. */
  protected emit(name: string, detail: unknown): void {
    this.dispatchEvent(new CustomEvent(name, { detail, bubbles: true, composed: true }));
  }
}

/** Attribute converter: present (even bare) means true, unless explicitly "false". */
export const presenceConverter = {
  fromAttribute: (value: string | null): boolean => value !== null && value !== 'false',
  toAttribute: (value: boolean): string | null => (value ? '' : null),
};

/** Attribute converter: true unless explicitly "false". */
export const defaultTrueConverter = {
  fromAttribute: (value: string | null): boolean => value !== 'false',
  toAttribute: (value: boolean): string => String(value),
};

/**
 * SSE-first live updates with polling fallback. SSE and polling are mutually
 * exclusive: while SSE is connected there is no polling; if SSE fails or
 * disconnects, polling takes over.
 */
export class LiveUpdater {
  private pollTimer: ReturnType<typeof setInterval> | undefined;
  private sse: SSEConnection | null = null;

  constructor(private readonly onTick: () => void) {}

  start(opts: { sse: boolean; client: QuorumClient; requestId: string; pollInterval: number }): void {
    this.stop();
    if (opts.sse && opts.requestId) {
      try {
        this.sse = opts.client.connectSSE(opts.requestId, this.onTick, () => {
          this.sse = null;
          this.startPolling(opts.pollInterval);
        });
        return;
      } catch {
        /* fall through to polling */
      }
    }
    this.startPolling(opts.pollInterval);
  }

  stop(): void {
    if (this.sse) {
      this.sse.close();
      this.sse = null;
    }
    if (this.pollTimer) {
      clearInterval(this.pollTimer);
      this.pollTimer = undefined;
    }
  }

  private startPolling(interval: number): void {
    if (this.pollTimer) clearInterval(this.pollTimer);
    if (interval > 0) this.pollTimer = setInterval(this.onTick, interval);
  }
}

/**
 * Theme tokens. Every widget reads these CSS custom properties with a built-in
 * fallback, so a host page can restyle all widgets by setting them on any
 * ancestor (custom properties inherit through shadow roots):
 *
 *   quorum-approval-panel { --quorum-accent: #0ea5e9; }
 */
export const tokens = css`
  :host {
    --_font: var(--quorum-font, 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif);
    --_bg: var(--quorum-bg, #fff);
    --_fg: var(--quorum-fg, #191c1d);
    --_muted: var(--quorum-muted, #464555);
    --_surface: var(--quorum-surface, #f3f4f5);
    --_border: var(--quorum-border, rgba(199, 196, 216, 0.4));
    --_divider: var(--quorum-divider, rgba(199, 196, 216, 0.15));
    --_track: var(--quorum-track, #edeeef);
    --_accent: var(--quorum-accent, #4f46e5);
    --_success: var(--quorum-success, #10b981);
    --_success-hover: var(--quorum-success-hover, #059669);
    --_danger: var(--quorum-danger, #ef4444);
    --_danger-hover: var(--quorum-danger-hover, #dc2626);
    --_radius: var(--quorum-radius, 8px);
    --_radius-sm: var(--quorum-radius-sm, 6px);
    --_shadow: var(--quorum-shadow, 0 4px 24px -4px rgba(25, 28, 29, 0.06));
    --_mono: var(--quorum-font-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  }
`;
