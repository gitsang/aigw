/** Chinese messages for the virtualModels console page. */
export default {
  title: 'Virtual Model · aigw',
  heading: 'Virtual Model',
  description:
    '虚拟模型 ID（客户端请求的 model）→ 加权 LB target → fallback。健康检查每分钟执行，不健康的 LB 自动移出，全部下线时使用 fallback。',
  action: {
    create: '添加虚拟模型',
    createEmpty: '创建虚拟模型'
  },
  setup: {
    before: '请先在',
    link: 'Model Provider',
    after: '添加至少一个上游服务。'
  },
  empty: {
    title: '还没有虚拟模型',
    description: '创建一个虚拟模型，把虚拟模型 ID 映射到一个或多个上游 provider。'
  },
  state: {
    enabled: '启用',
    disabled: '停用'
  },
  card: {
    meta: '{name} · 基础退避 {delay}ms / 最大退避 {backoff}ms'
  },
  column: {
    role: '角色',
    providerUpstream: 'Provider / Upstream',
    upstreamProtocol: '上游协议',
    weight: '权重',
    health: '健康'
  },
  health: {
    healthy: '健康',
    unhealthy: '不健康'
  },
  target: {
    inheritProtocol: '继承模型 / Provider',
    fallbackOnly: '仅兜底',
    role: '角色',
    provider: 'Provider',
    model: '模型',
    weight: '权重',
    upstreamProtocol: '上游协议',
    selectProvider: '选择…',
    selectModel: '选择模型…',
    autoProtocol: '（自动，无需配置）',
    endpoint: 'Endpoint',
    builtinEndpoint: '内置默认 Endpoint',
    noModels: '该 Provider 还没有模型，请先在 Provider 详情添加。',
    deleteAria: '删除 target'
  },
  dialog: {
    createTitle: '添加虚拟模型',
    editTitle: '编辑虚拟模型',
    description: '虚拟模型映射到多个上游 target，按权重负载均衡，失败时按策略重试与兜底。',
    enabledLabel: '启用该虚拟模型',
    enabledHint: '停用后该虚拟模型立即不可用'
  },
  form: {
    name: '名称',
    nameHint: '（写入日志 virtual_model_name）',
    virtualModelId: '虚拟模型 ID',
    virtualModelIdHint: '（客户端请求的 model）',
    fallbackPolicy: 'Fallback 策略',
    retryCount: '重试次数',
    retryPolicy: '退避策略',
    baseBackoff: '基础退避 (ms)',
    maxBackoff: '最大退避 (ms)'
  },
  policy: {
    fallbackFirst: 'fallback-first · 每轮先尝试全部 LB，再 fallback，整体循环重试',
    retryFirst: 'retry-first · 单个 target 重试 N 次后切换，最后 fallback',
    exponential: '指数退避',
    fixed: '固定退避'
  },
  targets: {
    title: 'Targets',
    descriptionPrefix: '每个 target 不需要选择上游协议：入口请求按',
    descriptionBold: '原协议',
    descriptionMiddle: '转发，上游不支持时按',
    descriptionSuffix: '的固定顺序取第一个可用协议并自动转换。协议由所选模型 / Provider 的配置决定，Endpoint 同源。'
  },
  toast: {
    modelsLoadFailed: '加载模型失败：{error}',
    virtualModelRequired: '请填写虚拟模型 ID',
    targetRequired: '至少配置一个 target',
    targetIncomplete: '每个 target 都需要 provider 和模型',
    created: '虚拟模型已创建',
    updated: '虚拟模型已更新',
    deleted: '虚拟模型已删除'
  },
  delete: {
    title: '删除虚拟模型',
    description: '确定删除虚拟模型 {name}？使用该模型的客户端将立即失败。'
  }
};
