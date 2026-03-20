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

  return Object.entries(details)
    .map(([key, value]) => `${formatDetailsLabel(key)}: ${value}`)
    .join('\n');
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
