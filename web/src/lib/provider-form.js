import { api } from '$lib/api';

export const API_FORMATS = [
  { value: 'chat.completions', label: 'Chat Completions', defaultPath: '/v1/chat/completions' },
  { value: 'responses', label: 'Responses', defaultPath: '/v1/responses' },
  { value: 'messages', label: 'Messages', defaultPath: '/v1/messages' }
];

export function apiFormatLabel(value) {
  return API_FORMATS.find((p) => p.value === value)?.label ?? value;
}

export function apiFormatDefaultPath(value) {
  return API_FORMATS.find((p) => p.value === value)?.defaultPath ?? '';
}

/** Empty provider form, mirroring the admin API payload. */
export function blankProvider() {
  return {
    name: '',
    description: '',
    base_url: '',
    auth_type: 'bearer',
    api_key: '',
    // { 'chat.completions': { enabled, path }, ... }
    api_formats: Object.fromEntries(API_FORMATS.map((p) => [p.value, { enabled: true, path: '' }])),
    headers: {},
    timeout_ms: 120000,
    retry_count: 0,
    retry_backoff: 'exponential',
    retry_initial_ms: 500,
    retry_max_ms: 10000,
    retry_linear_interval_ms: 5000,
    proxy_type: 'none',
    proxy_url: '',
    proxy_username: '',
    proxy_password: '',
    health_check_url: '',
    health_check_method: 'POST',
    health_check_headers: {},
    health_check_body: '',
    health_check_interval_ms: 0,
    health_check_timeout_ms: 0,
    enabled: true
  };
}

/** Map an API provider record onto the editable form shape. */
export function providerToForm(provider) {
  const form = blankProvider();
  form.name = provider.name ?? '';
  form.description = provider.description ?? '';
  form.base_url = provider.base_url ?? '';
  form.auth_type = provider.auth_type ?? 'bearer';
  form.headers = { ...(provider.headers || {}) };
  form.timeout_ms = provider.timeout_ms ?? 120000;
  form.retry_count = provider.retry_count ?? 0;
  form.retry_backoff = provider.retry_backoff || 'exponential';
  form.retry_initial_ms = provider.retry_initial_ms ?? 500;
  form.retry_max_ms = provider.retry_max_ms ?? 10000;
  form.retry_linear_interval_ms = provider.retry_linear_interval_ms ?? 5000;
  form.proxy_type = provider.proxy_type || 'none';
  form.proxy_url = provider.proxy_url || '';
  form.proxy_username = provider.proxy_username || '';
  form.health_check_url = provider.health_check_url || '';
  form.health_check_method = provider.health_check_method || 'POST';
  form.health_check_headers = { ...(provider.health_check_headers || {}) };
  form.health_check_body = provider.health_check_body || '';
  form.health_check_interval_ms = provider.health_check_interval_ms ?? 0;
  form.health_check_timeout_ms = provider.health_check_timeout_ms ?? 0;
  form.enabled = provider.enabled;
  // Rehydrate configured API formats; a provider without an explicit list keeps
  // every protocol enabled with the built-in default path.
  const types = provider.api_types?.length
    ? provider.api_types
    : API_FORMATS.map((p) => p.value);
  const overrides = provider.path_overrides || {};
  form.api_formats = Object.fromEntries(
    API_FORMATS.map((p) => [
      p.value,
      { enabled: types.includes(p.value), path: overrides[p.value] || '' }
    ])
  );
  return form;
}

/** Convert the editable form back into the admin API payload. */
export function formToPayload(form) {
  const apiTypes = [];
  const pathOverrides = {};
  for (const protocol of API_FORMATS) {
    const entry = form.api_formats?.[protocol.value];
    if (!entry?.enabled) continue;
    apiTypes.push(protocol.value);
    const path = (entry.path || '').trim();
    if (path) pathOverrides[protocol.value] = path;
  }
  return {
    name: form.name.trim(),
    description: form.description ?? '',
    base_url: form.base_url.trim(),
    auth_type: form.auth_type,
    api_key: form.api_key,
    headers: form.headers || {},
    timeout_ms: Number(form.timeout_ms) || 120000,
    retry_count: Number(form.retry_count) || 0,
    retry_backoff: form.retry_backoff,
    retry_initial_ms: Number(form.retry_initial_ms) || 0,
    retry_max_ms: Number(form.retry_max_ms) || 0,
    retry_linear_interval_ms: Number(form.retry_linear_interval_ms) || 0,
    proxy_type: form.proxy_type === 'none' ? '' : form.proxy_type || '',
    proxy_url: form.proxy_type && form.proxy_type !== 'none' ? (form.proxy_url || '').trim() : '',
    proxy_username:
      form.proxy_type && form.proxy_type !== 'none' ? (form.proxy_username || '').trim() : '',
    proxy_password: form.proxy_password || '',
    health_check_url: (form.health_check_url || '').trim(),
    health_check_method: form.health_check_method || 'POST',
    health_check_headers: form.health_check_headers || {},
    health_check_body: form.health_check_body || '',
    health_check_interval_ms: Number(form.health_check_interval_ms) || 0,
    health_check_timeout_ms: Number(form.health_check_timeout_ms) || 0,
    api_types: apiTypes,
    path_overrides: pathOverrides,
    enabled: form.enabled
  };
}

/**
 * Validate a form, returning { tab, message } entries so the UI can jump to the
 * offending tab instead of failing at submit time.
 */
export function validateProvider(form) {
  const errors = [];
  if (!form.name.trim()) errors.push({ tab: 'basic', message: '请填写名称' });
  if (!form.base_url.trim()) {
    errors.push({ tab: 'basic', message: '请填写 Base URL' });
  } else if (!/^https?:\/\//i.test(form.base_url.trim())) {
    errors.push({ tab: 'basic', message: 'Base URL 必须以 http:// 或 https:// 开头' });
  }
  const enabledFormats = Object.values(form.api_formats || {}).filter((v) => v.enabled);
  if (!enabledFormats.length) errors.push({ tab: 'basic', message: '至少启用一种 API 格式' });

  if (form.proxy_type && form.proxy_type !== 'none' && !form.proxy_url.trim()) {
    errors.push({ tab: 'proxy', message: '启用代理时必须填写代理 URL' });
  }
  if (form.retry_count < 0 || form.retry_count > 20) {
    errors.push({ tab: 'request', message: '重试次数必须在 0-20 之间' });
  }
  if (form.retry_initial_ms < 0 || form.retry_max_ms < 0) {
    errors.push({ tab: 'request', message: '退避时间不能为负数' });
  }
  if (form.retry_backoff === 'exponential' && form.retry_max_ms < form.retry_initial_ms) {
    errors.push({ tab: 'request', message: '最大退避不能小于基础退避' });
  }
  if (form.health_check_method === 'GET' && (form.health_check_body || '').trim()) {
    errors.push({ tab: 'health', message: 'GET 方法不支持请求 Body' });
  }
  if (form.health_check_interval_ms < 0 || form.health_check_timeout_ms < 0) {
    errors.push({ tab: 'health', message: '请求间隔与超时不能为负数' });
  }
  return errors;
}

/** Snapshot of editable state, used for dirty tracking. */
export function formSignature(form, headerRows, healthHeaderRows) {
  return JSON.stringify({ form, headerRows, healthHeaderRows });
}

/** Convert a header map into editable rows. */
export function mapToRows(map) {
  return Object.entries(map || {}).map(([name, value]) => ({ name, value: String(value) }));
}

/** Convert editable rows into a header map, dropping rows without a name. */
export function rowsToMap(rows) {
  const out = {};
  for (const row of rows || []) {
    const name = (row.name || '').trim();
    if (!name) continue;
    out[name] = row.value ?? '';
  }
  return out;
}

/** Human-friendly millisecond formatting for summaries. */
export function formatDuration(ms) {
  const value = Number(ms) || 0;
  if (value <= 0) return '—';
  if (value < 1000) return `${value}ms`;
  if (value % 60000 === 0) return `${value / 60000} 分钟`;
  if (value < 60000) return `${Number((value / 1000).toFixed(1))}s`;
  return `${Math.round(value / 1000)}s`;
}

/** Load every provider from the admin API. */
export async function loadProviders() {
  return api('/api/admin/providers');
}
