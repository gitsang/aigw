<script>
  import { toast } from 'svelte-sonner';
  import PlusIcon from '@lucide/svelte/icons/plus';
  import Trash2Icon from '@lucide/svelte/icons/trash-2';
  import PencilIcon from '@lucide/svelte/icons/pencil';
  import CheckIcon from '@lucide/svelte/icons/check';
  import XIcon from '@lucide/svelte/icons/x';
  import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
  import LayersIcon from '@lucide/svelte/icons/layers';
  import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';

  import { api } from '$lib/api';
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
  let editingId = $state(null);
  let form = $state(null);
  let saving = $state(false);
  let deleting = $state(null);
  let discovering = $state(false);

  const editing = $derived(form !== null);

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
    // Reload whenever the selected provider changes.
    providerId;
    load();
  });

  function openCreate() {
    editingId = null;
    form = blankModel();
  }

  function openEdit(model) {
    editingId = model.id;
    form = modelToForm(model);
  }

  function closeForm() {
    editingId = null;
    form = null;
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
      if (editingId) {
        await api(`/api/admin/providers/${providerId}/models/${editingId}`, {
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

  function addTier() {
    form.cost.tiers = [...form.cost.tiers, blankTier()];
  }

  function removeTier(index) {
    form.cost.tiers = form.cost.tiers.filter((_, i) => i !== index);
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

<div class="space-y-3">
  <div class="flex flex-wrap items-start justify-between gap-2">
    <div>
      <div class="text-xs font-medium">模型（{models.length}）</div>
      <p class="mt-0.5 text-[10.5px] text-muted-foreground">
        每个上游模型一行，可配置上下文窗口、最大输出与分级计费。
      </p>
    </div>
    <div class="flex items-center gap-1.5">
      <Button variant="outline" size="sm" onclick={discover} disabled={discovering || !providerId}>
        <RefreshCwIcon class={discovering ? 'animate-spin' : ''} />
        从 /models 同步
      </Button>
      <Button size="sm" onclick={openCreate} disabled={!providerId || editing}>
        <PlusIcon />
        添加模型
      </Button>
    </div>
  </div>

  {#if error}
    <Alert variant="destructive">
      <TriangleAlertIcon />
      <AlertDescription>{error}</AlertDescription>
    </Alert>
  {/if}

  {#if editing}
    <div class="space-y-3 rounded-lg border bg-muted/30 p-3">
      <div class="flex items-center justify-between">
        <span class="text-xs font-medium">
          {editingId ? '编辑模型' : '添加模型'}
        </span>
        <Button variant="ghost" size="xs" onclick={closeForm}>
          <XIcon />
          收起
        </Button>
      </div>

      <div class="grid gap-3 sm:grid-cols-2">
        <div class="grid gap-1.5">
          <Label for="m-id">模型 ID</Label>
          <Input
            id="m-id"
            class="font-mono text-[11px]"
            bind:value={form.model_id}
            placeholder="deepseek/deepseek-flash"
          />
        </div>
        <div class="grid gap-1.5">
          <Label for="m-name">名称</Label>
          <Input id="m-name" bind:value={form.display_name} placeholder="可选" />
        </div>
        <div class="grid gap-1.5">
          <Label for="m-ctx">上下文窗口</Label>
          <Input
            id="m-ctx"
            class="font-mono text-[11px]"
            bind:value={form.context_window}
            placeholder="256K / 1M / 131072"
          />
          <p class="text-[10.5px] text-muted-foreground">
            {#if parseTokenBudget(form.context_window) > 0}
              = {parseTokenBudget(form.context_window).toLocaleString('en-US')} tokens
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
            bind:value={form.max_output_tokens}
            placeholder="8K / 64K"
          />
          <p class="text-[10.5px] text-muted-foreground">
            {#if parseTokenBudget(form.max_output_tokens) > 0}
              = {parseTokenBudget(form.max_output_tokens).toLocaleString('en-US')} tokens
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
          <div class="w-24">
            <Input
              class="h-7 font-mono text-[11px] uppercase"
              bind:value={form.cost.currency}
              placeholder="USD"
              aria-label="币种"
            />
          </div>
        </div>

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
                bind:value={form.cost[key]}
              />
            </div>
          {/each}
        </div>

        <div class="space-y-1.5">
          <div class="flex items-center justify-between">
            <span class="text-[10.5px] font-medium text-muted-foreground">阶梯计价</span>
            <Button variant="ghost" size="xs" onclick={addTier}>
              <PlusIcon />
              添加阶梯
            </Button>
          </div>
          {#if form.cost.tiers.length}
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
              {#each form.cost.tiers as tier, index (index)}
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
                    onclick={() => removeTier(index)}
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
        <Switch bind:checked={form.enabled} />
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
        <div class="border-b px-3 py-2 last:border-b-0">
          <div class="flex flex-wrap items-start justify-between gap-2">
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
            <div class="flex shrink-0 items-center gap-1">
              <Button variant="ghost" size="xs" onclick={() => openEdit(model)} disabled={editing}>
                <PencilIcon />
              </Button>
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
        </div>
      {/each}
    {/if}
  </div>
</div>
