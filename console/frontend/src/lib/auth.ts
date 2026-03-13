import type { Operator } from './types';

const TOKEN_KEY = 'quorum_jwt';
const OPERATOR_KEY = 'quorum_operator';

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY);
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
  return !!getToken();
}
