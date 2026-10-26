import { browser } from '$app/environment';
import { goto } from '$app/navigation';

const API_BASE = import.meta.env.PUBLIC_API_BASE || '';
const TOKEN_KEY = 'aigw_admin_token';

export function getToken() {
  if (!browser) return '';
  return localStorage.getItem(TOKEN_KEY) || '';
}

export function setToken(token) {
  if (!browser) return;
  if (token) localStorage.setItem(TOKEN_KEY, token);
  else localStorage.removeItem(TOKEN_KEY);
}

export async function api(path, options = {}) {
  const token = getToken();
  const headers = {
    'Content-Type': 'application/json',
    ...(options.headers || {})
  };
  if (token) headers.Authorization = `Bearer ${token}`;
  const res = await fetch(`${API_BASE}${path}`, { ...options, headers });
  if (res.status === 401) {
    setToken('');
    if (browser && !location.pathname.startsWith('/login')) {
      goto(`/login?next=${encodeURIComponent(location.pathname + location.search)}`);
    }
    throw new Error('未授权，请重新输入 Admin Token');
  }
  if (res.status === 204) return null;
  const text = await res.text();
  let data = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = text;
  }
  if (!res.ok) {
    const message = data?.error?.message || data?.error || data?.message || `HTTP ${res.status}`;
    throw new Error(typeof message === 'string' ? message : JSON.stringify(message));
  }
  return data;
}

export const fmtNumber = (n) => new Intl.NumberFormat('zh-CN').format(Number(n || 0));
export const fmtCost = (n) => `$${Number(n || 0).toFixed(6)}`;
export const fmtLatency = (ms) => (ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${Math.round(ms || 0)}ms`);
export function fmtTime(value) {
  if (!value) return '—';
  return new Date(value).toLocaleString('zh-CN', { hour12: false });
}
export function pct(part, total) {
  if (!total) return '0%';
  return `${((Number(part || 0) / Number(total)) * 100).toFixed(1)}%`;
}
