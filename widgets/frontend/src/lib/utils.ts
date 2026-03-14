import type { RequestStatus } from './types';

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

export const STATUS_COLORS: Record<RequestStatus, { bg: string; text: string }> = {
  pending: { bg: '#fef3c7', text: '#92400e' },
  approved: { bg: '#d1fae5', text: '#065f46' },
  rejected: { bg: '#fee2e2', text: '#991b1b' },
  cancelled: { bg: '#f3f4f6', text: '#374151' },
  expired: { bg: '#ffedd5', text: '#9a3412' },
};

export const ACTION_COLORS: Record<string, string> = {
  created: '#6366f1',
  approved: '#10b981',
  rejected: '#ef4444',
  cancelled: '#6b7280',
  expired: '#f97316',
  stage_advanced: '#8b5cf6',
};
