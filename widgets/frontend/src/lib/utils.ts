import type { ResolvedDisplay, BypassLimitExceeded } from './types';

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleString();
}

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

export function capitalize(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

export const ACTION_COLORS: Record<string, string> = {
  created: '#4f46e5',
  approved: '#10b981',
  rejected: '#ef4444',
  cancelled: '#6b7280',
  expired: '#f97316',
  stage_advanced: '#8b5cf6',
  bypassed: '#10b981',
  bypass_limit_exceeded: '#f59e0b',
};

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
 * Extracts the resolved display data from request metadata, if present.
 * Returns null if metadata is missing or has no display key.
 */
export function getBypassLimitExceeded(metadata?: Record<string, unknown>): BypassLimitExceeded | null {
  const hit = metadata?.bypass_limit_exceeded as BypassLimitExceeded | undefined;
  return hit && typeof hit.measure === 'string' ? hit : null;
}

export function getDisplay(metadata?: Record<string, unknown>): ResolvedDisplay | null {
  if (!metadata || !metadata.display) return null;

  const display = metadata.display as Record<string, unknown>;
  if (!display.fields || !Array.isArray(display.fields)) return null;

  return display as unknown as ResolvedDisplay;
}
