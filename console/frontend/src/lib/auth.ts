import type { Operator } from './types';

const OPERATOR_KEY = 'quorum_operator';

export function clearSession(): void {
  localStorage.removeItem(OPERATOR_KEY);
}

export function getStoredOperator(): Operator | null {
  const raw = localStorage.getItem(OPERATOR_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as Operator;
  } catch {
    return null;
  }
}

export function setStoredOperator(op: Operator): void {
  localStorage.setItem(OPERATOR_KEY, JSON.stringify(op));
}

export function isAuthenticated(): boolean {
  return !!getStoredOperator();
}
