import { writable } from 'svelte/store';
import type { Operator, Tenant } from './types';

// Current authenticated operator
export const currentUser = writable<Operator | null>(null);

// Tenant selection
export const selectedTenant = writable<string>('');
export const availableTenants = writable<Tenant[]>([]);

// Toast notifications
export interface Toast {
  id: number;
  message: string;
  type: 'success' | 'error' | 'info';
}

let toastId = 0;
export const toasts = writable<Toast[]>([]);

export function addToast(message: string, type: Toast['type'] = 'info', duration = 4000): void {
  const id = ++toastId;
  toasts.update((t) => [...t, { id, message, type }]);
  setTimeout(() => {
    toasts.update((t) => t.filter((x) => x.id !== id));
  }, duration);
}

// Loading state
export const loading = writable(false);
