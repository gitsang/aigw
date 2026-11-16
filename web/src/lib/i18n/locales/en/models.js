/** Models catalog page, model editor form and models.dev price-sync helpers. */
export default {
  title: 'Models · aigw',
  heading: 'Models',
  description:
    'Every model the gateway can serve: virtual models created on the Virtual Model page, and the models each provider exposes (public id is provider/model). This page is a read-only catalog — create and edit them under Virtual Model or Model Provider.',
  stats: {
    all: 'All models',
    allHint: 'Virtual models + provider models',
    virtual: 'Virtual models',
    virtualHint: 'Model names clients can request',
    provider: 'Provider models',
    providerHint: '{count} unavailable'
  },
  tab: {
    all: 'All',
    virtual: 'Virtual models',
    provider: 'Provider models'
  },
  searchPlaceholder: 'Search model ID / name / provider',
  column: {
    modelId: 'Model ID',
    source: 'Source',
    providerName: 'Provider / name',
    protocol: 'Protocol',
    context: 'Context',
    price: 'Price / 1M',
    status: 'Status',
    createdAt: 'Created'
  },
  source: {
    virtual: 'Virtual model',
    provider: 'Provider',
    discovered: 'Synced',
    manual: 'Manual'
  },
  targets: '{count} targets',
  targets_one: '{count} target',
  tiers: '{count} tiers',
  tiers_one: '{count} tier',
  priceHint: 'Input / output',
  status: {
    available: 'Available',
    providerDisabled: 'Provider disabled',
    disabled: 'Disabled'
  },
  empty: {
    noMatchTitle: 'No matching models',
    noMatchDescription: 'Adjust the search term or the source filter and try again.',
    noneTitle: 'No models available yet',
    noneDescription:
      'Add a provider and its models under Model Provider, or create a virtual model on the Virtual Model page.'
  },
  form: {
    modelId: 'Model ID',
    modelIdRequired: 'Model ID is required',
    contextWindow: 'Context window',
    maxOutput: 'Max output',
    invalidBudget: '{label} has an invalid format; use 131072, 256K or 1M',
    currency: 'Currency',
    currencyInvalid: 'Currency must be a 3-letter ISO 4217 code, e.g. USD or CNY',
    price: {
      input: 'Input',
      output: 'Output',
      cacheRead: 'Cache read',
      cacheWrite: 'Cache write'
    },
    negativePrice: '{name} price cannot be negative',
    tierThresholdInvalid: 'Tier {index}: invalid input token threshold',
    tierThresholdsAscending: 'Tier thresholds must increase'
  },
  dev: {
    modelIdRequired: 'Enter a model ID first',
    sourceOption: '{provider} ({id}) · covers {count} models',
    sourceOption_one: '{provider} ({id}) · covers {count} model',
    tiers: '{count} tiers',
    tiers_one: '{count} tier',
    notSet: 'Not set',
    score: {
      provider: 'Provider match',
      baseUrl: 'API URL match',
      name: 'Name match',
      modelOnly: 'Model-only match'
    },
    action: {
      update: 'Updatable',
      skipHasPrice: 'Price already set',
      unmatched: 'Unmatched',
      ambiguous: 'Several matches',
      unchanged: 'Unchanged',
      error: 'Failed'
    },
    unknown: 'Unknown'
  }
};
