<script>
  import { toast } from 'svelte-sonner';
  import PlusIcon from '@lucide/svelte/icons/plus';
  import Trash2Icon from '@lucide/svelte/icons/trash-2';
  import CheckIcon from '@lucide/svelte/icons/check';
  import XIcon from '@lucide/svelte/icons/x';
  import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
  import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
  import LayersIcon from '@lucide/svelte/icons/layers';
  import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
  import SparklesIcon from '@lucide/svelte/icons/sparkles';
  import CloudDownloadIcon from '@lucide/svelte/icons/cloud-download';

  import { api, fmtTime } from '$lib/api';
  import { t } from '$lib/i18n/index.svelte.js';
  import { formatNumber, formatPrice } from '$lib/i18n/format.js';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import * as Select from '$lib/components/ui/select/index.js';
  import {
    modelsDevActionMeta,
    modelsDevCostText,
    modelsDevFormPatch,
    modelsDevPriceSummary,
    modelsDevScoreMeta,
    modelsDevSourceLabel,
    modelsDevSourceName,
    modelsDevSourceOptionLabel,
    lookupModelsDev,
    shouldPreferExplicitSource,
    syncModelPrices,
    MODELSDEV_ACTIONS
  } from '$lib/models-dev.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Label } from '$lib/components/ui/label/index.js';
  import { Switch } from '$lib/components/ui/switch/index.js';
  import * as Empty from '$lib/components/ui/empty/index.js';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import { Alert, AlertDescription } from '$lib/components/ui/alert/index.js';
  import { Separator } from '$lib/components/ui/separator/index.js';
  import { Spinner } from '$lib/components/ui/spinner/index.js';
  import StatusBadge from '$lib/components/app/status-badge.svelte';

  import {
    blankModel,
    modelToForm,
    modelToPayload,
    validateModel,
    parseTokenBudget,
    formatTokenBudget,
    blankTier
  } from '$lib/model-form.js';

  let { providerId = '', providerName = '', models = $bindable([]) } = $props();

  let loading = $state(true);
  let error = $state('');
  // The row whose editor is expanded in place, or null when everything is closed.
  let expandedId = $state(null);
  let creating = $state(false);
  let form = $state(null);
  let saving = $state(false);
  let deleting = $state(null);
  let discovering = $state(false);

  // Per-model models.dev lookup state. Candidates are only shown for the model
  // editor that is currently open, so they are dropped whenever it changes.
  let lookupBusy = $state(false);
  let lookupMatches = $state([]);
  let lookupFetchedAt = $state('');
  let appliedMatch = $state(null);

  // Sentinel value for the "automatic matching" option in the source picker.
  // Declared before the state that initializes from it: a `const` read before
  // its declaration throws a ReferenceError at component init.
  const AUTO_SOURCE = '__auto__';

  // Bulk models.dev price sync dialog state.
  let syncOpen = $state(false);
  let syncOverwrite = $state(false);
  let syncLoading = $state(false);
  let syncApplying = $state(false);
  let syncError = $state('');
  let syncPreview = $state(null);
  // Selected catalog source. AUTO_SOURCE means automatic slug/name matching.
  // Aggregator providers must pick an explicit source, otherwise nearly every
  // model is ambiguous because models.dev lists it under many providers at
  // prices that genuinely differ.
  let syncSource = $state(AUTO_SOURCE);
  // Catalog sources from the auto-mode preview, kept while an explicit source is
  // selected because the API only returns `sources` in auto mode.
  let syncSources = $state([]);
  // Guards the one-time auto selection so switching sources never re-defaults.
  let syncAutoDefaultDone = false;
  // Incremented on every preview request so late responses never overwrite the
  // state of a newer request (or a closed dialog).
  let syncRequest = 0;

  const syncItems = $derived(Array.isArray(syncPreview?.items) ? syncPreview.items : []);
  const syncCounts = $derived(syncPreview?.counts || {});
  const syncUpdateCount = $derived(Number(syncCounts.update) || 0);
  const canApplySync = $derived(
    !syncLoading && !syncApplying && !syncError && syncUpdateCount > 0
  );
  // Empty string means automatic matching to the admin API.
  const syncSourceProvider = $derived(syncSource === AUTO_SOURCE ? '' : syncSource);
  // One `{ value, label }` pair feeds both `Select.Root items` (so the closed
  // trigger renders the mapped label from data) and the option list, keeping
  // the trigger, the menu and the footer summary identical.
  const syncSourceItems = $derived([
    { value: AUTO_SOURCE, label: t('providers.sync.autoMatch') },
    ...syncSources.map((source) => ({
      value: source.provider_id,
      label: modelsDevSourceOptionLabel(source)
    }))
  ]);
  const syncSourceLabel = $derived(
    syncSourceItems.find((item) => item.value === syncSource)?.label ??
      modelsDevSourceName(syncSources, syncSource)
  );

  async function load() {
    if (!providerId) return;
    loading = true;
    error = '';
    try {
      models = await api(`/api/admin/providers/${providerId}/models`);
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    // Reload whenever the selected provider changes and drop any open editor.
    providerId;
    closeForm();
    closeSyncDialog();
    load();
  });

  function closeForm() {
    expandedId = null;
    creating = false;
    form = null;
    resetLookup();
  }

  /** Drop per-model models.dev candidate/applied state. */
  function resetLookup() {
    lookupBusy = false;
    lookupMatches = [];
    lookupFetchedAt = '';
    appliedMatch = null;
  }

  function openCreate() {
    if (creating) {
      closeForm();
      return;
    }
    expandedId = null;
    creating = true;
    form = blankModel();
    resetLookup();
  }

  function toggle(model) {
    if (expandedId === model.id) {
      closeForm();
      return;
    }
    creating = false;
    expandedId = model.id;
    form = modelToForm(model);
    resetLookup();
  }

  async function save() {
    const errors = validateModel(form);
    if (errors.length) {
      toast.error(errors[0]);
      return;
    }
    saving = true;
    try {
      const payload = modelToPayload(form);
      if (expandedId) {
        await api(`/api/admin/providers/${providerId}/models/${expandedId}`, {
          method: 'PUT',
          body: JSON.stringify(payload)
        });
        toast.success(t('providers.model.updatedToast'));
      } else {
        await api(`/api/admin/providers/${providerId}/models`, {
          method: 'POST',
          body: JSON.stringify(payload)
        });
        toast.success(t('providers.model.addedToast'));
      }
      closeForm();
      await load();
    } catch (e) {
      toast.error(e.message);
    } finally {
      saving = false;
    }
  }

  async function remove(model) {
    deleting = model.id;
    try {
      await api(`/api/admin/providers/${providerId}/models/${model.id}`, { method: 'DELETE' });
      toast.success(t('providers.model.deletedToast'));
      if (expandedId === model.id) closeForm();
      await load();
    } catch (e) {
      toast.error(e.message);
    } finally {
      deleting = null;
    }
  }

  async function discover() {
    discovering = true;
    try {
      const res = await api(`/api/admin/providers/${providerId}/models/discover`, {
        method: 'POST'
      });
      models = res.models;
      toast.success(t('providers.model.discoverSuccess', { count: res.discovered }));
    } catch (e) {
      toast.error(t('providers.model.discoverFailed', { error: e.message }));
    } finally {
      discovering = false;
    }
  }

  // ---- per-model models.dev sync -------------------------------------------

  /** Look up models.dev candidates for the model currently edited. */
  async function syncFromModelsDev(f) {
    const modelId = (f.model_id || '').trim();
    if (!modelId) {
      toast.error(t('providers.modelsDev.modelIdRequired'));
      return;
    }
    lookupBusy = true;
    lookupMatches = [];
    lookupFetchedAt = '';
    appliedMatch = null;
    try {
      const res = await lookupModelsDev(providerId, modelId);
      // Drop the result when the editor moved on while the request was in flight.
      if (form !== f || (f.model_id || '').trim() !== modelId) return;
      lookupFetchedAt = res?.fetched_at || '';
      const matches = Array.isArray(res?.matches) ? res.matches : [];
      // models.dev lists the same model id under many providers (often 10+, the
      // API caps the list), so "more than one match" is not the same as "the
      // source is unclear". Only ask the operator to pick when the best match is
      // tied; a strictly better match (provider slug or API host) is applied
      // directly, which is the common case for a direct provider.
      const best = uniqueBestMatch(matches);
      if (matches.length === 0) {
        toast.error(t('providers.modelsDev.notFound'));
      } else if (best) {
        applyMatch(f, best);
      } else {
        lookupMatches = matches;
      }
    } catch (e) {
      if (form !== f || (f.model_id || '').trim() !== modelId) return;
      toast.error(t('providers.modelsDev.lookupFailed', { error: e.message }));
    } finally {
      if (form === f && (f.model_id || '').trim() === modelId) lookupBusy = false;
    }
  }

  /**
   * Return the single best-ranked match, or null when the top score is shared.
   *
   * Matches arrive sorted by score (ascending); the callers treat a unique
   * winner as unambiguous even when lower-ranked candidates exist.
   */
  function uniqueBestMatch(matches) {
    if (!Array.isArray(matches) || matches.length === 0) return null;
    if (matches.length === 1) return matches[0];
    const bestScore = matches[0].score;
    const tied = matches.filter((match) => match.score === bestScore);
    return tied.length === 1 ? matches[0] : null;
  }

  /** Replace prices (and fill empty token limits) with one models.dev match. */
  function applyMatch(f, match) {
    if (!f || !match) return;
    const patch = modelsDevFormPatch(f, match);
    f.cost = patch.cost;
    if (patch.context_window !== undefined) f.context_window = patch.context_window;
    if (patch.max_output_tokens !== undefined) f.max_output_tokens = patch.max_output_tokens;
    appliedMatch = match;
    lookupMatches = [];
    toast.success(t('providers.modelsDev.applied'));
  }

  // ---- bulk models.dev price sync dialog ----------------------------------

  function openSyncDialog() {
    if (!providerId) return;
    syncOverwrite = false;
    syncPreview = null;
    syncError = '';
    syncSource = AUTO_SOURCE;
    syncSources = [];
    syncAutoDefaultDone = false;
    syncOpen = true;
    runSyncPreview({ autoDefault: true });
  }

  function closeSyncDialog() {
    syncOpen = false;
    resetSyncState();
  }

  function resetSyncState() {
    syncRequest += 1;
    syncLoading = false;
    syncApplying = false;
    syncError = '';
    syncPreview = null;
    syncOverwrite = false;
    syncSource = AUTO_SOURCE;
    syncSources = [];
    syncAutoDefaultDone = false;
  }

  /**
   * Load the dry-run preview for the current source + overwrite flags.
   *
   * In auto mode the response also carries `sources`, the catalog providers with
   * the widest coverage. On the first auto preview for an aggregator-like
   * provider the dialog defaults to the best source, otherwise automatic
   * matching would classify nearly every model as ambiguous.
   */
  async function runSyncPreview({ autoDefault = false } = {}) {
    if (!providerId) return;
    syncLoading = true;
    syncError = '';
    const requestId = ++syncRequest;
    const source = syncSourceProvider;
    try {
      const res = await syncModelPrices(providerId, {
        dryRun: true,
        overwrite: syncOverwrite,
        sourceProvider: source
      });
      if (requestId !== syncRequest) return;

      // `sources` only comes back in auto mode; cache it so the picker keeps its
      // options after switching to an explicit source.
      const sources = Array.isArray(res?.sources) ? res.sources : null;
      if (sources) syncSources = sources;

      if (autoDefault && !syncAutoDefaultDone) {
        syncAutoDefaultDone = true;
        const best = syncSources[0]?.provider_id || '';
        if (best && shouldPreferExplicitSource(res)) {
          syncSource = best;
          runSyncPreview();
          return;
        }
      }
      syncPreview = res;
    } catch (e) {
      if (requestId !== syncRequest) return;
      syncPreview = null;
      syncError = e.message;
    } finally {
      if (requestId === syncRequest) syncLoading = false;
    }
  }

  function toggleOverwrite(checked) {
    syncOverwrite = checked;
    runSyncPreview();
  }

  /** Switch the catalog source and refresh the preview. */
  function changeSyncSource(value) {
    syncSource = value;
    runSyncPreview();
  }

  /** Apply the previewed changes (dry_run:false) and reload the model list. */
  async function applySync() {
    if (!canApplySync) return;
    syncApplying = true;
    try {
      const source = syncSourceProvider;
      const res = await syncModelPrices(providerId, {
        dryRun: false,
        overwrite: syncOverwrite,
        sourceProvider: source
      });
      const counts = res?.counts || {};
      const updated = Number(counts.update ?? syncUpdateCount) || 0;
      const sourceName = source ? modelsDevSourceName(syncSources, source) : '';
      toast.success(
        sourceName
          ? t('providers.sync.appliedWithSource', { source: sourceName, count: updated })
          : t('providers.sync.applied', { count: updated })
      );
      if (Number(counts.error) > 0) {
        toast.error(t('providers.sync.failedCount', { count: Number(counts.error) }));
      }
      closeSyncDialog();
      await load();
    } catch (e) {
      toast.error(t('providers.sync.failed', { error: e.message }));
    } finally {
      syncApplying = false;
    }
  }

  function addTier(f) {
    f.cost.tiers = [...f.cost.tiers, blankTier()];
  }

  function removeTier(f, index) {
    f.cost.tiers = f.cost.tiers.filter((_, i) => i !== index);
  }

  /** Price grouped by the active locale; the currency symbol is printed separately. */
  function money(value) {
    return formatPrice(value);
  }

  function priceSummary(cost) {
    const parts = [
      `${t('providers.price.in')} ${money(cost.input)}`,
      `${t('providers.price.out')} ${money(cost.output)}`
    ];
    if (cost.cache_read) parts.push(`${t('providers.price.cacheRead')} ${money(cost.cache_read)}`);
    if (cost.cache_write) parts.push(`${t('providers.price.cacheWrite')} ${money(cost.cache_write)}`);
    return `${cost.currency} ${parts.join(' / ')} · ${t('providers.price.perMillion')}`;
  }
</script>

{#snippet modelEditor(f)}
  <div class="space-y-3">
    <div class="flex items-center justify-between">
      <span class="text-xs font-medium"
        >{creating ? t('providers.model.addTitle') : t('providers.model.editTitle')}</span
      >
      <Button
        variant="ghost"
        size="icon-xs"
        onclick={closeForm}
        aria-label={t('providers.model.collapseAria')}
      >
        <XIcon />
      </Button>
    </div>

    <div class="grid gap-3 sm:grid-cols-2">
      <div class="grid gap-1.5">
        <Label for="m-id">{t('providers.model.id')}</Label>
        <Input
          id="m-id"
          class="font-mono text-[11px]"
          bind:value={f.model_id}
          oninput={resetLookup}
          placeholder={t('providers.model.idPlaceholder')}
        />
      </div>
      <div class="grid gap-1.5">
        <Label for="m-name">{t('providers.model.name')}</Label>
        <Input
          id="m-name"
          bind:value={f.display_name}
          placeholder={t('providers.optional')}
        />
      </div>
      <div class="grid gap-1.5">
        <Label for="m-ctx">{t('providers.model.contextWindow')}</Label>
        <Input
          id="m-ctx"
          class="font-mono text-[11px]"
          bind:value={f.context_window}
          placeholder={t('providers.model.contextPlaceholder')}
        />
        <p class="text-[10.5px] text-muted-foreground">
          {#if parseTokenBudget(f.context_window) > 0}
            = {formatNumber(parseTokenBudget(f.context_window))} {t('common.unit.tokens')}
          {:else}
            {t('providers.model.tokenHint')}
          {/if}
        </p>
      </div>
      <div class="grid gap-1.5">
        <Label for="m-max">{t('providers.model.maxOutput')}</Label>
        <Input
          id="m-max"
          class="font-mono text-[11px]"
          bind:value={f.max_output_tokens}
          placeholder={t('providers.model.maxOutputPlaceholder')}
        />
        <p class="text-[10.5px] text-muted-foreground">
          {#if parseTokenBudget(f.max_output_tokens) > 0}
            = {formatNumber(parseTokenBudget(f.max_output_tokens))} {t('common.unit.tokens')}
          {:else}
            {t('providers.model.maxOutputHint')}
          {/if}
        </p>
      </div>
    </div>

    <Separator />

    <div class="space-y-2">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div>
          <div class="text-xs font-medium">{t('providers.model.cost.title')}</div>
          <p class="mt-0.5 text-[10.5px] text-muted-foreground">
            {t('providers.model.cost.description')}
          </p>
        </div>
        <div class="flex items-center gap-1.5">
          <div class="w-24">
            <Input
              class="h-7 font-mono text-[11px] uppercase"
              bind:value={f.cost.currency}
              placeholder={t('providers.model.cost.currencyPlaceholder')}
              aria-label={t('providers.model.cost.currencyAria')}
            />
          </div>
          <Button
            variant="outline"
            size="xs"
            onclick={() => syncFromModelsDev(f)}
            disabled={lookupBusy}
            aria-label={t('providers.model.cost.syncAria')}
          >
            {#if lookupBusy}
              <Spinner class="size-2.5" />
              {t('providers.model.cost.lookupBusy')}
            {:else}
              <SparklesIcon />
              {t('providers.model.cost.syncButton')}
            {/if}
          </Button>
        </div>
      </div>

      {#if appliedMatch}
        <div class="flex flex-wrap items-center gap-1.5 text-[10.5px] text-muted-foreground">
          <StatusBadge tone="success">{t('providers.model.cost.applied')}</StatusBadge>
          <span>{t('providers.model.cost.source', { source: modelsDevSourceLabel(appliedMatch) })}</span>
          <span>· {t('providers.model.cost.unit')}</span>
          <span class="font-mono">{modelsDevPriceSummary(appliedMatch.cost)}</span>
        </div>
      {/if}

      {#if lookupMatches.length > 1}
        <div class="space-y-1.5 rounded-md border border-dashed p-2">
          <div class="text-[10.5px] font-medium text-muted-foreground">
            {t('providers.model.cost.multipleMatches')}
          </div>
          {#each lookupMatches as match (match.provider_id + '/' + match.model_id)}
            {@const score = modelsDevScoreMeta(match.score)}
            <button
              type="button"
              class="w-full rounded-md border px-2 py-1.5 text-left transition-colors hover:bg-muted/60 focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:outline-none"
              onclick={() => applyMatch(f, match)}
              aria-label={t('providers.model.cost.applyAria', {
                provider: match.provider_name || match.provider_id
              })}
            >
              <div class="flex flex-wrap items-center gap-1.5">
                <span class="text-[11px] font-medium">
                  {match.provider_name || match.provider_id}
                </span>
                <StatusBadge tone={score.tone}>{score.label}</StatusBadge>
                <span class="font-mono text-[10.5px] text-muted-foreground">
                  {modelsDevPriceSummary(match.cost)}
                </span>
              </div>
              <div
                class="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-0.5 text-[10.5px] text-muted-foreground"
              >
                <span class="font-mono">{match.model_id}</span>
                {#if match.context_window}
                  <span
                    >{t('providers.model.contextWindowValue', {
                      value: formatTokenBudget(match.context_window)
                    })}</span
                  >
                {/if}
                {#if match.max_output_tokens}
                  <span
                    >{t('providers.model.maxOutputValue', {
                      value: formatTokenBudget(match.max_output_tokens)
                    })}</span
                  >
                {/if}
              </div>
            </button>
          {/each}
          {#if lookupFetchedAt}
            <p class="text-[10px] text-muted-foreground">
              {t('providers.model.cost.snapshot', { time: fmtTime(lookupFetchedAt) })}
            </p>
          {/if}
        </div>
      {/if}

      <div class="grid gap-2 sm:grid-cols-4">
        {#each [
          ['input', t('providers.model.cost.input')],
          ['output', t('providers.model.cost.output')],
          ['cache_read', t('providers.model.cost.cacheRead')],
          ['cache_write', t('providers.model.cost.cacheWrite')]
        ] as [key, label] (key)}
          <div class="grid gap-1">
            <Label for={`m-cost-${key}`} class="text-[10.5px] text-muted-foreground">
              {label}
            </Label>
            <Input
              id={`m-cost-${key}`}
              class="h-7 font-mono text-[11px]"
              type="number"
              min="0"
              step="0.000001"
              bind:value={f.cost[key]}
            />
          </div>
        {/each}
      </div>

      <div class="space-y-1.5">
        <div class="flex items-center justify-between">
          <span class="text-[10.5px] font-medium text-muted-foreground">
            {t('providers.model.cost.tiersTitle')}
          </span>
          <Button variant="ghost" size="xs" onclick={() => addTier(f)}>
            <PlusIcon />
            {t('providers.model.cost.addTier')}
          </Button>
        </div>
        {#if f.cost.tiers.length}
          <div class="overflow-hidden rounded-md border">
            <div
              class="grid grid-cols-[minmax(0,1.2fr)_repeat(4,minmax(0,1fr))_28px] gap-2 border-b bg-muted/50 px-2 py-1 text-[10px] font-medium text-muted-foreground"
            >
              <span>{t('providers.model.cost.tierThreshold')} &gt;</span>
              <span>{t('providers.model.cost.tierInput')}</span>
              <span>{t('providers.model.cost.tierOutput')}</span>
              <span>{t('providers.model.cost.tierCacheRead')}</span>
              <span>{t('providers.model.cost.tierCacheWrite')}</span>
              <span></span>
            </div>
            {#each f.cost.tiers as tier, index (index)}
              <div
                class="grid grid-cols-[minmax(0,1.2fr)_repeat(4,minmax(0,1fr))_28px] items-center gap-2 border-b px-2 py-1 last:border-b-0"
              >
                <Input
                  class="h-7 font-mono text-[11px]"
                  bind:value={tier.input_tokens_above}
                  placeholder="200K"
                  aria-label={t('providers.model.cost.tierThresholdAria', { index: index + 1 })}
                />
                {#each ['input', 'output', 'cache_read', 'cache_write'] as key (key)}
                  <Input
                    class="h-7 font-mono text-[11px]"
                    type="number"
                    min="0"
                    step="0.000001"
                    bind:value={tier[key]}
                    aria-label={t('providers.model.cost.tierValueAria', {
                      index: index + 1,
                      field: key
                    })}
                  />
                {/each}
                <Button
                  variant="ghost"
                  size="icon-xs"
                  class="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                  onclick={() => removeTier(f, index)}
                  aria-label={t('providers.model.cost.removeTierAria')}
                >
                  <XIcon />
                </Button>
              </div>
            {/each}
          </div>
        {:else}
          <div class="rounded-md border border-dashed px-2 py-2 text-center text-[10.5px] text-muted-foreground">
            {t('providers.model.cost.noTiers')}
          </div>
        {/if}
      </div>
    </div>

    <div class="flex items-center justify-between gap-2 rounded-md border px-2 py-1.5">
      <span class="text-xs">{t('providers.model.enabled')}</span>
      <Switch bind:checked={f.enabled} />
    </div>

    <div class="flex justify-end gap-1.5">
      <Button variant="ghost" size="sm" onclick={closeForm} disabled={saving}
        >{t('common.action.cancel')}</Button
      >
      <Button size="sm" onclick={save} disabled={saving}>
        {#if saving}
          <Spinner class="size-3.5" />
          {t('common.state.saving')}
        {:else}
          <CheckIcon />
          {t('common.action.save')}
        {/if}
      </Button>
    </div>
  </div>
{/snippet}

<div class="space-y-3">
  <div class="flex flex-wrap items-start justify-between gap-2">
    <div>
      <div class="text-xs font-medium">
        {t('providers.model.listTitle', { count: models.length })}
      </div>
      <p class="mt-0.5 text-[10.5px] text-muted-foreground">
        {t('providers.model.listDescription')}
      </p>
    </div>
    <div class="flex items-center gap-1.5">
      <Button variant="outline" size="sm" onclick={discover} disabled={discovering || !providerId}>
        <RefreshCwIcon class={discovering ? 'animate-spin' : ''} />
        {t('providers.model.discoverButton')}
      </Button>
      <Button
        variant="outline"
        size="sm"
        onclick={openSyncDialog}
        disabled={!providerId || !models.length}
        title={t('providers.model.syncPricesTitle')}
      >
        <CloudDownloadIcon />
        {t('providers.model.syncPricesButton')}
      </Button>
      <Button size="sm" onclick={openCreate} disabled={!providerId}>
        <PlusIcon />
        {creating ? t('providers.model.collapse') : t('providers.model.addTitle')}
      </Button>
    </div>
  </div>

  {#if error}
    <Alert variant="destructive">
      <TriangleAlertIcon />
      <AlertDescription>{error}</AlertDescription>
    </Alert>
  {/if}

  {#if creating && form}
    <div class="rounded-lg border bg-muted/30 p-3">
      {@render modelEditor(form)}
    </div>
  {/if}

  <div class="overflow-hidden rounded-lg border">
    {#if loading}
      <div class="space-y-2 p-3">
        {#each Array(3) as _, i (i)}
          <Skeleton class="h-10 w-full" />
        {/each}
      </div>
    {:else if !models.length}
      <Empty.Root class="border-0 py-8">
        <Empty.Header>
          <Empty.Media variant="icon"><LayersIcon /></Empty.Media>
          <Empty.Title>{t('providers.model.emptyTitle')}</Empty.Title>
          <Empty.Description>{t('providers.model.emptyDescription')}</Empty.Description>
        </Empty.Header>
      </Empty.Root>
    {:else}
      {#each models as model (model.id)}
        <div class="border-b last:border-b-0">
          <div class="flex items-start justify-between gap-2 px-3 py-2">
            <button
              type="button"
              class="flex min-w-0 flex-1 items-start gap-2 text-left"
              onclick={() => toggle(model)}
              aria-expanded={expandedId === model.id}
            >
              <ChevronRightIcon
                class="mt-0.5 size-3.5 shrink-0 text-muted-foreground transition-transform {expandedId ===
                model.id
                  ? 'rotate-90'
                  : ''}"
              />
              <div class="min-w-0 space-y-1">
                <div class="flex flex-wrap items-center gap-1.5">
                  <span class="truncate font-mono text-[11.5px] font-medium">{model.model_id}</span>
                  {#if model.display_name && model.display_name !== model.model_id}
                    <span class="truncate text-[10.5px] text-muted-foreground">
                      {model.display_name}
                    </span>
                  {/if}
                  <StatusBadge tone={model.source === 'discovered' ? 'info' : 'violet'}>
                    {model.source === 'discovered'
                      ? t('providers.model.sourceDiscovered')
                      : t('providers.model.sourceManual')}
                  </StatusBadge>
                  {#if !model.enabled}
                    <StatusBadge tone="neutral">{t('providers.status.disabled')}</StatusBadge>
                  {/if}
                </div>
                <div class="flex flex-wrap items-center gap-x-2 gap-y-1 text-[10.5px] text-muted-foreground">
                  {#if model.context_window}
                    <span
                      >{t('providers.model.contextWindowValue', {
                        value: formatTokenBudget(model.context_window)
                      })}</span
                    >
                  {/if}
                  {#if model.max_output_tokens}
                    <span
                      >{t('providers.model.maxOutputValue', {
                        value: formatTokenBudget(model.max_output_tokens)
                      })}</span
                    >
                  {/if}
                  <span class="font-mono">{priceSummary(model.cost || {})}</span>
                  {#if model.cost?.tiers?.length}
                    <StatusBadge tone="violet">
                      {t('providers.model.tiersCount', { count: model.cost.tiers.length })}
                    </StatusBadge>
                  {/if}
                </div>
              </div>
            </button>
            <div class="flex shrink-0 items-center gap-1">
              <Button
                variant="ghost"
                size="xs"
                class="text-destructive hover:bg-destructive/10 hover:text-destructive"
                onclick={() => remove(model)}
                disabled={deleting === model.id}
                aria-label={t('providers.model.deleteAria')}
              >
                {#if deleting === model.id}
                  <Spinner class="size-3" />
                {:else}
                  <Trash2Icon />
                {/if}
              </Button>
            </div>
          </div>
          {#if expandedId === model.id && form}
            <div class="border-t bg-muted/30 p-3">
              {@render modelEditor(form)}
            </div>
          {/if}
        </div>
      {/each}
    {/if}
  </div>
</div>

<Dialog.Root
  bind:open={syncOpen}
  onOpenChange={(open) => {
    if (!open) resetSyncState();
  }}
>
  <Dialog.Content class="sm:max-w-2xl">
    <Dialog.Header>
      <Dialog.Title>{t('providers.sync.title')}</Dialog.Title>
      <Dialog.Description>
        {t('providers.sync.description')}
      </Dialog.Description>
    </Dialog.Header>

    <div class="grid gap-3 sm:grid-cols-2">
      <div class="grid gap-1.5">
        <Label for="sync-source">{t('providers.sync.sourceLabel')}</Label>
        <Select.Root
          type="single"
          bind:value={syncSource}
          items={syncSourceItems}
          onValueChange={changeSyncSource}
          disabled={syncApplying}
        >
          <Select.Trigger
            class="w-full"
            id="sync-source"
            aria-label={t('providers.sync.sourceLabel')}
          >
            <Select.Value placeholder={t('providers.sync.autoMatch')} />
          </Select.Trigger>
          <Select.Content>
            {#each syncSourceItems as option (option.value)}
              <Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
            {/each}
          </Select.Content>
        </Select.Root>
        <p class="text-[10.5px] text-muted-foreground">
          {t('providers.sync.sourceHint')}
        </p>
      </div>

      <div class="flex items-center justify-between gap-2 rounded-md border px-2 py-1.5">
        <div>
          <Label for="sync-overwrite" class="text-xs">{t('providers.sync.overwrite')}</Label>
          <p class="mt-0.5 text-[10.5px] text-muted-foreground">
            {t('providers.sync.overwriteHint')}
          </p>
        </div>
        <Switch
          id="sync-overwrite"
          checked={syncOverwrite}
          onCheckedChange={toggleOverwrite}
          disabled={syncLoading || syncApplying}
          aria-label={t('providers.sync.overwrite')}
        />
      </div>
    </div>

    {#if syncLoading}
      <div class="space-y-2 py-2">
        {#each Array(4) as _, i (i)}
          <Skeleton class="h-7 w-full" />
        {/each}
        <p class="text-center text-[10.5px] text-muted-foreground">
          {t('providers.sync.loading')}
        </p>
      </div>
    {:else if syncError}
      <Alert variant="destructive">
        <TriangleAlertIcon />
        <AlertDescription>{syncError}</AlertDescription>
      </Alert>
      <div class="flex justify-end">
        <Button variant="outline" size="sm" onclick={() => runSyncPreview()} disabled={syncApplying}>
          <RefreshCwIcon />
          {t('providers.sync.retry')}
        </Button>
      </div>
    {:else if syncPreview}
      <div class="flex flex-wrap items-center gap-1.5">
        {#each MODELSDEV_ACTIONS as action (action)}
          {#if Number(syncCounts[action]) > 0}
            {@const meta = modelsDevActionMeta(action)}
            <StatusBadge tone={meta.tone}>{meta.label} {Number(syncCounts[action])}</StatusBadge>
          {/if}
        {/each}
        <span class="text-[10.5px] text-muted-foreground">
          {t('providers.sync.summary', {
            count: syncItems.length,
            time: fmtTime(syncPreview.fetched_at)
          })}
        </span>
      </div>

      {#if syncItems.length}
        <div class="max-h-[45vh] overflow-y-auto rounded-md border">
          <div
            class="grid grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)_minmax(0,1fr)_minmax(0,0.9fr)_auto] gap-2 border-b bg-muted/50 px-2 py-1 text-[10px] font-medium text-muted-foreground"
          >
            <span>{t('providers.sync.columns.model')}</span>
            <span>{t('providers.sync.columns.currentPrice')}</span>
            <span>{t('providers.sync.columns.after')}</span>
            <span>{t('providers.sync.columns.source')}</span>
            <span>{t('providers.sync.columns.status')}</span>
          </div>
          {#each syncItems as item (item.id || item.model_id)}
            {@const meta = modelsDevActionMeta(item.action)}
            <div
              class="grid grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)_minmax(0,1fr)_minmax(0,0.9fr)_auto] items-center gap-2 border-b px-2 py-1 last:border-b-0"
            >
              <span class="truncate font-mono text-[10.5px]">{item.model_id}</span>
              <span class="truncate font-mono text-[10px] text-muted-foreground">
                {modelsDevCostText(item.old?.cost)}
              </span>
              <span class="truncate font-mono text-[10px]">
                {modelsDevCostText(item.new?.cost)}
              </span>
              <span class="truncate text-[10.5px] text-muted-foreground">
                {item.match?.provider_name || '—'}
              </span>
              <div class="flex justify-end">
                <StatusBadge tone={meta.tone}>{meta.label}</StatusBadge>
              </div>
              {#if item.error}
                <p class="col-span-5 text-[10px] text-destructive">{item.error}</p>
              {/if}
            </div>
          {/each}
        </div>
      {:else}
        <div
          class="rounded-md border border-dashed px-2 py-4 text-center text-[10.5px] text-muted-foreground"
        >
          {t('providers.sync.empty')}
        </div>
      {/if}
    {/if}

    <Dialog.Footer>
      <span class="mr-auto self-center text-[10.5px] text-muted-foreground">
        {#if syncPreview}
          {t('providers.sync.footerWithCount', { source: syncSourceLabel, count: syncUpdateCount })}
        {:else}
          {t('providers.sync.footer', { source: syncSourceLabel })}
        {/if}
      </span>
      <Button variant="outline" size="sm" onclick={closeSyncDialog} disabled={syncApplying}
        >{t('common.action.cancel')}</Button
      >
      <Button size="sm" onclick={applySync} disabled={!canApplySync}>
        {#if syncApplying}
          <Spinner class="size-3" />
          {t('providers.sync.applying')}
        {:else}
          {t('providers.sync.apply', { count: syncUpdateCount })}
        {/if}
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>
