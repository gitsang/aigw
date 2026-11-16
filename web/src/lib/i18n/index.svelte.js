/**
 * Console i18n core.
 *
 * English is the default and the fallback: a key missing from the active
 * dictionary falls back to English, and a key missing everywhere renders as the
 * key itself so a gap is visible instead of silently empty.
 *
 * Dictionaries live in `./locales/<locale>/` and are split per feature so that
 * each page owns its own namespace file.
 */
import { browser } from '$app/environment';

import en from './locales/en/index.js';
import zh from './locales/zh/index.js';

/** Supported console locales, in switcher order. */
export const LOCALES = [
  { id: 'en', intl: 'en-US' },
  { id: 'zh', intl: 'zh-CN' }
];

export const DEFAULT_LOCALE = 'en';

const DICTIONARIES = { en, zh };
const INTL_LOCALES = Object.fromEntries(LOCALES.map((locale) => [locale.id, locale.intl]));
const STORAGE_KEY = 'aigw_locale';

/**
 * Reactive locale state. Kept in a module-level rune so that every `t()` call
 * made from a template or a `$derived` re-runs when the locale changes.
 */
export const i18n = $state({ locale: DEFAULT_LOCALE });

function lookup(dictionary, key) {
  return key
    .split('.')
    .reduce(
      (node, part) => (node && typeof node === 'object' ? node[part] : undefined),
      dictionary
    );
}

/**
 * Translate a dot-separated key, interpolating `{placeholder}` tokens from
 * `params`. Returns the key when the message is missing from every dictionary.
 *
 * Plural forms are opt-in per locale: when `params.count` is a number and the
 * dictionary defines `<key>_one` / `<key>_other`, the matching form is used and
 * the plain key is only a fallback. Locales without plural forms (Chinese) just
 * define the plain key and are unaffected.
 */
export function t(key, params = {}) {
  const dictionary = DICTIONARIES[i18n.locale] ?? DICTIONARIES[DEFAULT_LOCALE];
  const forms =
    typeof params.count === 'number' ? [`${key}_${params.count === 1 ? 'one' : 'other'}`, key] : [key];
  let message;
  for (const candidate of forms) {
    message = lookup(dictionary, candidate);
    if (typeof message !== 'string') {
      message = lookup(DICTIONARIES[DEFAULT_LOCALE], candidate);
    }
    if (typeof message === 'string') break;
  }
  if (typeof message !== 'string') {
    if (import.meta.env.DEV && browser) {
      console.warn(`[i18n] missing message for key: ${key}`);
    }
    return key;
  }
  return message.replace(/\{(\w+)\}/g, (match, name) =>
    Object.prototype.hasOwnProperty.call(params, name) ? String(params[name]) : match
  );
}

/** Switch the active locale, persisting the choice and syncing `<html lang>`. */
export function setLocale(id) {
  const next = DICTIONARIES[id] ? id : DEFAULT_LOCALE;
  i18n.locale = next;
  if (!browser) return;
  try {
    localStorage.setItem(STORAGE_KEY, next);
  } catch {
    /* storage may be unavailable (private mode); keep the in-memory choice */
  }
  document.documentElement.setAttribute('lang', INTL_LOCALES[next] ?? next);
}

/** Restore the persisted locale on startup. */
export function initLocale() {
  if (!browser) return;
  let stored = '';
  try {
    stored = localStorage.getItem(STORAGE_KEY) || '';
  } catch {
    stored = '';
  }
  setLocale(stored || DEFAULT_LOCALE);
}

/** BCP 47 tag used by `Intl` formatters for the active locale. */
export function intlLocale() {
  return INTL_LOCALES[i18n.locale] ?? INTL_LOCALES[DEFAULT_LOCALE];
}

/** Label of a locale in its own language, e.g. "English" or "Chinese". */
export function localeLabel(id) {
  return t(`common.language.${id}`);
}
