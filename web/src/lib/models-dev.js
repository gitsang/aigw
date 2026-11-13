/**
 * models.dev price sync helpers.
 *
 * The browser never talks to models.dev directly (CORS + ~6 MB payload); every
 * lookup goes through the admin API of the gateway. This module owns the HTTP
 * calls and the mapping between the admin API shape and the model editor form.
 */

import { api } from '$lib/api';
import { formatTokenBudget, parseTokenBudget } from '$lib/model-form.js';

/** Coerce an API price value to a finite non-negative number. */
function priceNumber(value) {
  const number = Number(value);
  return Number.isFinite(number) && number >= 0 ? number : 0;
}

/** True when a token-budget form field is empty (limits are opt-in). */
export function isBlankTokenBudget(value) {
  return String(value ?? '').trim() === '';
}

/**
 * Fetch the models.dev candidates for one provider model.
 *
 * @param {string} providerId provider id from the admin API
 * @param {string} modelId model id typed in the editor (trimmed here)
 * @returns {Promise<{model_id: string, fetched_at: string, matches: object[]}>}
 */
export function lookupModelsDev(providerId, modelId) {
  const id = String(modelId ?? '').trim();
  if (!id) return Promise.reject(new Error('请先填写模型 ID'));
  const query = new URLSearchParams({ model_id: id });
  return api(`/api/admin/providers/${encodeURIComponent(providerId)}/models/modelsdev?${query}`);
}

/**
 * Preview (dry_run) or apply the bulk models.dev price sync for a provider.
 *
 * `sourceProvider` restricts matching to one models.dev catalog provider
 * (required for aggregator providers whose model ids exist under many catalog
 * entries at different prices). Empty means automatic slug/name ranking.
 *
 * @param {string} providerId provider id from the admin API
 * @param {{dryRun?: boolean, overwrite?: boolean, sourceProvider?: string}} [options]
 * @returns {Promise<{dry_run: boolean, source_provider: string, fetched_at: string, sources: object[]|null, items: object[], counts: Record<string, number>}>}
 */
export function syncModelPrices(
  providerId,
  { dryRun = true, overwrite = false, sourceProvider = '' } = {}
) {
  return api(`/api/admin/providers/${encodeURIComponent(providerId)}/models/sync-prices`, {
    method: 'POST',
    body: JSON.stringify({
      dry_run: Boolean(dryRun),
      overwrite: Boolean(overwrite),
      source_provider: String(sourceProvider || '').trim()
    })
  });
}

/**
 * Decide whether the console should default the bulk-sync dialog to an explicit
 * source instead of automatic matching.
 *
 * Automatic matching can only confirm a single source; when it leaves at least
 * as many models unmatched/ambiguous as it can actually price, the best-covering
 * catalog provider (sources[0]) is the more useful default.
 *
 * @param {object|null} payload bulk-sync response from auto mode
 * @returns {boolean}
 */
export function shouldPreferExplicitSource(payload) {
  const counts = payload?.counts || {};
  const attention = (Number(counts.unmatched) || 0) + (Number(counts.ambiguous) || 0);
  if (attention === 0) return false;
  const usable =
    (Number(counts.update) || 0) +
    (Number(counts.skip_has_price) || 0) +
    (Number(counts.unchanged) || 0);
  return attention >= usable;
}

/** Render one source entry as "<provider_name> (<provider_id>) · 覆盖 N 个模型". */
export function modelsDevSourceOptionLabel(source) {
  if (!source) return '';
  const provider = source.provider_name || source.provider_id || '—';
  return `${provider} (${source.provider_id || '—'}) · 覆盖 ${Number(source.models) || 0} 个模型`;
}

/** Resolve the display name of a catalog provider id from a sources list. */
export function modelsDevSourceName(sources, providerId) {
  const id = String(providerId || '').trim();
  if (!id) return '';
  const found = (Array.isArray(sources) ? sources : []).find((source) => source?.provider_id === id);
  return found?.provider_name || id;
}

/** Compact "USD in 0.1 / out 0.4 · 1M" price summary for a cost object. */
export function modelsDevPriceSummary(cost) {
  const source = cost || {};
  const parts = [`in ${trimNumber(source.input)}`, `out ${trimNumber(source.output)}`];
  if (priceNumber(source.cache_read) > 0) parts.push(`cr ${trimNumber(source.cache_read)}`);
  if (priceNumber(source.cache_write) > 0) parts.push(`cw ${trimNumber(source.cache_write)}`);
  if (Array.isArray(source.tiers) && source.tiers.length > 0) {
    parts.push(`${source.tiers.length} 个阶梯`);
  }
  return `${String(source.currency || 'USD').toUpperCase()} ${parts.join(' / ')} · 1M`;
}

/**
 * Price text for a snapshot row: "未设置" when every price is zero, "—" when
 * the admin API omitted the snapshot entirely.
 */
export function modelsDevCostText(cost) {
  if (!cost) return '—';
  const keys = ['input', 'output', 'cache_read', 'cache_write'];
  const zero = keys.every((key) => priceNumber(cost[key]) === 0);
  const hasTiers = Array.isArray(cost.tiers) && cost.tiers.length > 0;
  if (zero && !hasTiers) return '未设置';
  return modelsDevPriceSummary(cost);
}

/** "provider_name · model_id" label for an applied match. */
export function modelsDevSourceLabel(match) {
  if (!match) return '';
  const provider = match.provider_name || match.provider_id || '—';
  return `${provider} · ${match.model_id || '—'}`;
}

/** Badge metadata for a lookup match score (0 = provider, 2 = model only). */
export function modelsDevScoreMeta(score) {
  if (score === 0) return { label: 'Provider 匹配', tone: 'success' };
  if (score === 1) return { label: '名称匹配', tone: 'info' };
  return { label: '仅模型匹配', tone: 'neutral' };
}

const ACTION_META = {
  update: { label: '可更新', tone: 'success' },
  skip_has_price: { label: '已有价格跳过', tone: 'warning' },
  unmatched: { label: '未匹配', tone: 'neutral' },
  ambiguous: { label: '多个匹配', tone: 'warning' },
  unchanged: { label: '无变化', tone: 'neutral' },
  error: { label: '失败', tone: 'danger' }
};

/** Every bulk-sync action key, in display order. */
export const MODELSDEV_ACTIONS = Object.keys(ACTION_META);

/** Badge metadata for a bulk-sync action. */
export function modelsDevActionMeta(action) {
  return ACTION_META[action] || { label: String(action ?? '未知'), tone: 'neutral' };
}

/**
 * Patch the model editor form with one lookup match.
 *
 * Prices/currency/tiers are always replaced. Token limits are only filled when
 * the matching form field is currently empty (never clobber user input).
 * The caller assigns the returned keys onto its reactive form state.
 *
 * @param {object|null} form current editor form
 * @param {object} match match object from the lookup API
 * @returns {{cost: object, context_window?: string, max_output_tokens?: string}}
 */
export function modelsDevFormPatch(form, match) {
  const source = match?.cost || {};
  const patch = {
    cost: {
      currency: 'USD',
      input: priceNumber(source.input),
      output: priceNumber(source.output),
      cache_read: priceNumber(source.cache_read),
      cache_write: priceNumber(source.cache_write),
      tiers: (Array.isArray(source.tiers) ? source.tiers : [])
        .map((tier) => ({
          input_tokens_above: formatTokenBudget(Number(tier?.input_tokens_above) || 0),
          input: priceNumber(tier?.input),
          output: priceNumber(tier?.output),
          cache_read: priceNumber(tier?.cache_read),
          cache_write: priceNumber(tier?.cache_write)
        }))
        .filter((tier) => tier.input_tokens_above !== '')
        .sort(
          (a, b) => (parseTokenBudget(a.input_tokens_above) || 0) - (parseTokenBudget(b.input_tokens_above) || 0)
        )
    }
  };

  const contextWindow = Number(match?.context_window) || 0;
  if (contextWindow > 0 && isBlankTokenBudget(form?.context_window)) {
    patch.context_window = formatTokenBudget(contextWindow);
  }

  const maxOutputTokens = Number(match?.max_output_tokens) || 0;
  if (maxOutputTokens > 0 && isBlankTokenBudget(form?.max_output_tokens)) {
    patch.max_output_tokens = formatTokenBudget(maxOutputTokens);
  }

  return patch;
}

/** Format a price with up to 6 decimals, matching the model list rows. */
export function trimNumber(value) {
  return Number(value || 0).toLocaleString('en-US', {
    minimumFractionDigits: 0,
    maximumFractionDigits: 6
  });
}
