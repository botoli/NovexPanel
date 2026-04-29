import { API_BASE } from './api';
import { tokenStore } from '../Store/TokenStore';

export async function apiRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers || {});
  if (!headers.has('Content-Type') && init.body) headers.set('Content-Type', 'application/json');
  const token = tokenStore.getToken();
  if (token) headers.set('Authorization', `Bearer ${token}`);

  const response = await fetch(`${API_BASE}${path}`, { ...init, headers });
  if (!response.ok) {
    let message = `HTTP ${response.status}`;
    try {
      const payload = await response.json() as { error?: string; };
      if (payload?.error) message = payload.error;
    } catch {
      // ignore
    }
    throw new Error(message);
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}
