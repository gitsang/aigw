/** English messages for the providers console page, provider model editor and provider form helpers. */
export default {
  title: 'Model Provider · aigw',
  heading: 'Model Provider',
  description:
    'Manage upstream services, request policies, outbound proxies and the model catalog. Every setting here directly affects how the gateway behaves at runtime.',
  optional: 'Optional',
  keepExisting: 'Leave empty to keep unchanged',
  action: {
    revert: 'Revert',
    revertToast: 'Reverted to the saved configuration',
    addProvider: 'Add Provider',
    createdToast: 'Provider created',
    savedToast: 'Provider saved',
    deletedToast: 'Provider deleted',
    deleteProvider: 'Delete Provider',
    unsavedSwitchConfirm: 'You have unsaved changes. Switch Provider anyway?'
  },
  list: {
    title: 'Provider',
    searchPlaceholder: 'Search name / address',
    noResults: 'No matching Provider',
    empty: 'No Provider yet'
  },
  status: {
    enabled: 'Enabled',
    disabled: 'Disabled',
    unsaved: 'Unsaved'
  },
  tabs: {
    basic: 'Basics',
    request: 'Request',
    proxy: 'Proxy',
    models: 'Models'
  },
  empty: {
    noProvidersDescription:
      'Add an upstream service first, then configure its models and virtual models.',
    selectTitle: 'Select a Provider on the left',
    selectDescription: 'Or create a new upstream service.'
  },
  basic: {
    name: 'Name',
    namePlaceholder: 'deepseek-official',
    id: 'ID',
    idPlaceholder: 'Leave empty to derive it from the name',
    baseUrl: 'Base URL',
    baseUrlPlaceholder: 'https://api.example.com',
    description: 'Description',
    descriptionPlaceholder: 'Optional, helps the team identify this upstream',
    authType: 'Authentication',
    authPlaceholder: 'Select an authentication method',
    auth: {
      bearer: 'Authorization: Bearer',
      xApiKey: 'x-api-key',
      xGoogApiKey: 'x-goog-api-key (Google)',
      none: 'No authentication'
    },
    apiKey: 'API Key',
    apiKeyConfigured: ' (configured ••••{last4})',
    apiKeyNotRequired: 'Authentication is "No authentication", no key needed',
    apiKeyPlaceholder: 'sk-...',
    showApiKey: 'Show API Key',
    hideApiKey: 'Hide API Key'
  },
  protocols: {
    title: 'Upstream protocols',
    description:
      'Tick the protocols this upstream supports (OpenAI Chat Completions / Responses, Anthropic Messages, Gemini Interactions); an empty Endpoint uses the built-in default.',
    endpointAria: '{protocol} Endpoint'
  },
  request: {
    timeout: 'Request timeout (ms)',
    timeoutHint: '{duration} · upper bound for a single upstream call',
    retryTitle: 'Retry policy',
    retryDescription:
      'Provider-level retries; when set to 0 only the virtual model retry policy applies.',
    attempts: '{count} attempts',
    attempts_one: '{count} attempt',
    retryDisabled: 'Not enabled',
    retryCount: 'Retry count',
    backoff: 'Backoff strategy',
    backoffExponential: 'Exponential backoff (exponential)',
    backoffLinear: 'Linear backoff (linear)',
    retryInitial: 'Base backoff (ms)',
    retryMax: 'Max backoff (ms)',
    retryLinearInterval: 'Backoff interval (ms)',
    retryLinearHint: 'Wait a fixed {duration} before each retry'
  },
  headers: {
    label: 'Extra Headers',
    description:
      'Appended to every request sent to this upstream. Protected headers such as Content-Type and Authorization are ignored.'
  },
  proxy: {
    type: 'Proxy type',
    typeHint: 'How the gateway egresses when it calls this upstream',
    options: {
      none: 'No proxy',
      http: 'HTTP / HTTPS',
      socks5: 'SOCKS5'
    },
    url: 'URL',
    urlPlaceholderHttp: 'http://127.0.0.1:7890',
    urlPlaceholderSocks5: 'socks5://127.0.0.1:1080',
    username: 'Username',
    password: 'Password',
    passwordConfigured: '(configured ••••{last4})',
    showPassword: 'Show proxy password',
    hidePassword: 'Hide proxy password',
    encryptedNotice:
      'The proxy password is stored encrypted with AES-256-GCM like the API Key, and the API only returns the last 4 characters. New requests use it as soon as you save.',
    disabledNotice:
      'No proxy is enabled, so the gateway connects to the upstream directly. If this upstream must be reached through a proxy, choose a proxy type and fill in the URL.'
  },
  create: {
    title: 'Add Provider',
    description:
      'Enter a name first; once created, fill in the address, authentication and policies on the detail page.'
  },
  delete: {
    title: 'Delete Provider',
    description:
      'Delete {name}? Its models are deleted as well. Deletion is blocked while a virtual model target references it.',
  },
  duration: {
    minutes: '{value} min'
  },
  price: {
    in: 'in',
    out: 'out',
    cacheRead: 'cr',
    cacheWrite: 'cw',
    perMillion: '1M'
  },
  model: {
    listTitle: 'Models ({count})',
    listDescription:
      'One row per upstream model; click a row to edit it. Context window, max output and tiered pricing are configurable.',
    addTitle: 'Add model',
    editTitle: 'Edit model',
    collapseAria: 'Collapse editor',
    id: 'Model ID',
    idPlaceholder: 'deepseek/deepseek-flash',
    name: 'Name',
    contextWindow: 'Context window',
    contextPlaceholder: '256K / 1M / 131072',
    tokenHint: '1K = 1024, 1M = 1024K',
    maxOutput: 'Max output',
    maxOutputPlaceholder: '8K / 64K',
    maxOutputHint: 'Leave empty to use the upstream default',
    contextWindowValue: 'Context {value}',
    maxOutputValue: 'Output {value}',
    enabled: 'Enable this model',
    deleteAria: 'Delete model',
    sourceDiscovered: 'Synced',
    sourceManual: 'Manual',
    tiersCount: '{count} tiers',
    tiersCount_one: '{count} tier',
    emptyTitle: 'No models yet',
    emptyDescription: 'Sync from the upstream /models endpoint, or add a row manually.',
    discoverButton: 'Sync from /models',
    discoverSuccess: 'Synced {count} models',
    discoverSuccess_one: 'Synced {count} model',
    discoverFailed: 'Sync failed: {error}',
    syncPricesButton: 'Sync prices',
    syncPricesTitle: 'Preview and sync all model prices from models.dev',
    collapse: 'Collapse',
    updatedToast: 'Model updated',
    addedToast: 'Model added',
    deletedToast: 'Model deleted',
    cost: {
      title: 'Pricing',
      description:
        'Price per 1,000,000 tokens; a tier above the threshold replaces the base price as a whole.',
      currencyPlaceholder: 'USD',
      currencyAria: 'Currency',
      syncAria: 'Sync prices from models.dev',
      lookupBusy: 'Looking up…',
      syncButton: 'Sync from models.dev',
      applied: 'Applied',
      source: 'Source: {source}',
      unit: 'USD / 1M tokens',
      multipleMatches: 'Several sources match, pick one',
      applyAria: 'Apply the price from {provider}',
      snapshot: 'Snapshot: {time}',
      input: 'Input',
      output: 'Output',
      cacheRead: 'Cache read',
      cacheWrite: 'Cache write',
      tiersTitle: 'Tiered pricing',
      addTier: 'Add tier',
      tierThreshold: 'Input >',
      tierInput: 'Input',
      tierOutput: 'Output',
      tierCacheRead: 'Cache read',
      tierCacheWrite: 'Cache write',
      tierThresholdAria: 'Tier {index} threshold',
      tierValueAria: 'Tier {index} {field}',
      removeTierAria: 'Remove tier',
      noTiers: 'No tiers configured, every request uses the base price'
    }
  },
  modelsDev: {
    modelIdRequired: 'Enter a model ID first',
    notFound: 'models.dev does not list that model',
    lookupFailed: 'models.dev lookup failed: {error}',
    applied: 'models.dev prices applied (USD / 1M); they take effect after saving'
  },
  sync: {
    title: 'Sync models.dev prices',
    description:
      'Preview the prices pulled from models.dev first; nothing is written until you apply. Prices are USD per 1,000,000 tokens.',
    sourceLabel: 'Price source',
    autoMatch: 'Automatic matching',
    sourceHint:
      'Automatic matching can only confirm a single source; for aggregator providers pick a price source.',
    overwrite: 'Overwrite existing prices',
    overwriteHint: 'When off, only empty prices are filled and existing prices are kept.',
    loading: 'Fetching prices from models.dev…',
    retry: 'Retry',
    summary: '{count} models · snapshot {time}',
    summary_one: '{count} model · snapshot {time}',
    columns: {
      model: 'Model',
      currentPrice: 'Current price',
      after: 'After sync',
      source: 'Source',
      status: 'Status'
    },
    empty: 'This Provider has no models.',
    footer: 'Source: {source}',
    footerWithCount: 'Source: {source} · {count} to update',
    applying: 'Applying…',
    apply: 'Apply {count}',
    applied: 'Updated prices for {count} models',
    applied_one: 'Updated prices for {count} model',
    appliedWithSource: 'Updated prices for {count} models from {source}',
    appliedWithSource_one: 'Updated prices for {count} model from {source}',
    failedCount: '{count} models failed to write, please check and retry',
    failedCount_one: '{count} model failed to write, please check and retry',
    failed: 'Sync failed: {error}'
  },
  validation: {
    nameRequired: 'Name is required',
    baseUrlRequired: 'Base URL is required',
    baseUrlScheme: 'Base URL must start with http:// or https://',
    protocolsRequired: 'Enable at least one upstream protocol',
    proxyUrlRequired: 'Proxy URL is required when a proxy is enabled',
    retryCountRange: 'Retry count must be between 0 and 20',
    backoffNonNegative: 'Backoff times cannot be negative',
    backoffMaxLessThanInitial: 'Max backoff cannot be less than the base backoff'
  }
};
