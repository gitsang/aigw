/** English messages for the API keys console page. */
export default {
  title: 'API Keys · aigw',
  heading: 'API Keys',
  description:
    'Clients call the gateway with these keys. A key is shown only once; the database stores only its SHA-256 hash.',
  create: 'Create API Key',
  list: {
    title: 'Key list',
    count: '{count} keys',
    count_one: '{count} key'
  },
  column: {
    name: 'Name',
    app: 'App',
    keyPrefix: 'Key Prefix',
    allowedModels: 'Allowed models',
    status: 'Status',
    lastUsed: 'Last used',
    expiresAt: 'Expires at',
    createdAt: 'Created at',
    actions: 'Actions'
  },
  empty: {
    title: 'No API Keys yet',
    descriptionBefore: 'Once created, call the gateway with',
    descriptionAfter: '.'
  },
  allModels: 'All models',
  never: 'Never',
  noExpiry: 'Never expires',
  toast: {
    updated: 'API Key updated',
    copied: 'Copied to clipboard',
    copyFailed: 'Copy failed, please select and copy it manually',
    deleted: 'API Key deleted'
  },
  dialog: {
    createdTitle: 'API Key created',
    editTitle: 'Edit API Key',
    createTitle: 'Create API Key',
    createdDescription:
      'Copy it now and store it safely; the full key cannot be viewed again after closing.',
    formDescription: 'Configure the name, app identifier, allowed models and expiry.',
    copied: 'Copied',
    example: 'Example request',
    done: 'Done'
  },
  form: {
    name: 'Name',
    namePlaceholder: 'production-app',
    app: 'App / business identifier',
    appPlaceholder: 'my-app',
    appHint: 'Shown in logs and stats so requests can be filtered by app.',
    allowedModels: 'Allowed models (comma-separated, leave empty to allow all)',
    expiresAt: 'Expires at (optional)',
    enabled: 'Enable this key',
    enabledHint: 'Disabling it revokes client access immediately'
  },
  delete: {
    title: 'Delete API Key',
    description: 'Delete {name}? Clients using this key will immediately lose access.'
  }
};
