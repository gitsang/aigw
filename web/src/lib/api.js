import { browser } from '$app/environment';
import { goto } from '$app/navigation';

import { t } from './i18n/index.svelte.js';

/**
 * Locale-aware formatters live in `$lib/i18n/format.js`; they are re-exported
 * here so existing pages can keep importing them from `$lib/api`.
 */
export {
  formatNumber as fmtNumber,
  formatMoney as fmtCost,
  formatLatency as fmtLatency,
  formatTime as fmtTime,
  formatTimeOfDay as fmtTimeOfDay,
  formatPercent as pct
} from './i18n/format.js';

export const API_BASE = import.meta.env.PUBLIC_API_BASE || '';
const TOKEN_KEY = 'aigw_admin_token';

/** Resolve an API path against the configured base (empty = same origin). */
export function apiURL(path) {
  return `${API_BASE}${path}`;
}

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
    throw new Error(t('app.error.unauthorized'));
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
