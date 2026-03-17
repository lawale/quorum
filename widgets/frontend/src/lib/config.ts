import type { ClientConfig } from './api';

/** A value or a getter that returns the value at call time. */
export type MaybeGetter<T> = T | (() => T);

/** Global config where each field can be a static value or a getter function. */
export interface GlobalConfig {
  apiUrl?: MaybeGetter<string>;
  token?: MaybeGetter<string>;
  tenantId?: MaybeGetter<string>;
  authHeaders?: MaybeGetter<Record<string, string>>;
}

function resolve<T>(v: MaybeGetter<T> | undefined): T | undefined {
  return typeof v === 'function' ? (v as () => T)() : v;
}

let globalConfig: GlobalConfig = {};

/**
 * Set global default configuration for all Quorum widgets.
 * Values can be static or getter functions for session-dynamic data.
 * Per-widget HTML attributes override these defaults.
 *
 * Dispatches a `quorum:configured` event so already-mounted widgets
 * re-evaluate and load with the new config.
 *
 * @example
 * QuorumEmbed.configure({
 *   apiUrl: '/api',
 *   token: () => localStorage.getItem('auth_token') ?? '',
 *   tenantId: () => currentUser.tenantId,
 * });
 */
export function configure(config: GlobalConfig): void {
  globalConfig = { ...globalConfig, ...config };
  if (typeof document !== 'undefined') {
    document.dispatchEvent(new CustomEvent('quorum:configured'));
  }
}

/**
 * Returns true if an API URL is available from the global config.
 * Used by widget guards to determine if they can load.
 */
export function hasGlobalApiUrl(): boolean {
  return !!resolve(globalConfig.apiUrl);
}

/**
 * Resolves the current global configuration, calling any getter functions.
 */
export function getGlobalConfig(): Partial<ClientConfig> {
  const resolved: Partial<ClientConfig> = {};
  const apiUrl = resolve(globalConfig.apiUrl);
  const token = resolve(globalConfig.token);
  const tenantId = resolve(globalConfig.tenantId);
  const authHeaders = resolve(globalConfig.authHeaders);
  if (apiUrl) resolved.apiUrl = apiUrl;
  if (token) resolved.token = token;
  if (tenantId) resolved.tenantId = tenantId;
  if (authHeaders) resolved.authHeaders = authHeaders;
  return resolved;
}
