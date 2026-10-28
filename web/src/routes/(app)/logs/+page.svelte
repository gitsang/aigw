<script>
  import { onMount } from 'svelte';
  import { api, fmtCost, fmtLatency, fmtNumber, fmtTime } from '$lib/api';
  import StatCard from '$lib/components/StatCard.svelte';
  import Badge from '$lib/components/Badge.svelte';

  let range = '24h';
  let stats = null;
  let logs = [];
  let total = 0;
  let limit = 50;
  let offset = 0;
  let loading = false;
  let error = '';
  let filters = { search: '', api_type: '', model: '', app: '', status: '' };
  let lastUpdated = null;
  let timer;

  async function loadStats() {
    stats = await api(`/api/admin/stats?range=${range}`);
  }

  async function loadLogs() {
    loading = true;
    error = '';
    try {
      const q = new URLSearchParams({ limit, offset });
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

  async function refresh() {
    await Promise.all([loadStats(), loadLogs()]);
  }

  function applyFilters(event) {
    event?.preventDefault?.();
    offset = 0;
    loadLogs();
  }

  function setRange(value) {
    range = value;
    loadStats();
  }

  onMount(() => {
    refresh();
    timer = setInterval(refresh, 15000);
    return () => clearInterval(timer);
  });

  $: pages = Math.max(1, Math.ceil(total / limit));
  $: page = Math.floor(offset / limit) + 1;
  $: successRate = stats?.total_requests ? ((stats.success_requests / stats.total_requests) * 100).toFixed(1) : '0.0';
  $: maxSeries = Math.max(1, ...(stats?.recent_series || []).map((p) => p.requests));
</script>

<svelte:head><title>日志与统计 · aigw</title></svelte:head>

<header class="page-head">
  <div>
    <h1>日志与统计</h1>
    <p>统一观测所有 API 调用、路由尝试、token 用量与成本。</p>
  </div>
  <div class="actions">
    <span class="live"><i></i>{lastUpdated ? `更新于 ${lastUpdated.toLocaleTimeString('zh-CN', { hour12: false })}` : '加载中'}</span>
    <button class="btn" on:click={refresh} disabled={loading}>刷新</button>
  </div>
</header>

<div class="range-tabs">
  {#each [['1h', '近 1 小时'], ['24h', '近 24 小时'], ['7d', '近 7 天'], ['30d', '近 30 天']] as [v, label]}
    <button class:active={range === v} on:click={() => setRange(v)}>{label}</button>
  {/each}
</div>

{#if stats}
  <section class="stats">
    <StatCard label="总请求" value={fmtNumber(stats.total_requests)} hint={`${successRate}% 成功`} tone="blue" />
    <StatCard label="Token 总量" value={fmtNumber(stats.total_tokens)} hint={`输入 ${fmtNumber(stats.prompt_tokens)} / 输出 ${fmtNumber(stats.completion_tokens)}`} tone="violet" />
    <StatCard label="总成本" value={fmtCost(stats.total_cost)} hint="按 provider 单价估算" tone="green" />
    <StatCard label="平均延迟" value={fmtLatency(stats.avg_latency_ms)} hint={`P95 ${fmtLatency(stats.p95_latency_ms)}`} tone="amber" />
    <StatCard label="失败请求" value={fmtNumber(stats.failed_requests)} hint={`成功率 ${successRate}%`} tone={stats.failed_requests ? 'amber' : 'green'} />
  </section>

  <section class="charts">
    <div class="panel chart">
      <div class="panel-title"><h2>请求趋势</h2><span>按小时聚合</span></div>
      {#if stats.recent_series?.length}
        <div class="bars">
          {#each stats.recent_series as p}
            <div class="bar-wrap" title={`${fmtTime(p.time)} · ${p.requests} 次请求 · ${fmtNumber(p.tokens)} tokens`}>
              <div class="bar" style={`height:${Math.max(4, (p.requests / maxSeries) * 100)}%`}></div>
            </div>
          {/each}
        </div>
        <div class="axis"><span>{fmtTime(stats.recent_series[0]?.time)}</span><span>{fmtTime(stats.recent_series[stats.recent_series.length - 1]?.time)}</span></div>
      {:else}
        <div class="empty">暂无数据</div>
      {/if}
    </div>
    <div class="panel breakdown">
      <div class="panel-title"><h2>模型分布</h2><span>请求量 Top 10</span></div>
      {#if Object.keys(stats.requests_by_model || {}).length}
        {#each Object.entries(stats.requests_by_model).slice(0, 6) as [model, count]}
          <div class="break-row">
            <span class="mono model">{model}</span>
            <div class="track"><div class="fill" style={`width:${(count / stats.total_requests) * 100}%`}></div></div>
            <span class="count">{fmtNumber(count)}</span>
          </div>
        {/each}
      {:else}
        <div class="empty">暂无数据</div>
      {/if}
      <div class="api-types">
        {#each Object.entries(stats.requests_by_api || {}) as [k, v]}
          <span class="chip">{k} · {fmtNumber(v)}</span>
        {/each}
      </div>
    </div>
  </section>
{/if}

<section class="panel logs">
  <div class="panel-title">
    <h2>调用日志</h2>
    <span>共 {fmtNumber(total)} 条</span>
  </div>
  <form class="filters" on:submit={applyFilters}>
    <input class="input" placeholder="搜索 model / app / key / request id" bind:value={filters.search} />
    <select class="select" bind:value={filters.api_type}>
      <option value="">全部 API</option>
      <option value="chat.completions">chat.completions</option>
      <option value="responses">responses</option>
      <option value="messages">messages</option>
    </select>
    <input class="input narrow" placeholder="model" bind:value={filters.model} />
    <input class="input narrow" placeholder="app" bind:value={filters.app} />
    <button class="btn primary" type="submit" disabled={loading}>查询</button>
    <button class="btn ghost" type="button" on:click={() => { filters = { search: '', api_type: '', model: '', app: '', status: '' }; offset = 0; loadLogs(); }}>重置</button>
  </form>

  {#if error}<div class="error">{error}</div>{/if}

  <div class="table-wrap">
    <table class="data">
      <thead>
        <tr>
          <th>Time</th><th>ApiType</th><th>Model</th><th>App</th><th>Provider</th>
          <th>Latency</th><th>Tokens</th><th>Cost</th><th>ApikeyName</th><th>Status</th><th>Route</th><th>Attempts</th>
        </tr>
      </thead>
      <tbody>
        {#if loading && !logs.length}
          <tr><td colspan="12"><div class="empty"><div class="spinner" style="margin:auto"></div></div></td></tr>
        {:else if !logs.length}
          <tr><td colspan="12"><div class="empty">暂无日志。通过网关发起一次调用后即可在此看到详情。</div></td></tr>
        {:else}
          {#each logs as log (log.id)}
            <tr>
              <td class="mono nowrap">{fmtTime(log.time)}</td>
              <td><Badge tone="blue">{log.api_type}</Badge></td>
              <td class="mono model-cell">{log.model}{#if log.upstream_model && log.upstream_model !== log.model}<div class="sub mono">→ {log.upstream_model}</div>{/if}</td>
              <td>{log.app || '—'}</td>
              <td>{log.provider_name || '—'}</td>
              <td class="mono">{fmtLatency(log.latency_ms)}</td>
              <td class="mono">{fmtNumber(log.total_tokens)}<div class="sub">{fmtNumber(log.prompt_tokens)} / {fmtNumber(log.completion_tokens)}</div></td>
              <td class="mono">{fmtCost(log.cost)}</td>
              <td>{log.api_key_name || '—'}</td>
              <td><Badge tone={log.status_code >= 200 && log.status_code < 400 ? 'green' : 'red'} dot>{log.status_code}</Badge></td>
              <td class="nowrap">{log.route_name || '—'}{#if log.fallback_used}<div><Badge tone="amber">fallback</Badge></div>{/if}</td>
              <td class="mono">{log.attempt_count}{#if log.stream}<div><Badge tone="violet">stream</Badge></div>{/if}</td>
            </tr>
            {#if log.error}
              <tr class="error-row"><td colspan="12"><span>错误：{log.error}</span></td></tr>
            {/if}
          {/each}
        {/if}
      </tbody>
    </table>
  </div>

  <div class="pager">
    <span>第 {page} / {pages} 页</span>
    <div>
      <button class="btn sm" disabled={offset === 0 || loading} on:click={() => { offset = Math.max(0, offset - limit); loadLogs(); }}>上一页</button>
      <button class="btn sm" disabled={offset + limit >= total || loading} on:click={() => { offset += limit; loadLogs(); }}>下一页</button>
    </div>
  </div>
</section>

<style>
  .page-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; margin-bottom: 18px; }
  h1 { margin: 0; font-size: 23px; letter-spacing: -.03em; }
  .page-head p { margin: 6px 0 0; color: #64748b; font-size: 13px; }
  .actions { display: flex; align-items: center; gap: 12px; }
  .live { font-size: 11.5px; color: #64748b; display: flex; align-items: center; gap: 6px; }
  .live i { width: 7px; height: 7px; border-radius: 50%; background: #10b981; box-shadow: 0 0 0 3px #d1fae5; }
  .range-tabs { display: inline-flex; gap: 0; background: #e4eaf2; border: 1px solid #d5dfeb; border-radius: 0; padding: 0; margin-bottom: 14px; }
  .range-tabs button { border: 0; border-right: 1px solid #d5dfeb; background: transparent; border-radius: 0; padding: 5px 12px; font-size: 12px; color: #64748b; cursor: pointer; }
  .range-tabs button:last-child { border-right: 0; }
  .range-tabs button.active { background: #fff; color: #0f172a; font-weight: 650; }
  .stats { display: grid; grid-template-columns: repeat(5, 1fr); gap: 12px; margin-bottom: 16px; }
  .charts { display: grid; grid-template-columns: 1.45fr 1fr; gap: 14px; margin-bottom: 16px; }
  .panel-title { display: flex; justify-content: space-between; align-items: baseline; padding: 12px 14px 9px; }
  .panel-title h2 { margin: 0; font-size: 13.5px; }
  .panel-title span { font-size: 11px; color: #94a3b8; }
  .chart { padding-bottom: 10px; }
  .bars { height: 150px; display: flex; align-items: flex-end; gap: 4px; padding: 8px 14px 0; }
  .bar-wrap { flex: 1; height: 100%; display: flex; align-items: flex-end; min-width: 5px; }
  .bar { width: 100%; border-radius: 0; background: #2563eb; min-height: 4px; }
  .axis { display: flex; justify-content: space-between; color: #94a3b8; font-size: 10px; padding: 7px 14px 0; }
  .break-row { display: flex; align-items: center; gap: 10px; padding: 5px 14px; }
  .model { flex: 0 0 130px; font-size: 11.5px; overflow: hidden; text-overflow: ellipsis; }
  .track { flex: 1; height: 6px; background: #f1f5f9; border-radius: 0; overflow: hidden; }
  .fill { height: 100%; background: #8b5cf6; border-radius: 0; }
  .count { font-size: 11.5px; color: #64748b; min-width: 30px; text-align: right; }
  .api-types { display: flex; gap: 5px; flex-wrap: wrap; padding: 10px 14px 4px; }
  .logs { overflow: hidden; }
  .filters { display: grid; grid-template-columns: minmax(230px, 2fr) 1fr .7fr .7fr auto auto; gap: 7px; padding: 4px 14px 12px; }
  .input.narrow { min-width: 0; }
  .error { margin: 0 14px 12px; padding: 8px 10px; background: #fef2f2; color: #b91c1c; border-radius: 2px; font-size: 12.5px; }
  .model-cell { font-weight: 600; }
  .sub { color: #94a3b8; font-size: 10.5px; font-weight: 400; margin-top: 2px; }
  .nowrap { white-space: nowrap; }
  .error-row td { background: #fff7f7; color: #b91c1c; font-size: 11.5px; padding-top: 0; border-bottom: 1px solid #fecaca; }
  .pager { display: flex; justify-content: space-between; align-items: center; padding: 10px 14px; font-size: 12px; color: #64748b; border-top: 1px solid #e2e8f0; }
  .pager > div { display: flex; gap: 6px; }
  @media (max-width: 1200px) {
    .stats { grid-template-columns: repeat(3, 1fr); }
    .charts { grid-template-columns: 1fr; }
    .filters { grid-template-columns: 1fr 1fr; }
  }
  @media (max-width: 700px) {
    .stats { grid-template-columns: 1fr 1fr; }
    .filters { grid-template-columns: 1fr; }
  }
</style>
