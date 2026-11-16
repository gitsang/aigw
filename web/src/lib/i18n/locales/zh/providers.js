/** 中文词条：Provider 控制台页、Provider 模型编辑器与 Provider 表单辅助函数。 */
export default {
  title: 'Model Provider · aigw',
  heading: 'Model Provider',
  description:
    '管理上游服务、请求策略、出站代理与模型目录。这里的每项配置都会直接影响网关的实时行为。',
  optional: '可选',
  keepExisting: '留空表示不修改',
  action: {
    revert: '还原',
    revertToast: '已还原为已保存的配置',
    addProvider: '添加 Provider',
    createdToast: 'Provider 已创建',
    savedToast: 'Provider 已保存',
    deletedToast: 'Provider 已删除',
    deleteProvider: '删除 Provider',
    unsavedSwitchConfirm: '当前改动尚未保存，确定切换 Provider 吗？'
  },
  list: {
    title: 'Provider',
    searchPlaceholder: '搜索名称 / 地址',
    noResults: '没有匹配的 Provider',
    empty: '还没有 Provider'
  },
  status: {
    enabled: '启用',
    disabled: '停用',
    unsaved: '未保存'
  },
  tabs: {
    basic: '基础',
    request: '请求',
    proxy: '代理',
    models: '模型'
  },
  empty: {
    noProvidersDescription: '先添加一个上游服务，再配置模型与虚拟模型。',
    selectTitle: '选择左侧 Provider',
    selectDescription: '或者新建一个上游服务。'
  },
  basic: {
    name: '名称',
    namePlaceholder: 'deepseek-official',
    id: 'ID',
    idPlaceholder: '留空则由名称自动生成',
    baseUrl: 'Base URL',
    baseUrlPlaceholder: 'https://api.example.com',
    description: '描述',
    descriptionPlaceholder: '可选，便于团队识别该上游用途',
    authType: '认证方式',
    authPlaceholder: '选择认证方式',
    auth: {
      bearer: 'Authorization: Bearer',
      xApiKey: 'x-api-key',
      none: '无认证'
    },
    apiKey: 'API Key',
    apiKeyConfigured: '（已配置 ••••{last4}）',
    apiKeyNotRequired: '认证方式为「无认证」，无需填写',
    apiKeyPlaceholder: 'sk-...',
    showApiKey: '显示 API Key',
    hideApiKey: '隐藏 API Key'
  },
  protocols: {
    title: '上游协议',
    description:
      '勾选该上游支持的协议（OpenAI Chat Completions / Responses / Anthropic Messages）；Endpoint 留空则使用内置默认值。',
    endpointAria: '{protocol} Endpoint'
  },
  request: {
    timeout: '请求超时 (ms)',
    timeoutHint: '{duration} · 单次上游调用上限',
    retryTitle: '重试规则',
    retryDescription: 'Provider 级重试；设为 0 时仅由虚拟模型的重试策略控制。',
    attempts: '共 {count} 次尝试',
    retryDisabled: '未启用',
    retryCount: '重试次数',
    backoff: '退避策略',
    backoffPlaceholder: 'exponential',
    backoffExponential: '指数退避 (exponential)',
    backoffLinear: '线性退避 (linear)',
    retryInitial: '基础退避 (ms)',
    retryMax: '最大退避 (ms)',
    retryLinearInterval: '退避间隔 (ms)',
    retryLinearHint: '每次重试前固定等待 {duration}'
  },
  headers: {
    label: '额外 Header',
    description:
      '会附加到每个发往该上游的请求。Content-Type、Authorization 等受保护 Header 会被忽略。'
  },
  proxy: {
    type: '代理类型',
    typeHint: '网关访问该上游时经由此代理出站',
    options: {
      none: '不使用代理',
      http: 'HTTP / HTTPS',
      socks5: 'SOCKS5'
    },
    url: 'URL',
    urlPlaceholderHttp: 'http://127.0.0.1:7890',
    urlPlaceholderSocks5: 'socks5://127.0.0.1:1080',
    username: 'Username',
    password: 'Password',
    passwordConfigured: '（已配置 ••••{last4}）',
    showPassword: '显示代理密码',
    hidePassword: '隐藏代理密码',
    encryptedNotice:
      '代理密码与 API Key 一样使用 AES-256-GCM 加密存储，接口只回传末 4 位。保存后新请求立即生效。',
    disabledNotice:
      '未启用代理，网关直连上游。若该上游需要经代理访问，请选择代理类型并填写 URL。'
  },
  create: {
    title: '添加 Provider',
    description: '先填写名称，创建后在详情页补齐地址、认证与策略配置。'
  },
  delete: {
    title: '删除 Provider',
    description:
      '确定删除 {name}？其下模型会一并删除，被虚拟模型的 target 引用时会阻止删除。',
  },
  duration: {
    minutes: '{value} 分钟'
  },
  price: {
    in: '输入',
    out: '输出',
    cacheRead: '缓存读',
    cacheWrite: '缓存写',
    perMillion: '1M'
  },
  model: {
    listTitle: '模型（{count}）',
    listDescription: '每个上游模型一行，点击一行展开编辑；可配置上下文窗口、最大输出与分级计费。',
    addTitle: '添加模型',
    editTitle: '编辑模型',
    collapseAria: '收起编辑',
    id: '模型 ID',
    idPlaceholder: 'deepseek/deepseek-flash',
    name: '名称',
    contextWindow: '上下文窗口',
    contextPlaceholder: '256K / 1M / 131072',
    tokenHint: '1K = 1024，1M = 1024K',
    maxOutput: '最大输出',
    maxOutputPlaceholder: '8K / 64K',
    maxOutputHint: '留空表示使用上游默认',
    contextWindowValue: '上下文 {value}',
    maxOutputValue: '输出 {value}',
    enabled: '启用该模型',
    deleteAria: '删除模型',
    sourceDiscovered: '自动同步',
    sourceManual: '手动',
    tiersCount: '{count} 个阶梯',
    emptyTitle: '还没有模型',
    emptyDescription: '从上游 /models 同步，或手动添加一行。',
    discoverButton: '从 /models 同步',
    discoverSuccess: '已同步 {count} 个模型',
    discoverFailed: '同步失败：{error}',
    syncPricesButton: '同步价格',
    syncPricesTitle: '从 models.dev 预览并同步全部模型价格',
    collapse: '收起',
    updatedToast: '模型已更新',
    addedToast: '模型已添加',
    deletedToast: '模型已删除',
    cost: {
      title: '计费',
      description: '每 100 万 tokens 的价格；阶梯超过阈值后整体替代基础价。',
      currencyPlaceholder: 'USD',
      currencyAria: '币种',
      syncAria: '从 models.dev 同步价格',
      lookupBusy: '查询中…',
      syncButton: '从 models.dev 同步',
      applied: '已填入',
      source: '来源：{source}',
      unit: 'USD / 1M tokens',
      multipleMatches: '多个来源匹配，请选择',
      applyAria: '应用 {provider} 的价格',
      snapshot: '数据快照：{time}',
      input: '输入',
      output: '输出',
      cacheRead: '缓存读取',
      cacheWrite: '缓存写入',
      tiersTitle: '阶梯计价',
      addTier: '添加阶梯',
      tierThreshold: '输入 >',
      tierInput: '输入',
      tierOutput: '输出',
      tierCacheRead: '缓存读',
      tierCacheWrite: '缓存写',
      tierThresholdAria: '阶梯 {index} 阈值',
      tierValueAria: '阶梯 {index} {field}',
      removeTierAria: '删除阶梯',
      noTiers: '未配置阶梯，全部请求使用基础价'
    }
  },
  modelsDev: {
    modelIdRequired: '请先填写模型 ID',
    notFound: 'models.dev 未收录该模型',
    lookupFailed: 'models.dev 查询失败：{error}',
    applied: '已填入 models.dev 价格（USD / 1M），保存后生效'
  },
  sync: {
    title: '同步 models.dev 价格',
    description: '先预览将从 models.dev 拉取的价格，点击应用后才会写入。价格为 USD / 100 万 tokens。',
    sourceLabel: '价格来源',
    autoMatch: '自动匹配',
    sourceHint: '自动匹配只能确认唯一来源；聚合类 Provider 请选择一个价格来源。',
    overwrite: '覆盖已有价格',
    overwriteHint: '关闭时只填充空价格，已有价格保持不变。',
    loading: '正在从 models.dev 拉取价格…',
    retry: '重试',
    summary: '共 {count} 个模型 · 快照 {time}',
    columns: {
      model: '模型',
      currentPrice: '当前价格',
      after: '同步后',
      source: '来源',
      status: '状态'
    },
    empty: '该 Provider 下没有模型。',
    footer: '来源：{source}',
    footerWithCount: '来源：{source} · 将更新 {count} 项',
    applying: '应用中…',
    apply: '应用 {count} 项',
    applied: '已更新 {count} 个模型价格',
    appliedWithSource: '已按 {source} 更新 {count} 个模型价格',
    failedCount: '{count} 个模型写入失败，请检查后重试',
    failed: '同步失败：{error}'
  },
  validation: {
    nameRequired: '请填写名称',
    baseUrlRequired: '请填写 Base URL',
    baseUrlScheme: 'Base URL 必须以 http:// 或 https:// 开头',
    protocolsRequired: '至少启用一种上游协议',
    proxyUrlRequired: '启用代理时必须填写代理 URL',
    retryCountRange: '重试次数必须在 0-20 之间',
    backoffNonNegative: '退避时间不能为负数',
    backoffMaxLessThanInitial: '最大退避不能小于基础退避'
  }
};
