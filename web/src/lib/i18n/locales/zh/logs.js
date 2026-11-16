/** Chinese messages for the logs console page. */
export default {
  title: '日志与统计 · aigw',
  heading: '日志与统计',
  description: '统一观测所有 API 调用、路由尝试、Token 用量与成本。数据每 15 秒自动刷新。',
  updatedAt: '更新于 {time}',
  range: {
    '1h': '近 1 小时',
    '24h': '近 24 小时',
    '7d': '近 7 天',
    '30d': '近 30 天'
  },
  stats: {
    totalRequests: '总请求',
    successRate: '成功率 {rate}%',
    totalTokens: 'Token 总量',
    tokenBreakdown: '输入 {input} / 输出 {output}',
    totalCost: '总成本',
    costHint: '按模型单价估算',
    avgLatency: '平均延迟',
    p95: 'P95 {value}',
    failedRequests: '失败请求',
    failedHint: '可在下方日志中排查',
    allSucceeded: '全部请求成功'
  },
  trend: {
    title: '请求趋势',
    description: '按小时聚合的请求量',
    emptyTitle: '暂无趋势数据',
    emptyDescription: '当前时间窗口内还没有请求。'
  },
  chart: {
    requests: '请求数',
    requestCount: '{count} 次'
  },
  models: {
    title: '模型分布',
    description: '请求量 Top 6',
    emptyTitle: '暂无模型数据'
  },
  table: {
    title: '调用日志',
    totalCount: '共 {count} 条',
    pageInfo: '第 {page} / {pages} 页 · 每页 {limit} 条',
    searchPlaceholder: '搜索 model / app / key / request id',
    allApis: '全部 API',
    column: {
      time: '时间',
      model: 'Model',
      protocol: '协议',
      caller: '调用方',
      providerAndModel: 'Provider / 虚拟模型',
      latency: '延迟',
      tokens: 'Tokens',
      cost: '成本',
      status: '状态',
      attempts: '尝试'
    },
    fallback: 'fallback',
    stream: 'stream',
    emptyTitle: '暂无日志',
    emptyDescription: '通过网关发起一次调用后，即可在这里看到完整请求链路。',
    footerInfo: '共 {total} 条 · 当前 {shown} 条',
    previous: '上一页',
    next: '下一页'
  }
};
