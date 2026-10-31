/**
 * A trusted identity, sent as gateway headers to a server in `trust` mode.
 * Only safe when the client itself is the trusted gateway, or sits behind
 * one that strips these headers from the outside world.
 */
export interface Identity {
  userId: string;
  /** Falls back to the client's `tenant` option. */
  tenantId?: string;
  roles?: string[];
  permissions?: string[];
  /**
   * Named identity facts such as `branch_id`. Each is sent in the header the
   * server reads it from; see `HeaderNames.attributes`.
   */
  attributes?: Record<string, string>;
}

/** A JWT for a server in `verify` mode. Create one with `bearer()`. */
export interface BearerToken {
  token: string;
}

export type Credentials = Identity | BearerToken;

export function bearer(token: string): BearerToken {
  return { token };
}

/**
 * The headers a `trust` mode server reads identity from. They must match the
 * server's `auth.trust` configuration.
 */
export interface HeaderNames {
  userId: string;
  roles: string;
  permissions: string;
  tenantId: string;
  /** Maps an attribute name to its header, e.g. `{ branch_id: "X-User-Branch" }`. */
  attributes: Record<string, string>;
}

export const DEFAULT_HEADER_NAMES: Readonly<HeaderNames> = Object.freeze({
  userId: "X-User-ID",
  roles: "X-User-Roles",
  permissions: "X-User-Permissions",
  tenantId: "X-Tenant-ID",
  attributes: {},
});

export function mergeHeaderNames(base: HeaderNames, override?: Partial<HeaderNames>): HeaderNames {
  if (!override) return base;
  return {
    userId: override.userId ?? base.userId,
    roles: override.roles ?? base.roles,
    permissions: override.permissions ?? base.permissions,
    tenantId: override.tenantId ?? base.tenantId,
    attributes: { ...base.attributes, ...(override.attributes ?? {}) },
  };
}

export function credentialHeaders(
  creds: Credentials | undefined,
  names: HeaderNames,
  defaultTenant: string | undefined,
): Record<string, string> {
  const h: Record<string, string> = {};
  if (!creds) return h;
  if ("token" in creds) {
    h["Authorization"] = `Bearer ${creds.token}`;
    return h;
  }
  if (creds.userId) h[names.userId] = creds.userId;
  const tenant = creds.tenantId ?? defaultTenant;
  if (tenant) h[names.tenantId] = tenant;
  if (creds.roles?.length) h[names.roles] = creds.roles.join(",");
  if (creds.permissions?.length) h[names.permissions] = creds.permissions.join(",");
  for (const [attr, value] of Object.entries(creds.attributes ?? {})) {
    const header = names.attributes[attr];
    if (header && value) h[header] = value;
  }
  return h;
}
