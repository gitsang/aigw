/**
 * Locale-aware value formatters for the console.
 *
 * Every helper reads the active locale through `intlLocale()`, so calling one
 * from a Svelte template keeps that output in sync with the language switcher.
 */
import { intlLocale } from './index.svelte.js';

const EMPTY = '—';

/** Group integer/decimal digits using the active locale conventions. */
export function formatNumber(value, options) {
  return new Intl.NumberFormat(intlLocale(), options).format(Number(value || 0));
}

/** Price rendered the way the gateway stores it: USD, six decimals by default. */
export function formatMoney(value, digits = 6) {
  return `$${Number(value || 0).toFixed(digits)}`;
}

/** Price rendered without a currency symbol, trimming trailing zeros. */
export function formatPrice(value) {
  return formatNumber(value, { minimumFractionDigits: 0, maximumFractionDigits: 6 });
}

/** Full date and time; `—` for empty or unparseable input. */
export function formatTime(value) {
  if (!value) return EMPTY;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return EMPTY;
  return date.toLocaleString(intlLocale(), { hour12: false });
}

/** Time of day with seconds, used by the "last updated" hints. */
export function formatTimeOfDay(value) {
  if (!value) return EMPTY;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return EMPTY;
  return date.toLocaleTimeString(intlLocale(), { hour12: false });
}

/** Hour and minute only, for chart axes where seconds add noise. */
export function formatHourMinute(value) {
  if (!value) return EMPTY;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return EMPTY;
  return date.toLocaleTimeString(intlLocale(), {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false
  });
}

/** Duration in milliseconds, switching to seconds above one second. */
export function formatLatency(ms) {
  const value = Number(ms || 0);
  return value >= 1000 ? `${(value / 1000).toFixed(2)}s` : `${Math.round(value)}ms`;
}

/** `part` as a percentage of `total`, one decimal place. */
export function formatPercent(part, total) {
  if (!total) return '0%';
  return `${((Number(part || 0) / Number(total)) * 100).toFixed(1)}%`;
}

/** Token counts rendered compactly (1024 -> 1K, 1048576 -> 1M). */
export function formatCompactNumber(value) {
  return new Intl.NumberFormat(intlLocale(), {
    notation: 'compact',
    maximumFractionDigits: 1
  }).format(Number(value || 0));
}
