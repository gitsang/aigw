import { t } from '$lib/i18n/index.svelte.js';

/**
 * Parse a token budget such as "256K", "1M" or "131072".
 *
 * Binary units: 1K = 1024, 1M = 1024K = 1048576. Returns NaN when the input is
 * not a valid budget so callers can show a validation error.
 */
export function parseTokenBudget(input) {
  const raw = String(input ?? '').trim();
  if (raw === '') return 0;
  const match = /^(\d+(?:\.\d+)?)\s*([kKmM]?)$/.exec(raw);
  if (!match) return NaN;
  const value = Number(match[1]);
  const unit = match[2].toUpperCase();
  if (unit === 'K') return Math.round(value * 1024);
  if (unit === 'M') return Math.round(value * 1024 * 1024);
  return Math.round(value);
}

/** Render a token budget using the shortest exact K/M form. */
export function formatTokenBudget(tokens) {
  const value = Number(tokens) || 0;
  if (value <= 0) return '';
  if (value % (1024 * 1024) === 0) return `${value / (1024 * 1024)}M`;
  if (value % 1024 === 0) return `${value / 1024}K`;
  return String(value);
}

/** Empty editable model row. */
export function blankModel() {
  return {
    model_id: '',
    display_name: '',
    context_window: '',
    max_output_tokens: '',
    cost: {
      currency: 'USD',
      input: 0,
      output: 0,
      cache_read: 0,
      cache_write: 0,
      tiers: []
    },
    enabled: true
  };
}

/** Map an API model record onto the editable row shape. */
export function modelToForm(model) {
  const cost = model.cost || {};
  return {
    model_id: model.model_id ?? '',
    display_name: model.display_name ?? '',
    context_window: formatTokenBudget(model.context_window),
    max_output_tokens: formatTokenBudget(model.max_output_tokens),
    // Stored as-is: an empty schema means "inherit from the provider".
    api_schema: model.api_schema || {},
    cost: {
      currency: cost.currency || 'USD',
      input: Number(cost.input) || 0,
      output: Number(cost.output) || 0,
      cache_read: Number(cost.cache_read) || 0,
      cache_write: Number(cost.cache_write) || 0,
      tiers: (cost.tiers || []).map((tier) => ({
        input_tokens_above: formatTokenBudget(tier.input_tokens_above),
        input: Number(tier.input) || 0,
        output: Number(tier.output) || 0,
        cache_read: Number(tier.cache_read) || 0,
        cache_write: Number(tier.cache_write) || 0
      }))
    },
    enabled: model.enabled !== false
  };
}

/**
 * Convert an editable model row into the admin API payload.
 *
 * The API schema is kept out of the editor on purpose: the provider-level
 * schema already covers the common case and the gateway validates support. The
 * value is passed through untouched so an unset model schema keeps inheriting
 * from its provider.
 */
export function modelToPayload(form) {
  return {
    model_id: (form.model_id || '').trim(),
    display_name: (form.display_name || '').trim(),
    context_window: parseTokenBudget(form.context_window) || 0,
    max_output_tokens: parseTokenBudget(form.max_output_tokens) || 0,
    cost: {
      currency: (form.cost?.currency || 'USD').toUpperCase(),
      input: Number(form.cost?.input) || 0,
      output: Number(form.cost?.output) || 0,
      cache_read: Number(form.cost?.cache_read) || 0,
      cache_write: Number(form.cost?.cache_write) || 0,
      tiers: (form.cost?.tiers || [])
        .map((tier) => ({
          input_tokens_above: parseTokenBudget(tier.input_tokens_above) || 0,
          input: Number(tier.input) || 0,
          output: Number(tier.output) || 0,
          cache_read: Number(tier.cache_read) || 0,
          cache_write: Number(tier.cache_write) || 0
        }))
        .sort((a, b) => a.input_tokens_above - b.input_tokens_above)
    },
    enabled: form.enabled,
    api_schema: form.api_schema || {}
  };
}

/** Validate an editable model row, returning a list of messages. */
export function validateModel(form) {
  const errors = [];
  if (!(form.model_id || '').trim()) errors.push(t('models.form.modelIdRequired'));

  for (const [labelKey, value] of [
    ['models.form.contextWindow', form.context_window],
    ['models.form.maxOutput', form.max_output_tokens]
  ]) {
    if (String(value ?? '').trim() === '') continue;
    if (Number.isNaN(parseTokenBudget(value))) {
      errors.push(t('models.form.invalidBudget', { label: t(labelKey) }));
    }
  }

  const currency = (form.cost?.currency || '').toUpperCase();
  if (!/^[A-Z]{3}$/.test(currency)) {
    errors.push(t('models.form.currencyInvalid'));
  }

  for (const [nameKey, value] of [
    ['models.form.price.input', form.cost?.input],
    ['models.form.price.output', form.cost?.output],
    ['models.form.price.cacheRead', form.cost?.cache_read],
    ['models.form.price.cacheWrite', form.cost?.cache_write]
  ]) {
    if (Number(value) < 0) errors.push(t('models.form.negativePrice', { name: t(nameKey) }));
  }

  let previous = -1;
  (form.cost?.tiers || []).forEach((tier, index) => {
    const threshold = parseTokenBudget(tier.input_tokens_above);
    if (Number.isNaN(threshold) || threshold <= 0) {
      errors.push(t('models.form.tierThresholdInvalid', { index: index + 1 }));
      return;
    }
    if (threshold <= previous) {
      errors.push(t('models.form.tierThresholdsAscending'));
    }
    previous = threshold;
  });
  return errors;
}

/** Blank tier row for the cost editor. */
export function blankTier() {
  return {
    input_tokens_above: '',
    input: 0,
    output: 0,
    cache_read: 0,
    cache_write: 0
  };
}
