/**
 * Format an ISO timestamp to a human-readable date string.
 */
export function formatDate(iso: string): string {
  return new Date(iso).toLocaleString();
}

/**
 * Format a relative time (e.g., "2 hours ago").
 */
export function timeAgo(iso: string): string {
  const seconds = Math.floor((Date.now() - new Date(iso).getTime()) / 1000);
  if (seconds < 60) return 'just now';
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

/**
 * Capitalize the first letter of a string.
 */
export function capitalize(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

/**
 * Converts a snake_case key to a Title Case label.
 * e.g. "from_stage" → "From Stage"
 */
export function formatDetailsLabel(key: string): string {
  return key
    .split('_')
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(' ');
}

/**
 * Formats audit log details into a human-readable string.
 * Handles known action-specific templates (e.g. stage_advanced)
 * and falls back to "Label: value" pairs for unknown shapes.
 */
export function formatDetails(details: Record<string, unknown>, action?: string): string {
  if (!details || Object.keys(details).length === 0) return '';

  // Known template: stage advancement
  if (action === 'stage_advanced' && 'from_stage' in details && 'to_stage' in details) {
    return `Stage ${details.from_stage} → ${details.to_stage}`;
  }
  if (action === 'bypass_limit_exceeded' && 'measure' in details) {
    const what = details.measure === 'sum' ? `total ${details.path} of ${details.observed}` : `${details.observed} requests`;
    return `Bypass rule "${details.rule}" skipped: ${what} in ${details.window} exceeds limit ${details.limit}`;
  }
  if (action === 'bypassed' && 'rule' in details) {
    return details.rule ? `Auto-approved by rule "${details.rule}"` : 'Auto-approved by a bypass rule';
  }

  return Object.entries(details)
    .map(([key, value]) => `${formatDetailsLabel(key)}: ${value}`)
    .join('\n');
}

/**
 * Converts a snake_case slug to a human-readable Title Case label.
 * e.g. "wire_transfer" → "Wire Transfer", "access_request" → "Access Request"
 */
export function humanize(slug: string): string {
  return slug
    .split('_')
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(' ');
}

/**
 * Parse a Go duration string ("24h", "1h30m", "90s") into whole seconds.
 * A bare number is treated as nanoseconds, which is how older servers
 * serialised durations.
 */
function parseGoDurationSeconds(s: string): number {
  if (/^\d+$/.test(s)) return Math.floor(Number(s) / 1_000_000_000);
  const unitSeconds: Record<string, number> = { h: 3600, m: 60, s: 1, ms: 0.001, us: 0.000001, µs: 0.000001, ns: 1e-9 };
  let total = 0;
  let matched = false;
  for (const [, num, unit] of s.matchAll(/(\d+(?:\.\d+)?)(h|ms|m|s|us|µs|ns)/g)) {
    total += Number(num) * unitSeconds[unit];
    matched = true;
  }
  return matched ? Math.floor(total) : NaN;
}

/**
 * Converts a policy duration ("24h", "72h", "25h", or legacy nanoseconds) to a
 * compact human-readable string: "1d", "3d", "1d1h".
 */
export function formatDuration(value: number | string): string {
  let seconds = typeof value === 'string' ? parseGoDurationSeconds(value) : Math.floor(value / 1_000_000_000);
  if (!seconds || isNaN(seconds) || seconds <= 0) return '—';

  const parts: string[] = [];

  const year = 365 * 24 * 3600;
  const week = 7 * 24 * 3600;
  const day = 24 * 3600;
  const hour = 3600;
  const minute = 60;

  if (seconds >= year) {
    const y = Math.floor(seconds / year);
    parts.push(`${y}y`);
    seconds %= year;
  }
  if (seconds >= week) {
    const w = Math.floor(seconds / week);
    parts.push(`${w}w`);
    seconds %= week;
  }
  if (seconds >= day) {
    const d = Math.floor(seconds / day);
    parts.push(`${d}d`);
    seconds %= day;
  }
  if (seconds >= hour) {
    const h = Math.floor(seconds / hour);
    parts.push(`${h}h`);
    seconds %= hour;
  }
  if (seconds >= minute) {
    const m = Math.floor(seconds / minute);
    parts.push(`${m}m`);
    seconds %= minute;
  }
  if (seconds > 0 && parts.length === 0) {
    parts.push(`${seconds}s`);
  }

  return parts.join('') || '—';
}

/**
 * Copy text to clipboard and return true on success.
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
}

/**
 * Status color mapping for badges.
 */
export function statusColor(status: string): string {
  switch (status) {
    case 'pending': return 'bg-status-pending-bg text-status-pending-text';
    case 'approved': return 'bg-status-approved-bg text-status-approved-text';
    case 'rejected': return 'bg-status-rejected-bg text-status-rejected-text';
    case 'cancelled': return 'bg-status-cancelled-bg text-status-cancelled-text';
    case 'expired': return 'bg-status-expired-bg text-status-expired-text';
    default: return 'bg-surface-container text-on-surface-variant';
  }
}
