/** 模型目录页、模型编辑表单与 models.dev 价格同步工具。 */
export default {
  title: '模型目录 · aigw',
  heading: '模型目录',
  description:
    '网关可用的全部模型：Virtual Model 页创建的虚拟模型，以及 Provider 提供的模型（对外 ID 为 provider/model）。此页是只读目录，创建与编辑请到 Virtual Model 或 Model Provider。',
  stats: {
    all: '全部模型',
    allHint: '虚拟模型 + Provider 模型',
    virtual: '虚拟模型',
    virtualHint: '客户端请求时可用的模型名',
    provider: 'Provider 模型',
    providerHint: '{count} 个不可用'
  },
  tab: {
    all: '全部',
    virtual: '虚拟模型',
    provider: 'Provider 模型'
  },
  searchPlaceholder: '搜索模型 ID / 名称 / Provider',
  column: {
    modelId: '模型 ID',
    source: '来源',
    providerName: 'Provider / 名称',
    protocol: '协议',
    context: '上下文',
    price: '价格 / 1M',
    status: '状态',
    createdAt: '创建时间'
  },
  source: {
    virtual: '虚拟模型',
    provider: 'Provider',
    discovered: '自动同步',
    manual: '手动'
  },
  targets: '{count} 个 target',
  tiers: '含 {count} 个阶梯',
  priceHint: '输入 / 输出',
  status: {
    available: '可用',
    providerDisabled: 'Provider 停用',
    disabled: '停用'
  },
  empty: {
    noMatchTitle: '没有匹配的模型',
    noMatchDescription: '调整搜索词或来源筛选后重试。',
    noneTitle: '还没有可用模型',
    noneDescription:
      '先在 Model Provider 添加 Provider 与模型，或到 Virtual Model 页创建虚拟模型。'
  },
  form: {
    modelId: '模型 ID',
    modelIdRequired: '请填写模型 ID',
    contextWindow: '上下文窗口',
    maxOutput: '最大输出',
    invalidBudget: '{label}格式无效，支持 131072、256K、1M',
    currency: '币种',
    currencyInvalid: '币种必须是 3 位 ISO 4217 代码，如 USD、CNY',
    price: {
      input: '输入',
      output: '输出',
      cacheRead: '缓存读取',
      cacheWrite: '缓存写入'
    },
    negativePrice: '{name}价格不能为负数',
    tierThresholdInvalid: '阶梯 {index}：输入 token 阈值无效',
    tierThresholdsAscending: '阶梯阈值必须递增'
  },
  dev: {
    modelIdRequired: '请先填写模型 ID',
    sourceOption: '{provider} ({id}) · 覆盖 {count} 个模型',
    tiers: '{count} 个阶梯',
    notSet: '未设置',
    score: {
      provider: 'Provider 匹配',
      baseUrl: 'API 地址匹配',
      name: '名称匹配',
      modelOnly: '仅模型匹配'
    },
    action: {
      update: '可更新',
      skipHasPrice: '已有价格跳过',
      unmatched: '未匹配',
      ambiguous: '多个匹配',
      unchanged: '无变化',
      error: '失败'
    },
    unknown: '未知'
  }
};
