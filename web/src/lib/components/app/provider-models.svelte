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
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import {
    modelsDevActionMeta,
    modelsDevCostText,
    modelsDevFormPatch,
    modelsDevPriceSummary,
    modelsDevScoreMeta,
    modelsDevSourceLabel,
    lookupModelsDev,
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

  // Bulk models.dev price sync dialog state.
  let syncOpen = $state(false);
  let syncOverwrite = $state(false);
  let syncLoading = $state(false);
  let syncApplying = $state(false);
  let syncError = $state('');
  let syncPreview = $state(null);
  // Incremented on every preview request so late responses never overwrite the
  // state of a newer request (or a closed dialog).
  let syncRequest = 0;

  const syncItems = $derived(Array.isArray(syncPreview?.items) ? syncPreview.items : []);
  const syncCounts = $derived(syncPreview?.counts || {});
  const syncUpdateCount = $derived(Number(syncCounts.update) || 0);
  const canApplySync = $derived(
    !syncLoading && !syncApplying && !syncError && syncUpdateCount > 0
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
        toast.success('模型已更新');
      } else {
        await api(`/api/admin/providers/${providerId}/models`, {
          method: 'POST',
          body: JSON.stringify(payload)
        });
        toast.success('模型已添加');
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
      toast.success('模型已删除');
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
      toast.success(`已同步 ${res.discovered} 个模型`);
    } catch (e) {
      toast.error(`同步失败：${e.message}`);
    } finally {
      discovering = false;
    }
  }

  // ---- per-model models.dev sync -------------------------------------------

  /** Look up models.dev candidates for the model currently edited. */
  async function syncFromModelsDev(f) {
    const modelId = (f.model_id || '').trim();
    if (!modelId) {
      toast.error('请先填写模型 ID');
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
      if (matches.length === 0) {
        toast.error('models.dev 未收录该模型');
      } else if (matches.length === 1) {
        applyMatch(f, matches[0]);
      } else {
        lookupMatches = matches;
      }
    } catch (e) {
      if (form !== f || (f.model_id || '').trim() !== modelId) return;
      toast.error(`models.dev 查询失败：${e.message}`);
    } finally {
      if (form === f && (f.model_id || '').trim() === modelId) lookupBusy = false;
    }
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
    toast.success('已填入 models.dev 价格（USD / 1M），保存后生效');
  }

  // ---- bulk models.dev price sync dialog ----------------------------------

  function openSyncDialog() {
    if (!providerId) return;
    syncOverwrite = false;
    syncPreview = null;
    syncError = '';
    syncOpen = true;
    runSyncPreview();
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
  }

  /** Load the dry-run preview for the current overwrite flag. */
  async function runSyncPreview() {
    if (!providerId) return;
    syncLoading = true;
    syncError = '';
    const requestId = ++syncRequest;
    try {
      const res = await syncModelPrices(providerId, { dryRun: true, overwrite: syncOverwrite });
      if (requestId !== syncRequest) return;
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

  /** Apply the previewed changes (dry_run:false) and reload the model list. */
  async function applySync() {
    if (!canApplySync) return;
    syncApplying = true;
    try {
      const res = await syncModelPrices(providerId, { dryRun: false, overwrite: syncOverwrite });
      const counts = res?.counts || {};
      const updated = Number(counts.update ?? syncUpdateCount) || 0;
      toast.success(`已更新 ${updated} 个模型价格`);
      if (Number(counts.error) > 0) {
        toast.error(`${Number(counts.error)} 个模型写入失败，请检查后重试`);
      }
      closeSyncDialog();
      await load();
    } catch (e) {
      toast.error(`同步失败：${e.message}`);
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

  function money(value) {
    return Number(value || 0).toLocaleString('en-US', {
      minimumFractionDigits: 0,
      maximumFractionDigits: 6
    });
  }

  function priceSummary(cost) {
    const parts = [`in ${money(cost.input)}`, `out ${money(cost.output)}`];
    if (cost.cache_read) parts.push(`cr ${money(cost.cache_read)}`);
    if (cost.cache_write) parts.push(`cw ${money(cost.cache_write)}`);
    return `${cost.currency} ${parts.join(' / ')} · 1M`;
  }
</script>

{#snippet modelEditor(f)}
  <div class="space-y-3">
    <div class="flex items-center justify-between">
      <span class="text-xs font-medium">{creating ? '添加模型' : '编辑模型'}</span>
      <Button variant="ghost" size="icon-xs" onclick={closeForm} aria-label="收起编辑">
        <XIcon />
      </Button>
    </div>

    <div class="grid gap-3 sm:grid-cols-2">
      <div class="grid gap-1.5">
        <Label for="m-id">模型 ID</Label>
        <Input
          id="m-id"
          class="font-mono text-[11px]"
          bind:value={f.model_id}
          oninput={resetLookup}
          placeholder="deepseek/deepseek-flash"
        />
      </div>
      <div class="grid gap-1.5">
        <Label for="m-name">名称</Label>
        <Input id="m-name" bind:value={f.display_name} placeholder="可选" />
      </div>
      <div class="grid gap-1.5">
        <Label for="m-ctx">上下文窗口</Label>
        <Input
          id="m-ctx"
          class="font-mono text-[11px]"
          bind:value={f.context_window}
          placeholder="256K / 1M / 131072"
        />
        <p class="text-[10.5px] text-muted-foreground">
          {#if parseTokenBudget(f.context_window) > 0}
            = {parseTokenBudget(f.context_window).toLocaleString('en-US')} tokens
          {:else}
            1K = 1024，1M = 1024K
          {/if}
        </p>
      </div>
      <div class="grid gap-1.5">
        <Label for="m-max">最大输出</Label>
        <Input
          id="m-max"
          class="font-mono text-[11px]"
          bind:value={f.max_output_tokens}
          placeholder="8K / 64K"
        />
        <p class="text-[10.5px] text-muted-foreground">
          {#if parseTokenBudget(f.max_output_tokens) > 0}
            = {parseTokenBudget(f.max_output_tokens).toLocaleString('en-US')} tokens
          {:else}
            留空表示使用上游默认
          {/if}
        </p>
      </div>
    </div>

    <Separator />

    <div class="space-y-2">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div>
          <div class="text-xs font-medium">计费</div>
          <p class="mt-0.5 text-[10.5px] text-muted-foreground">
            每 100 万 tokens 的价格；阶梯超过阈值后整体替代基础价。
          </p>
        </div>
        <div class="flex items-center gap-1.5">
          <div class="w-24">
            <Input
              class="h-7 font-mono text-[11px] uppercase"
              bind:value={f.cost.currency}
              placeholder="USD"
              aria-label="币种"
            />
          </div>
          <Button
            variant="outline"
            size="xs"
            onclick={() => syncFromModelsDev(f)}
            disabled={lookupBusy}
            aria-label="从 models.dev 同步价格"
          >
            {#if lookupBusy}
              <Spinner class="size-2.5" />
              查询中…
            {:else}
              <SparklesIcon />
              从 models.dev 同步
            {/if}
          </Button>
        </div>
      </div>

      {#if appliedMatch}
        <div class="flex flex-wrap items-center gap-1.5 text-[10.5px] text-muted-foreground">
          <StatusBadge tone="success">已填入</StatusBadge>
          <span>来源：{modelsDevSourceLabel(appliedMatch)}</span>
          <span>· USD / 1M tokens</span>
          <span class="font-mono">{modelsDevPriceSummary(appliedMatch.cost)}</span>
        </div>
      {/if}

      {#if lookupMatches.length > 1}
        <div class="space-y-1.5 rounded-md border border-dashed p-2">
          <div class="text-[10.5px] font-medium text-muted-foreground">多个来源匹配，请选择</div>
          {#each lookupMatches as match (match.provider_id + '/' + match.model_id)}
            {@const score = modelsDevScoreMeta(match.score)}
            <button
              type="button"
              class="w-full rounded-md border px-2 py-1.5 text-left transition-colors hover:bg-muted/60 focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:outline-none"
              onclick={() => applyMatch(f, match)}
              aria-label={`应用 ${match.provider_name || match.provider_id} 的价格`}
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
                  <span>上下文 {formatTokenBudget(match.context_window)}</span>
                {/if}
                {#if match.max_output_tokens}
                  <span>输出 {formatTokenBudget(match.max_output_tokens)}</span>
                {/if}
              </div>
            </button>
          {/each}
          {#if lookupFetchedAt}
            <p class="text-[10px] text-muted-foreground">数据快照：{fmtTime(lookupFetchedAt)}</p>
          {/if}
        </div>
      {/if}

      <div class="grid gap-2 sm:grid-cols-4">
        {#each [
          ['input', '输入'],
          ['output', '输出'],
          ['cache_read', '缓存读取'],
          ['cache_write', '缓存写入']
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
          <span class="text-[10.5px] font-medium text-muted-foreground">阶梯计价</span>
          <Button variant="ghost" size="xs" onclick={() => addTier(f)}>
            <PlusIcon />
            添加阶梯
          </Button>
        </div>
        {#if f.cost.tiers.length}
          <div class="overflow-hidden rounded-md border">
            <div
              class="grid grid-cols-[minmax(0,1.2fr)_repeat(4,minmax(0,1fr))_28px] gap-2 border-b bg-muted/50 px-2 py-1 text-[10px] font-medium text-muted-foreground"
            >
              <span>输入 &gt;</span>
              <span>输入</span>
              <span>输出</span>
              <span>缓存读</span>
              <span>缓存写</span>
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
                  aria-label={`阶梯 ${index + 1} 阈值`}
                />
                {#each ['input', 'output', 'cache_read', 'cache_write'] as key (key)}
                  <Input
                    class="h-7 font-mono text-[11px]"
                    type="number"
                    min="0"
                    step="0.000001"
                    bind:value={tier[key]}
                    aria-label={`阶梯 ${index + 1} ${key}`}
                  />
                {/each}
                <Button
                  variant="ghost"
                  size="icon-xs"
                  class="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                  onclick={() => removeTier(f, index)}
                  aria-label="删除阶梯"
                >
                  <XIcon />
                </Button>
              </div>
            {/each}
          </div>
        {:else}
          <div class="rounded-md border border-dashed px-2 py-2 text-center text-[10.5px] text-muted-foreground">
            未配置阶梯，全部请求使用基础价
          </div>
        {/if}
      </div>
    </div>

    <div class="flex items-center justify-between gap-2 rounded-md border px-2 py-1.5">
      <span class="text-xs">启用该模型</span>
      <Switch bind:checked={f.enabled} />
    </div>

    <div class="flex justify-end gap-1.5">
      <Button variant="ghost" size="sm" onclick={closeForm} disabled={saving}>取消</Button>
      <Button size="sm" onclick={save} disabled={saving}>
        {#if saving}
          <Spinner class="size-3.5" />
          保存中…
        {:else}
          <CheckIcon />
          保存
        {/if}
      </Button>
    </div>
  </div>
{/snippet}

<div class="space-y-3">
  <div class="flex flex-wrap items-start justify-between gap-2">
    <div>
      <div class="text-xs font-medium">模型（{models.length}）</div>
      <p class="mt-0.5 text-[10.5px] text-muted-foreground">
        每个上游模型一行，点击一行展开编辑；可配置上下文窗口、最大输出与分级计费。
      </p>
    </div>
    <div class="flex items-center gap-1.5">
      <Button variant="outline" size="sm" onclick={discover} disabled={discovering || !providerId}>
        <RefreshCwIcon class={discovering ? 'animate-spin' : ''} />
        从 /models 同步
      </Button>
      <Button
        variant="outline"
        size="sm"
        onclick={openSyncDialog}
        disabled={!providerId || !models.length}
        title="从 models.dev 预览并同步全部模型价格"
      >
        <CloudDownloadIcon />
        同步价格
      </Button>
      <Button size="sm" onclick={openCreate} disabled={!providerId}>
        <PlusIcon />
        {creating ? '收起' : '添加模型'}
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
          <Empty.Title>还没有模型</Empty.Title>
          <Empty.Description>从上游 /models 同步，或手动添加一行。</Empty.Description>
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
                    {model.source === 'discovered' ? '自动同步' : '手动'}
                  </StatusBadge>
                  {#if !model.enabled}
                    <StatusBadge tone="neutral">停用</StatusBadge>
                  {/if}
                </div>
                <div class="flex flex-wrap items-center gap-x-2 gap-y-1 text-[10.5px] text-muted-foreground">
                  {#if model.context_window}
                    <span>上下文 {formatTokenBudget(model.context_window)}</span>
                  {/if}
                  {#if model.max_output_tokens}
                    <span>输出 {formatTokenBudget(model.max_output_tokens)}</span>
                  {/if}
                  <span class="font-mono">{priceSummary(model.cost || {})}</span>
                  {#if model.cost?.tiers?.length}
                    <StatusBadge tone="violet">
                      {model.cost.tiers.length} 个阶梯
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
                aria-label="删除模型"
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
      <Dialog.Title>同步 models.dev 价格</Dialog.Title>
      <Dialog.Description>
        先预览将从 models.dev 拉取的价格，点击应用后才会写入。价格为 USD / 100 万 tokens。
      </Dialog.Description>
    </Dialog.Header>

    <div class="flex items-center justify-between gap-2 rounded-md border px-2 py-1.5">
      <div>
        <Label for="sync-overwrite" class="text-xs">覆盖已有价格</Label>
        <p class="mt-0.5 text-[10.5px] text-muted-foreground">
          关闭时只填充空价格，已有价格保持不变。
        </p>
      </div>
      <Switch
        id="sync-overwrite"
        checked={syncOverwrite}
        onCheckedChange={toggleOverwrite}
        disabled={syncLoading || syncApplying}
        aria-label="覆盖已有价格"
      />
    </div>

    {#if syncLoading}
      <div class="space-y-2 py-2">
        {#each Array(4) as _, i (i)}
          <Skeleton class="h-7 w-full" />
        {/each}
        <p class="text-center text-[10.5px] text-muted-foreground">正在从 models.dev 拉取价格…</p>
      </div>
    {:else if syncError}
      <Alert variant="destructive">
        <TriangleAlertIcon />
        <AlertDescription>{syncError}</AlertDescription>
      </Alert>
      <div class="flex justify-end">
        <Button variant="outline" size="sm" onclick={runSyncPreview} disabled={syncApplying}>
          <RefreshCwIcon />
          重试
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
          共 {syncItems.length} 个模型 · 快照 {fmtTime(syncPreview.fetched_at)}
        </span>
      </div>

      {#if syncItems.length}
        <div class="max-h-[45vh] overflow-y-auto rounded-md border">
          <div
            class="grid grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)_minmax(0,1fr)_minmax(0,0.9fr)_auto] gap-2 border-b bg-muted/50 px-2 py-1 text-[10px] font-medium text-muted-foreground"
          >
            <span>模型</span>
            <span>当前价格</span>
            <span>同步后</span>
            <span>来源</span>
            <span>状态</span>
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
          该 Provider 下没有模型。
        </div>
      {/if}
    {/if}

    <Dialog.Footer>
      <span class="mr-auto self-center text-[10.5px] text-muted-foreground">
        {#if syncPreview}
          将更新 {syncUpdateCount} 项
        {:else}
          —
        {/if}
      </span>
      <Button variant="outline" size="sm" onclick={closeSyncDialog} disabled={syncApplying}>
        取消
      </Button>
      <Button size="sm" onclick={applySync} disabled={!canApplySync}>
        {#if syncApplying}
          <Spinner class="size-3" />
          应用中…
        {:else}
          应用 {syncUpdateCount} 项
        {/if}
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>
