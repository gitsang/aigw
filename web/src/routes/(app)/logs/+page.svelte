<script>
  import { onMount } from 'svelte';
  import { AreaChart } from 'layerchart';
  import { scaleTime } from 'd3-scale';

  import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
  import ArrowRightIcon from '@lucide/svelte/icons/arrow-right';
  import ActivityIcon from '@lucide/svelte/icons/activity';
  import CoinsIcon from '@lucide/svelte/icons/coins';
  import GaugeIcon from '@lucide/svelte/icons/gauge';
  import HashIcon from '@lucide/svelte/icons/hash';
  import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
  import SearchIcon from '@lucide/svelte/icons/search';
  import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
  import InboxIcon from '@lucide/svelte/icons/inbox';

  import { api, fmtCost, fmtLatency, fmtNumber, fmtTime, fmtTimeOfDay } from '$lib/api';
  import { formatHourMinute } from '$lib/i18n/format.js';
  import { t } from '$lib/i18n/index.svelte.js';
  import { apiSchemaLabel, API_PROTOCOLS } from '$lib/api-schema.js';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import * as Table from '$lib/components/ui/table/index.js';
  import * as Tabs from '$lib/components/ui/tabs/index.js';
  import * as Select from '$lib/components/ui/select/index.js';
  import * as Chart from '$lib/components/ui/chart/index.js';
  import * as Empty from '$lib/components/ui/empty/index.js';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import { Progress } from '$lib/components/ui/progress/index.js';
  import { Alert, AlertDescription } from '$lib/components/ui/alert/index.js';
  import * as InputGroup from '$lib/components/ui/input-group/index.js';

  import PageHeader from '$lib/components/app/page-header.svelte';
  import StatCard from '$lib/components/app/stat-card.svelte';
  import StatusBadge from '$lib/components/app/status-badge.svelte';

  const RANGES = [
    { value: '1h', labelKey: 'logs.range.1h' },
    { value: '24h', labelKey: 'logs.range.24h' },
    { value: '7d', labelKey: 'logs.range.7d' },
    { value: '30d', labelKey: 'logs.range.30d' }
  ];

  // Protocol filter. The stored value stays the raw backend protocol
  // (`chat.completions`, …) while the label mirrors the rest of the console, so
  // the closed trigger, the open option list and the table badge all agree.
  const API_TYPE_ITEMS = $derived([
    { value: '', label: t('logs.table.allApis') },
    ...API_PROTOCOLS.map((protocol) => ({ value: protocol.value, label: protocol.label }))
  ]);

  const chartConfig = $derived({
    requests: { label: t('logs.chart.requests'), color: 'var(--chart-2)' }
  });

  let range = $state('24h');
  let stats = $state(null);
  let logs = $state([]);
  let total = $state(0);
  let limit = $state(50);
  let offset = $state(0);
  let loading = $state(false);
  let statsLoading = $state(false);
  let error = $state('');
  let filters = $state({ search: '', api_type: '', model: '', app: '', status: '' });
  let lastUpdated = $state(null);

  const successRate = $derived(
    stats?.total_requests
      ? ((stats.success_requests / stats.total_requests) * 100).toFixed(1)
      : '0.0'
  );
  const pages = $derived(Math.max(1, Math.ceil(total / limit)));
  const page = $derived(Math.floor(offset / limit) + 1);
  const activeRange = $derived(RANGES.find((item) => item.value === range) ?? RANGES[1]);
  const series = $derived(
    (stats?.recent_series || []).map((point) => ({
      date: new Date(point.time),
      requests: point.requests
    }))
  );
  const modelRows = $derived(
    Object.entries(stats?.requests_by_model || {})
      .sort((a, b) => b[1] - a[1])
      .slice(0, 6)
  );
  const apiRows = $derived(Object.entries(stats?.requests_by_api || {}));
  const maxModel = $derived(Math.max(1, ...modelRows.map(([, count]) => count)));

  async function loadStats() {
    statsLoading = true;
    try {
      stats = await api(`/api/admin/stats?range=${range}`);
    } catch (e) {
      error = e.message;
    } finally {
      statsLoading = false;
    }
  }

  async function loadLogs() {
    loading = true;
    error = '';
    try {
      const q = new URLSearchParams({ limit: String(limit), offset: String(offset) });
      for (const [k, v] of Object.entries(filters)) if (v) q.set(k, v);
      const data = await api(`/api/admin/logs?${q}`);
      logs = data.items || [];
      total = data.total || 0;
      lastUpdated = new Date();
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function refresh() {
    return Promise.all([loadStats(), loadLogs()]);
  }

  function applyFilters(event) {
    event?.preventDefault?.();
    offset = 0;
    loadLogs();
  }

  function resetFilters() {
    filters = { search: '', api_type: '', model: '', app: '', status: '' };
    offset = 0;
    loadLogs();
  }

  function goPage(next) {
    offset = Math.max(0, Math.min(offset + next * limit, (pages - 1) * limit));
    loadLogs();
  }

  onMount(() => {
    loadLogs();
    const timer = setInterval(refresh, 15000);
    return () => clearInterval(timer);
  });

  $effect(() => {
    // Reload aggregates whenever the selected window changes (also runs once on mount).
    range;
    loadStats();
  });

  const isOk = (code) => code >= 200 && code < 400;
</script>

<svelte:head>
  <title>{t('logs.title')}</title>
</svelte:head>

<PageHeader
  title={t('logs.heading')}
  description={t('logs.description')}
>
  <span class="flex items-center gap-1.5 text-[11px] text-muted-foreground">
    <span class="size-1.5 rounded-full bg-emerald-500 shadow-[0_0_0_3px] shadow-emerald-500/20"
    ></span>
    {lastUpdated
      ? t('logs.updatedAt', { time: fmtTimeOfDay(lastUpdated) })
      : t('common.state.loading')}
  </span>
  <Button variant="outline" size="sm" onclick={refresh} disabled={loading}>
    <RefreshCwIcon class={loading ? 'animate-spin' : ''} />
    {t('common.action.refresh')}
  </Button>
</PageHeader>

{#if error}
  <Alert variant="destructive">
    <TriangleAlertIcon />
    <AlertDescription>{error}</AlertDescription>
  </Alert>
{/if}

<div class="flex flex-wrap items-center justify-between gap-2">
  <Tabs.Root bind:value={range} class="w-fit">
    <Tabs.List>
      {#each RANGES as item (item.value)}
        <Tabs.Trigger value={item.value} class="px-2.5">{t(item.labelKey)}</Tabs.Trigger>
      {/each}
    </Tabs.List>
  </Tabs.Root>
  <div class="flex items-center gap-1.5">
    {#each apiRows as [name, count] (name)}
      <StatusBadge tone="info">{name} · {fmtNumber(count)}</StatusBadge>
    {/each}
  </div>
</div>

<section class="grid grid-cols-2 gap-2.5 md:grid-cols-3 xl:grid-cols-5">
  <StatCard
    label={t('logs.stats.totalRequests')}
    value={fmtNumber(stats?.total_requests)}
    hint={t('logs.stats.successRate', { rate: successRate })}
    icon={ActivityIcon}
  />
  <StatCard
    label={t('logs.stats.totalTokens')}
    value={fmtNumber(stats?.total_tokens)}
    hint={t('logs.stats.tokenBreakdown', {
      input: fmtNumber(stats?.prompt_tokens),
      output: fmtNumber(stats?.completion_tokens)
    })}
    icon={HashIcon}
  />
  <StatCard
    label={t('logs.stats.totalCost')}
    value={fmtCost(stats?.total_cost)}
    hint={t('logs.stats.costHint')}
    icon={CoinsIcon}
  />
  <StatCard
    label={t('logs.stats.avgLatency')}
    value={fmtLatency(stats?.avg_latency_ms)}
    hint={t('logs.stats.p95', { value: fmtLatency(stats?.p95_latency_ms) })}
    icon={GaugeIcon}
  />
  <StatCard
    label={t('logs.stats.failedRequests')}
    value={fmtNumber(stats?.failed_requests)}
    hint={stats?.failed_requests ? t('logs.stats.failedHint') : t('logs.stats.allSucceeded')}
    icon={TriangleAlertIcon}
  />
</section>

<section class="grid gap-2.5 lg:grid-cols-[1.55fr_1fr]">
  <Card.Root class="gap-2">
    <Card.Header>
      <div class="flex items-start justify-between gap-2">
        <div class="space-y-0.5">
          <Card.Title class="text-sm">{t('logs.trend.title')}</Card.Title>
          <Card.Description class="text-[11px]">{t('logs.trend.description')}</Card.Description>
        </div>
        <StatusBadge tone="primary">{t(activeRange.labelKey)}</StatusBadge>
      </div>
    </Card.Header>
    <Card.Content>
      {#if statsLoading && !series.length}
        <Skeleton class="h-[210px] w-full" />
      {:else if series.length}
        <Chart.Container config={chartConfig} class="aspect-auto h-[210px] w-full">
          <AreaChart
            data={series}
            x="date"
            xScale={scaleTime()}
            series={[{ key: 'requests', label: t('logs.chart.requests'), color: 'var(--chart-2)' }]}
            axis="x"
            props={{
              area: { fillOpacity: 0.3, line: { class: 'stroke-1' }, motion: 'tween' },
              xAxis: {
                format: (v) => formatHourMinute(v)
              }
            }}
          >
            {#snippet tooltip()}
              <Chart.Tooltip
                indicator="line"
                labelFormatter={(v) => fmtTime(v)}
                formatter={(value) =>
                  t('logs.chart.requestCount', { count: fmtNumber(value) })}
              />
            {/snippet}
          </AreaChart>
        </Chart.Container>
      {:else}
        <Empty.Root class="h-[210px] border">
          <Empty.Header>
            <Empty.Media variant="icon"><InboxIcon /></Empty.Media>
            <Empty.Title>{t('logs.trend.emptyTitle')}</Empty.Title>
            <Empty.Description>{t('logs.trend.emptyDescription')}</Empty.Description>
          </Empty.Header>
        </Empty.Root>
      {/if}
    </Card.Content>
  </Card.Root>

  <Card.Root class="gap-2">
    <Card.Header>
      <div class="space-y-0.5">
        <Card.Title class="text-sm">{t('logs.models.title')}</Card.Title>
        <Card.Description class="text-[11px]">{t('logs.models.description')}</Card.Description>
      </div>
    </Card.Header>
    <Card.Content>
      {#if modelRows.length}
        <div class="space-y-2.5">
          {#each modelRows as [model, count] (model)}
            <div class="space-y-1">
              <div class="flex items-baseline justify-between gap-2">
                <span class="truncate font-mono text-[11px] text-foreground">{model}</span>
                <span class="shrink-0 text-[11px] tabular-nums text-muted-foreground">
                  {fmtNumber(count)}
                </span>
              </div>
              <Progress value={(count / maxModel) * 100} class="h-1.5" />
            </div>
          {/each}
        </div>
      {:else}
        <Empty.Root class="border py-8">
          <Empty.Header>
            <Empty.Media variant="icon"><ActivityIcon /></Empty.Media>
            <Empty.Title>{t('logs.models.emptyTitle')}</Empty.Title>
          </Empty.Header>
        </Empty.Root>
      {/if}
    </Card.Content>
  </Card.Root>
</section>

<Card.Root class="gap-3">
  <Card.Header class="gap-3">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div class="flex items-center gap-2">
        <Card.Title class="text-sm">{t('logs.table.title')}</Card.Title>
        <StatusBadge tone="neutral">{t('logs.table.totalCount', { count: fmtNumber(total) })}</StatusBadge>
      </div>
      <span class="text-[11px] text-muted-foreground">
        {t('logs.table.pageInfo', { page, pages, limit })}
      </span>
    </div>

    <form class="grid gap-2 md:grid-cols-2 xl:grid-cols-[minmax(200px,1.6fr)_minmax(150px,1fr)_minmax(120px,1fr)_minmax(120px,1fr)_auto_auto]" onsubmit={applyFilters}>
      <InputGroup.Root>
        <InputGroup.Addon><SearchIcon /></InputGroup.Addon>
        <InputGroup.Input
          placeholder={t('logs.table.searchPlaceholder')}
          bind:value={filters.search}
        />
      </InputGroup.Root>

      <Select.Root type="single" bind:value={filters.api_type} items={API_TYPE_ITEMS}>
        <Select.Trigger class="w-full">
          <Select.Value placeholder={t('logs.table.allApis')} />
        </Select.Trigger>
        <Select.Content>
          {#each API_TYPE_ITEMS as option (option.value)}
            <Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
          {/each}
        </Select.Content>
      </Select.Root>

      <Input placeholder="model" bind:value={filters.model} />
      <Input placeholder="app" bind:value={filters.app} />

      <Button type="submit" disabled={loading}>{t('common.action.search')}</Button>
      <Button type="button" variant="ghost" onclick={resetFilters}>
        {t('common.action.reset')}
      </Button>
    </form>
  </Card.Header>

  <Card.Content class="px-0">
    <Table.Root>
      <Table.Header>
        <Table.Row>
          <Table.Head>{t('logs.table.column.time')}</Table.Head>
          <Table.Head>{t('logs.table.column.model')}</Table.Head>
          <Table.Head>{t('logs.table.column.protocol')}</Table.Head>
          <Table.Head>{t('logs.table.column.caller')}</Table.Head>
          <Table.Head>{t('logs.table.column.providerAndModel')}</Table.Head>
          <Table.Head class="text-right">{t('logs.table.column.latency')}</Table.Head>
          <Table.Head class="text-right">{t('logs.table.column.tokens')}</Table.Head>
          <Table.Head class="text-right">{t('logs.table.column.cost')}</Table.Head>
          <Table.Head>{t('logs.table.column.status')}</Table.Head>
          <Table.Head>{t('logs.table.column.attempts')}</Table.Head>
        </Table.Row>
      </Table.Header>
      <Table.Body>
        {#if loading && !logs.length}
          {#each Array(6) as _, i (i)}
            <Table.Row>
              {#each Array(10) as __, j (j)}
                <Table.Cell><Skeleton class="h-4 w-full" /></Table.Cell>
              {/each}
            </Table.Row>
          {/each}
        {:else if !logs.length}
          <Table.Row class="hover:bg-transparent">
            <Table.Cell colspan={10}>
              <Empty.Root class="border-0 py-10">
                <Empty.Header>
                  <Empty.Media variant="icon"><InboxIcon /></Empty.Media>
                  <Empty.Title>{t('logs.table.emptyTitle')}</Empty.Title>
                  <Empty.Description>
                    {t('logs.table.emptyDescription')}
                  </Empty.Description>
                </Empty.Header>
              </Empty.Root>
            </Table.Cell>
          </Table.Row>
        {:else}
          {#each logs as log (log.id)}
            <Table.Row>
              <Table.Cell class="whitespace-nowrap font-mono text-[11px] text-muted-foreground">
                {fmtTime(log.time)}
              </Table.Cell>
              <Table.Cell class="max-w-[190px]">
                <div class="truncate font-mono text-[11px] font-medium">{log.model}</div>
                {#if log.upstream_model && log.upstream_model !== log.model}
                  <div class="truncate font-mono text-[10px] text-muted-foreground">
                    → {log.upstream_model}
                  </div>
                {/if}
              </Table.Cell>
              <Table.Cell>
                <StatusBadge tone="info">{apiSchemaLabel(log.api_type) || '—'}</StatusBadge>
              </Table.Cell>
              <Table.Cell class="max-w-[130px]">
                <div class="truncate text-[11px]">{log.app || '—'}</div>
                <div class="truncate text-[10px] text-muted-foreground">
                  {log.api_key_name || '—'}
                </div>
              </Table.Cell>
              <Table.Cell class="max-w-[150px]">
                <div class="truncate text-[11px]">{log.provider_name || '—'}</div>
                <div class="truncate text-[10px] text-muted-foreground">
                  {log.virtual_model_name || '—'}
                  {#if log.fallback_used}<span class="text-amber-600 dark:text-amber-400">· {t('logs.table.fallback')}</span>{/if}
                </div>
              </Table.Cell>
              <Table.Cell class="text-right font-mono text-[11px] tabular-nums">
                {fmtLatency(log.latency_ms)}
              </Table.Cell>
              <Table.Cell class="text-right font-mono text-[11px] tabular-nums">
                <div>{fmtNumber(log.total_tokens)}</div>
                <div class="text-[10px] text-muted-foreground">
                  {fmtNumber(log.prompt_tokens)} / {fmtNumber(log.completion_tokens)}
                </div>
              </Table.Cell>
              <Table.Cell class="text-right font-mono text-[11px] tabular-nums">
                {fmtCost(log.cost)}
              </Table.Cell>
              <Table.Cell>
                <StatusBadge tone={isOk(log.status_code) ? 'success' : 'danger'} dot>
                  {log.status_code}
                </StatusBadge>
              </Table.Cell>
              <Table.Cell>
                <div class="flex items-center gap-1">
                  <span class="font-mono text-[11px] tabular-nums">{log.attempt_count}</span>
                  {#if log.stream}
                    <StatusBadge tone="violet">{t('logs.table.stream')}</StatusBadge>
                  {/if}
                </div>
              </Table.Cell>
            </Table.Row>
            {#if log.error}
              <Table.Row class="hover:bg-transparent">
                <Table.Cell colspan={10} class="bg-destructive/5 py-1.5">
                  <div class="flex items-start gap-1.5 text-[11px] text-destructive">
                    <TriangleAlertIcon class="mt-0.5 size-3 shrink-0" />
                    <span class="break-all">{log.error}</span>
                  </div>
                </Table.Cell>
              </Table.Row>
            {/if}
          {/each}
        {/if}
      </Table.Body>
    </Table.Root>
  </Card.Content>

  <Card.Footer class="justify-between">
    <span class="text-[11px] text-muted-foreground">
      {t('logs.table.footerInfo', { total: fmtNumber(total), shown: fmtNumber(logs.length) })}
    </span>
    <div class="flex items-center gap-1.5">
      <Button variant="outline" size="sm" disabled={offset === 0 || loading} onclick={() => goPage(-1)}>
        <ArrowLeftIcon />
        {t('logs.table.previous')}
      </Button>
      <Button
        variant="outline"
        size="sm"
        disabled={offset + limit >= total || loading}
        onclick={() => goPage(1)}
      >
        {t('logs.table.next')}
        <ArrowRightIcon />
      </Button>
    </div>
  </Card.Footer>
</Card.Root>
