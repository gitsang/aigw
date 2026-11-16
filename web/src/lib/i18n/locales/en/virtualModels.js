/** Virtual Model console page: the model list, editor dialog and delete confirmation. */
export default {
  title: 'Virtual Model · aigw',
  heading: 'Virtual Model',
  description:
    'Virtual model ID (the model clients request) → weighted LB targets → fallback. Health checks run every minute; unhealthy LB targets are removed automatically, and the fallback target is used when all of them are offline.',
  action: {
    create: 'Add virtual model',
    createEmpty: 'Create virtual model'
  },
  setup: {
    before: 'Please add at least one upstream service in',
    link: 'Model Provider',
    after: 'first.'
  },
  empty: {
    title: 'No virtual models yet',
    description: 'Create a virtual model to map a virtual model ID to one or more upstream providers.'
  },
  state: {
    enabled: 'Enabled',
    disabled: 'Disabled'
  },
  card: {
    meta: '{name} · {delay}ms base / {backoff}ms max backoff'
  },
  column: {
    role: 'Role',
    providerUpstream: 'Provider / Upstream',
    upstreamProtocol: 'Upstream protocol',
    weight: 'Weight',
    health: 'Health'
  },
  health: {
    healthy: 'Healthy',
    unhealthy: 'Unhealthy'
  },
  target: {
    inheritProtocol: 'Inherit from model / Provider',
    fallbackOnly: 'Fallback only',
    role: 'Role',
    provider: 'Provider',
    model: 'Model',
    weight: 'Weight',
    upstreamProtocol: 'Upstream protocol',
    selectProvider: 'Select…',
    selectModel: 'Select model…',
    autoProtocol: '(automatic, no configuration needed)',
    endpoint: 'Endpoint',
    builtinEndpoint: 'Built-in default Endpoint',
    noModels: 'This Provider has no models yet. Add one on the Provider detail page first.',
    deleteAria: 'Delete target'
  },
  dialog: {
    createTitle: 'Add virtual model',
    editTitle: 'Edit virtual model',
    description:
      'A virtual model maps to several upstream targets, load-balances by weight, and retries or falls back according to the configured policies on failure.',
    enabledLabel: 'Enable this virtual model',
    enabledHint: 'Disabling it makes the virtual model unavailable immediately'
  },
  form: {
    name: 'Name',
    nameHint: '(written to the log as virtual_model_name)',
    virtualModelId: 'Virtual model ID',
    virtualModelIdHint: '(the model clients request)',
    fallbackPolicy: 'Fallback policy',
    retryCount: 'Retry count',
    retryPolicy: 'Backoff policy',
    baseBackoff: 'Base backoff (ms)',
    maxBackoff: 'Max backoff (ms)'
  },
  policy: {
    fallbackFirst:
      'fallback-first · try all LB targets first each round, then fall back, retrying the whole cycle',
    retryFirst: 'retry-first · retry a single target N times before switching, falling back last',
    exponential: 'Exponential backoff',
    fixed: 'Fixed backoff'
  },
  targets: {
    title: 'Targets',
    descriptionPrefix:
      'Each target does not pick an upstream protocol: inbound requests are forwarded with their ',
    descriptionBold: 'original protocol',
    descriptionMiddle:
      '; when the upstream does not support it, the first available protocol in the fixed order',
    descriptionSuffix:
      'is picked and converted automatically. The protocol is determined by the selected model / Provider configuration, and the Endpoint comes from the same source.'
  },
  toast: {
    modelsLoadFailed: 'Failed to load models: {error}',
    virtualModelRequired: 'Enter a virtual model ID',
    targetRequired: 'Configure at least one target',
    targetIncomplete: 'Every target needs a provider and a model',
    created: 'Virtual model created',
    updated: 'Virtual model updated',
    deleted: 'Virtual model deleted'
  },
  delete: {
    title: 'Delete virtual model',
    description: 'Delete virtual model {name}? Clients using it will start failing immediately.'
  }
};
