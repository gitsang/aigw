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

  import { api, fmtCost, fmtLatency, fmtNumber, fmtTime } from '$lib/api';
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
    { value: '1h', label: '近 1 小时' },
    { value: '24h', label: '近 24 小时' },
    { value: '7d', label: '近 7 天' },
    { value: '30d', label: '近 30 天' }
  ];

  const API_TYPES = ['chat.completions', 'responses', 'messages'];

  const chartConfig = {
    requests: { label: '请求数', color: 'var(--chart-2)' }
  };

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
  <title>日志与统计 · aigw</title>
</svelte:head>

<PageHeader
  title="日志与统计"
  description="统一观测所有 API 调用、路由尝试、token 用量与成本。数据每 15 秒自动刷新。"
>
  <span class="flex items-center gap-1.5 text-[11px] text-muted-foreground">
    <span class="size-1.5 rounded-full bg-emerald-500 shadow-[0_0_0_3px] shadow-emerald-500/20"
    ></span>
    {lastUpdated ? `更新于 ${lastUpdated.toLocaleTimeString('zh-CN', { hour12: false })}` : '加载中…'}
  </span>
  <Button variant="outline" size="sm" onclick={refresh} disabled={loading}>
    <RefreshCwIcon class={loading ? 'animate-spin' : ''} />
    刷新
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
        <Tabs.Trigger value={item.value} class="px-2.5">{item.label}</Tabs.Trigger>
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
    label="总请求"
    value={fmtNumber(stats?.total_requests)}
    hint={`成功率 ${successRate}%`}
    icon={ActivityIcon}
  />
  <StatCard
    label="Token 总量"
    value={fmtNumber(stats?.total_tokens)}
    hint={`输入 ${fmtNumber(stats?.prompt_tokens)} / 输出 ${fmtNumber(stats?.completion_tokens)}`}
    icon={HashIcon}
  />
  <StatCard
    label="总成本"
    value={fmtCost(stats?.total_cost)}
    hint="按模型单价估算"
    icon={CoinsIcon}
  />
  <StatCard
    label="平均延迟"
    value={fmtLatency(stats?.avg_latency_ms)}
    hint={`P95 ${fmtLatency(stats?.p95_latency_ms)}`}
    icon={GaugeIcon}
  />
  <StatCard
    label="失败请求"
    value={fmtNumber(stats?.failed_requests)}
    hint={stats?.failed_requests ? '可在下方日志中排查' : '全部请求成功'}
    icon={TriangleAlertIcon}
  />
</section>

<section class="grid gap-2.5 lg:grid-cols-[1.55fr_1fr]">
  <Card.Root class="gap-2">
    <Card.Header>
      <div class="flex items-start justify-between gap-2">
        <div class="space-y-0.5">
          <Card.Title class="text-sm">请求趋势</Card.Title>
          <Card.Description class="text-[11px]">按小时聚合的请求量</Card.Description>
        </div>
        <StatusBadge tone="primary">{RANGES.find((r) => r.value === range)?.label}</StatusBadge>
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
            series={[{ key: 'requests', label: '请求数', color: 'var(--chart-2)' }]}
            axis="x"
            props={{
              area: { fillOpacity: 0.3, line: { class: 'stroke-1' }, motion: 'tween' },
              xAxis: {
                format: (v) =>
                  new Date(v).toLocaleTimeString('zh-CN', {
                    hour: '2-digit',
                    minute: '2-digit',
                    hour12: false
                  })
              }
            }}
          >
            {#snippet tooltip()}
              <Chart.Tooltip
                indicator="line"
                labelFormatter={(v) => fmtTime(v)}
                formatter={(value) => `${fmtNumber(value)} 次`}
              />
            {/snippet}
          </AreaChart>
        </Chart.Container>
      {:else}
        <Empty.Root class="h-[210px] border">
          <Empty.Header>
            <Empty.Media variant="icon"><InboxIcon /></Empty.Media>
            <Empty.Title>暂无趋势数据</Empty.Title>
            <Empty.Description>当前时间窗口内还没有请求。</Empty.Description>
          </Empty.Header>
        </Empty.Root>
      {/if}
    </Card.Content>
  </Card.Root>

  <Card.Root class="gap-2">
    <Card.Header>
      <div class="space-y-0.5">
        <Card.Title class="text-sm">模型分布</Card.Title>
        <Card.Description class="text-[11px]">请求量 Top 6</Card.Description>
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
            <Empty.Title>暂无模型数据</Empty.Title>
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
        <Card.Title class="text-sm">调用日志</Card.Title>
        <StatusBadge tone="neutral">共 {fmtNumber(total)} 条</StatusBadge>
      </div>
      <span class="text-[11px] text-muted-foreground">
        第 {page} / {pages} 页 · 每页 {limit} 条
      </span>
    </div>

    <form class="grid gap-2 md:grid-cols-2 xl:grid-cols-[minmax(200px,1.6fr)_minmax(150px,1fr)_minmax(120px,1fr)_minmax(120px,1fr)_auto_auto]" onsubmit={applyFilters}>
      <InputGroup.Root>
        <InputGroup.Addon><SearchIcon /></InputGroup.Addon>
        <InputGroup.Input placeholder="搜索 model / app / key / request id" bind:value={filters.search} />
      </InputGroup.Root>

      <Select.Root type="single" bind:value={filters.api_type}>
        <Select.Trigger class="w-full">
          <Select.Value placeholder="全部 API" />
        </Select.Trigger>
        <Select.Content>
          {#each API_TYPES as name (name)}
            <Select.Item value={name} label={name}>{name}</Select.Item>
          {/each}
        </Select.Content>
      </Select.Root>

      <Input placeholder="model" bind:value={filters.model} />
      <Input placeholder="app" bind:value={filters.app} />

      <Button type="submit" disabled={loading}>查询</Button>
      <Button type="button" variant="ghost" onclick={resetFilters}>重置</Button>
    </form>
  </Card.Header>

  <Card.Content class="px-0">
    <Table.Root>
      <Table.Header>
        <Table.Row>
          <Table.Head>时间</Table.Head>
          <Table.Head>Model</Table.Head>
          <Table.Head>ApiType</Table.Head>
          <Table.Head>调用方</Table.Head>
          <Table.Head>Provider / Route</Table.Head>
          <Table.Head class="text-right">延迟</Table.Head>
          <Table.Head class="text-right">Tokens</Table.Head>
          <Table.Head class="text-right">成本</Table.Head>
          <Table.Head>状态</Table.Head>
          <Table.Head>尝试</Table.Head>
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
                  <Empty.Title>暂无日志</Empty.Title>
                  <Empty.Description>
                    通过网关发起一次调用后，即可在这里看到完整请求链路。
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
              <Table.Cell><StatusBadge tone="info">{log.api_type}</StatusBadge></Table.Cell>
              <Table.Cell class="max-w-[130px]">
                <div class="truncate text-[11px]">{log.app || '—'}</div>
                <div class="truncate text-[10px] text-muted-foreground">
                  {log.api_key_name || '—'}
                </div>
              </Table.Cell>
              <Table.Cell class="max-w-[150px]">
                <div class="truncate text-[11px]">{log.provider_name || '—'}</div>
                <div class="truncate text-[10px] text-muted-foreground">
                  {log.route_name || '—'}
                  {#if log.fallback_used}<span class="text-amber-600 dark:text-amber-400">· fallback</span>{/if}
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
                    <StatusBadge tone="violet">stream</StatusBadge>
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
      共 {fmtNumber(total)} 条 · 当前 {fmtNumber(logs.length)} 条
    </span>
    <div class="flex items-center gap-1.5">
      <Button variant="outline" size="sm" disabled={offset === 0 || loading} onclick={() => goPage(-1)}>
        <ArrowLeftIcon />
        上一页
      </Button>
      <Button
        variant="outline"
        size="sm"
        disabled={offset + limit >= total || loading}
        onclick={() => goPage(1)}
      >
        下一页
        <ArrowRightIcon />
      </Button>
    </div>
  </Card.Footer>
</Card.Root>
