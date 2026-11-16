<script>
  import { onMount } from 'svelte';
  import { api, fmtTime } from '$lib/api';
  import { t } from '$lib/i18n/index.svelte.js';
  import { formatPrice } from '$lib/i18n/format.js';

  import SearchIcon from '@lucide/svelte/icons/search';
  import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
  import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
  import LayersIcon from '@lucide/svelte/icons/layers';
  import RouteIcon from '@lucide/svelte/icons/route';
  import BoxIcon from '@lucide/svelte/icons/box';

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
  import { apiSchemaLabel, apiSchemaProtocols } from '$lib/api-schema.js';

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

  function priceText(model) {
    const cost = model.cost;
    if (!cost) return '';
    const input = Number(cost.input) || 0;
    const output = Number(cost.output) || 0;
    if (input === 0 && output === 0) return '';
    return `${formatPrice(input)} / ${formatPrice(output)}`;
  }

  function hasTiers(model) {
    return !!model.cost?.tiers?.length;
  }

  onMount(load);
</script>

<svelte:head>
  <title>{t('models.title')}</title>
</svelte:head>

<PageHeader title={t('models.heading')} description={t('models.description')}>
  <Button variant="outline" size="sm" onclick={load} disabled={loading}>
    <RefreshCwIcon class={loading ? 'animate-spin' : ''} />
    {t('common.action.refresh')}
  </Button>
</PageHeader>

<div class="grid gap-3 sm:grid-cols-3">
  <StatCard
    label={t('models.stats.all')}
    value={models.length}
    hint={t('models.stats.allHint')}
    icon={LayersIcon}
  />
  <StatCard
    label={t('models.stats.virtual')}
    value={virtualModelCount}
    hint={t('models.stats.virtualHint')}
    icon={RouteIcon}
  />
  <StatCard
    label={t('models.stats.provider')}
    value={providerCount}
    hint={t('models.stats.providerHint', { count: offlineCount })}
    icon={BoxIcon}
  />
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
            <Tabs.Trigger value="all">{t('models.tab.all')}</Tabs.Trigger>
            <Tabs.Trigger value="virtual_model">{t('models.tab.virtual')}</Tabs.Trigger>
            <Tabs.Trigger value="provider">{t('models.tab.provider')}</Tabs.Trigger>
          </Tabs.List>
        </Tabs.Root>
        <StatusBadge tone="neutral">{visible.length} / {models.length}</StatusBadge>
      </div>
      <div class="w-full sm:w-64">
        <InputGroup.Root>
          <InputGroup.Addon><SearchIcon /></InputGroup.Addon>
          <InputGroup.Input
            placeholder={t('models.searchPlaceholder')}
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
          <Table.Head>{t('models.column.modelId')}</Table.Head>
          <Table.Head>{t('models.column.source')}</Table.Head>
          <Table.Head>{t('models.column.providerName')}</Table.Head>
          <Table.Head>{t('models.column.protocol')}</Table.Head>
          <Table.Head class="text-right">{t('models.column.context')}</Table.Head>
          <Table.Head class="text-right">{t('models.column.price')}</Table.Head>
          <Table.Head>{t('models.column.status')}</Table.Head>
          <Table.Head>{t('models.column.createdAt')}</Table.Head>
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
                    {models.length ? t('models.empty.noMatchTitle') : t('models.empty.noneTitle')}
                  </Empty.Title>
                  <Empty.Description>
                    {models.length
                      ? t('models.empty.noMatchDescription')
                      : t('models.empty.noneDescription')}
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
                    <StatusBadge tone="violet">{t('models.source.virtual')}</StatusBadge>
                  {:else}
                    <StatusBadge tone="info">{t('models.source.provider')}</StatusBadge>
                    <span class="text-[10px] text-muted-foreground">
                      {model.source === 'discovered'
                        ? t('models.source.discovered')
                        : t('models.source.manual')}
                    </span>
                  {/if}
                </div>
              </Table.Cell>
              <Table.Cell class="max-w-[220px]">
                {#if model.kind === 'virtual_model'}
                  <div class="truncate text-[11px]">{model.virtual_model_name || model.id}</div>
                  <div class="text-[10px] text-muted-foreground">
                    {t('models.targets', { count: model.target_count })}
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
                  {#each apiSchemaProtocols(model.api_schema) as type (type)}
                    <StatusBadge tone="neutral">{apiSchemaLabel(type)}</StatusBadge>
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
                      {t('models.tiers', { count: model.cost.tiers.length })}
                    {:else}
                      {t('models.priceHint')}
                    {/if}
                  </div>
                {:else}
                  <span class="text-[11px] text-muted-foreground">—</span>
                {/if}
              </Table.Cell>
              <Table.Cell>
                {#if model.enabled}
                  <StatusBadge tone="success" dot>{t('models.status.available')}</StatusBadge>
                {:else if model.kind === 'provider' && !model.provider_enabled}
                  <StatusBadge tone="neutral">{t('models.status.providerDisabled')}</StatusBadge>
                {:else}
                  <StatusBadge tone="neutral">{t('models.status.disabled')}</StatusBadge>
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
