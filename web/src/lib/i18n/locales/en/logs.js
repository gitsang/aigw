/** English messages for the logs console page. */
export default {
  title: 'Logs & Stats · aigw',
  heading: 'Logs & Stats',
  description:
    'Observe every API call, routing attempt, token usage and cost in one place. Data refreshes every 15 seconds.',
  updatedAt: 'Updated at {time}',
  range: {
    '1h': 'Last hour',
    '24h': 'Last 24 hours',
    '7d': 'Last 7 days',
    '30d': 'Last 30 days'
  },
  stats: {
    totalRequests: 'Total requests',
    successRate: 'Success rate {rate}%',
    totalTokens: 'Total tokens',
    tokenBreakdown: 'Input {input} / Output {output}',
    totalCost: 'Total cost',
    costHint: 'Estimated from model pricing',
    avgLatency: 'Average latency',
    p95: 'P95 {value}',
    failedRequests: 'Failed requests',
    failedHint: 'Inspect the logs below',
    allSucceeded: 'All requests succeeded'
  },
  trend: {
    title: 'Request trend',
    description: 'Requests aggregated by hour',
    emptyTitle: 'No trend data yet',
    emptyDescription: 'There are no requests in this time window yet.'
  },
  chart: {
    requests: 'Requests',
    requestCount: '{count} requests',
    requestCount_one: '{count} request'
  },
  models: {
    title: 'Model distribution',
    description: 'Top 6 by request count',
    emptyTitle: 'No model data yet'
  },
  table: {
    title: 'Request logs',
    totalCount: '{count} entries',
    totalCount_one: '{count} entry',
    pageInfo: 'Page {page} of {pages} · {limit} per page',
    searchPlaceholder: 'Search model / app / key / request id',
    allApis: 'All APIs',
    column: {
      time: 'Time',
      model: 'Model',
      protocol: 'Protocol',
      caller: 'Caller',
      providerAndModel: 'Provider / Virtual Model',
      latency: 'Latency',
      tokens: 'Tokens',
      cost: 'Cost',
      status: 'Status',
      attempts: 'Attempts'
    },
    fallback: 'fallback',
    stream: 'stream',
    emptyTitle: 'No logs yet',
    emptyDescription:
      'Make a request through the gateway and the full request chain will appear here.',
    footerInfo: '{total} total · {shown} shown',
    previous: 'Previous',
    next: 'Next'
  }
};
