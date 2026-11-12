<script>
  import { onMount } from 'svelte';
  import { api, fmtTime } from '$lib/api';

  import SearchIcon from '@lucide/svelte/icons/search';
  import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
  import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
  import LayersIcon from '@lucide/svelte/icons/layers';
  import RouteIcon from '@lucide/svelte/icons/route';
  import BoxIcon from '@lucide/svelte/icons/box';
  import PowerIcon from '@lucide/svelte/icons/power';

  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import * as Table from '$lib/components/ui/table/index.js';
  import * as Tabs from '$lib/components/ui/tabs/index.js';
  import * as Empty from '$lib/components/ui/empty/index.js';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import { Alert, AlertDescription } from '$lib/components/ui/alert/index.js';
  import * as InputGroup from '$lib/components/ui/input-group/index.js';

  import PageHeader from '$lib/components/app/page-header.svelte';
  import StatCard from '$lib/components/app/stat-card.svelte';
  import StatusBadge from '$lib/components/app/status-badge.svelte';

  import { formatTokenBudget } from '$lib/model-form.js';

  const PROTOCOL_NAMES = {
    'chat.completions': 'Chat',
    responses: 'Responses',
    messages: 'Messages'
  };
  const apiLabel = (value) => PROTOCOL_NAMES[value] || value;

  let models = $state([]);
  let loading = $state(true);
  let error = $state('');
  let search = $state('');
  let kind = $state('all');

  const virtualModelCount = $derived(models.filter((model) => model.kind === 'virtual_model').length);
  const providerCount = $derived(models.filter((model) => model.kind === 'provider').length);
  const offlineCount = $derived(models.filter((model) => !model.enabled).length);

  const visible = $derived(
    models.filter((model) => {
      if (kind === 'virtual_model' && model.kind !== 'virtual_model') return false;
      if (kind === 'provider' && model.kind !== 'provider') return false;
      const q = search.trim().toLowerCase();
      if (!q) return true;
      return [
        model.id,
        model.name,
        model.virtual_model_name,
        model.provider_name,
        model.provider_slug,
        model.model_id,
        model.display_name
      ].some((value) => (value || '').toLowerCase().includes(q));
    })
  );

  async function load() {
    loading = true;
    error = '';
    try {
      models = await api('/api/admin/models');
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function money(value) {
    return Number(value || 0).toLocaleString('en-US', {
      minimumFractionDigits: 0,
      maximumFractionDigits: 6
    });
  }

  function priceText(model) {
    const cost = model.cost;
    if (!cost) return '';
    const input = Number(cost.input) || 0;
    const output = Number(cost.output) || 0;
    if (input === 0 && output === 0) return '';
    return `${money(input)} / ${money(output)}`;
  }

  function hasTiers(model) {
    return !!model.cost?.tiers?.length;
  }

  onMount(load);
</script>

<svelte:head>
  <title>Models · aigw</title>
</svelte:head>

<PageHeader
  title="Models"
  description="网关可用的全部模型：Virtual Model 页创建的虚拟模型，以及 Provider 提供的模型（对外 ID 为 provider/model）。此页是只读目录，创建与编辑请到 Virtual Model 或 Model Provider。"
>
  <Button variant="outline" size="sm" onclick={load} disabled={loading}>
    <RefreshCwIcon class={loading ? 'animate-spin' : ''} />
    刷新
  </Button>
</PageHeader>

<div class="grid gap-3 sm:grid-cols-3">
  <StatCard label="全部模型" value={models.length} hint="虚拟模型 + Provider 模型" icon={LayersIcon} />
  <StatCard label="虚拟模型" value={virtualModelCount} hint="客户端请求时可用的模型名" icon={RouteIcon} />
  <StatCard label="Provider 模型" value={providerCount} hint={`${offlineCount} 个不可用`} icon={BoxIcon} />
</div>

{#if error}
  <Alert variant="destructive">
    <TriangleAlertIcon />
    <AlertDescription>{error}</AlertDescription>
  </Alert>
{/if}

<Card.Root class="gap-0">
  <Card.Header class="border-b">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div class="flex flex-wrap items-center gap-2">
        <Tabs.Root bind:value={kind}>
          <Tabs.List>
            <Tabs.Trigger value="all">全部</Tabs.Trigger>
            <Tabs.Trigger value="virtual_model">虚拟模型</Tabs.Trigger>
            <Tabs.Trigger value="provider">Provider 模型</Tabs.Trigger>
          </Tabs.List>
        </Tabs.Root>
        <StatusBadge tone="neutral">{visible.length} / {models.length}</StatusBadge>
      </div>
      <div class="w-full sm:w-64">
        <InputGroup.Root>
          <InputGroup.Addon><SearchIcon /></InputGroup.Addon>
          <InputGroup.Input
            placeholder="搜索模型 ID / 名称 / Provider"
            bind:value={search}
            class="text-[11px]"
          />
        </InputGroup.Root>
      </div>
    </div>
  </Card.Header>
  <Card.Content class="px-0">
    <Table.Root>
      <Table.Header>
        <Table.Row>
          <Table.Head>模型 ID</Table.Head>
          <Table.Head>来源</Table.Head>
          <Table.Head>Provider / 名称</Table.Head>
          <Table.Head>接口</Table.Head>
          <Table.Head class="text-right">上下文</Table.Head>
          <Table.Head class="text-right">价格 / 1M</Table.Head>
          <Table.Head>状态</Table.Head>
          <Table.Head>创建时间</Table.Head>
        </Table.Row>
      </Table.Header>
      <Table.Body>
        {#if loading}
          {#each Array(6) as _, i (i)}
            <Table.Row>
              {#each Array(8) as __, j (j)}
                <Table.Cell><Skeleton class="h-4 w-full" /></Table.Cell>
              {/each}
            </Table.Row>
          {/each}
        {:else if !visible.length}
          <Table.Row class="hover:bg-transparent">
            <Table.Cell colspan={8}>
              <Empty.Root class="border-0 py-12">
                <Empty.Header>
                  <Empty.Media variant="icon"><LayersIcon /></Empty.Media>
                  <Empty.Title>
                    {models.length ? '没有匹配的模型' : '还没有可用模型'}
                  </Empty.Title>
                  <Empty.Description>
                    {models.length
                      ? '调整搜索词或来源筛选后重试。'
                      : '先在 Model Provider 添加 Provider 与模型，或到 Virtual Model 页创建虚拟模型。'}
                  </Empty.Description>
                </Empty.Header>
              </Empty.Root>
            </Table.Cell>
          </Table.Row>
        {:else}
          {#each visible as model (model.kind + ':' + model.id)}
            <Table.Row>
              <Table.Cell class="max-w-[320px]">
                <div class="truncate font-mono text-[11.5px] font-medium">{model.id}</div>
                {#if model.name && model.name !== model.id}
                  <div class="truncate text-[10.5px] text-muted-foreground">{model.name}</div>
                {/if}
              </Table.Cell>
              <Table.Cell>
                <div class="flex flex-wrap items-center gap-1">
                  {#if model.kind === 'virtual_model'}
                    <StatusBadge tone="violet">虚拟模型</StatusBadge>
                  {:else}
                    <StatusBadge tone="info">Provider</StatusBadge>
                    <span class="text-[10px] text-muted-foreground">
                      {model.source === 'discovered' ? '自动同步' : '手动'}
                    </span>
                  {/if}
                </div>
              </Table.Cell>
              <Table.Cell class="max-w-[220px]">
                {#if model.kind === 'virtual_model'}
                  <div class="truncate text-[11px]">{model.virtual_model_name || model.id}</div>
                  <div class="text-[10px] text-muted-foreground">
                    {model.target_count} 个 target
                  </div>
                {:else}
                  <div class="truncate text-[11px]">{model.provider_name}</div>
                  <div class="truncate font-mono text-[10px] text-muted-foreground">
                    {model.provider_slug || model.provider_name}
                  </div>
                {/if}
              </Table.Cell>
              <Table.Cell>
                <div class="flex flex-wrap gap-1">
                  {#each model.api_types || [] as type (type)}
                    <StatusBadge tone="neutral">{apiLabel(type)}</StatusBadge>
                  {/each}
                </div>
              </Table.Cell>
              <Table.Cell class="whitespace-nowrap text-right font-mono text-[11px] tabular-nums">
                {model.kind === 'provider' && model.context_window
                  ? formatTokenBudget(model.context_window)
                  : '—'}
              </Table.Cell>
              <Table.Cell class="whitespace-nowrap text-right">
                {#if model.kind === 'provider' && priceText(model)}
                  <div class="font-mono text-[11px] tabular-nums">
                    {model.cost.currency} {priceText(model)}
                  </div>
                  <div class="text-[10px] text-muted-foreground">
                    {#if hasTiers(model)}
                      含 {model.cost.tiers.length} 个阶梯
                    {:else}
                      输入 / 输出
                    {/if}
                  </div>
                {:else}
                  <span class="text-[11px] text-muted-foreground">—</span>
                {/if}
              </Table.Cell>
              <Table.Cell>
                {#if model.enabled}
                  <StatusBadge tone="success" dot>可用</StatusBadge>
                {:else if model.kind === 'provider' && !model.provider_enabled}
                  <StatusBadge tone="neutral">Provider 停用</StatusBadge>
                {:else}
                  <StatusBadge tone="neutral">停用</StatusBadge>
                {/if}
              </Table.Cell>
              <Table.Cell class="whitespace-nowrap text-[11px] tabular-nums text-muted-foreground">
                {fmtTime(model.created_at)}
              </Table.Cell>
            </Table.Row>
          {/each}
        {/if}
      </Table.Body>
    </Table.Root>
  </Card.Content>
</Card.Root>
