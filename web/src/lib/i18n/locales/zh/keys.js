/** Chinese messages for the API keys console page. */
export default {
  title: 'API Keys · aigw',
  heading: 'API Keys',
  description: '客户端使用这些 Key 调用网关。Key 仅展示一次，数据库只保存 SHA-256 哈希。',
  create: '创建 API Key',
  list: {
    title: '密钥列表',
    count: '{count} 个'
  },
  column: {
    name: '名称',
    app: 'App',
    keyPrefix: 'Key Prefix',
    allowedModels: '允许模型',
    status: '状态',
    lastUsed: '最后使用',
    expiresAt: '过期时间',
    createdAt: '创建时间',
    actions: '操作'
  },
  empty: {
    title: '还没有 API Key',
    descriptionBefore: '创建后即可通过',
    descriptionAfter: '调用网关。'
  },
  allModels: '全部模型',
  never: '从未',
  noExpiry: '永不过期',
  toast: {
    updated: 'API Key 已更新',
    copied: '已复制到剪贴板',
    copyFailed: '复制失败，请手动选择复制',
    deleted: 'API Key 已删除'
  },
  dialog: {
    createdTitle: 'API Key 已创建',
    editTitle: '编辑 API Key',
    createTitle: '创建 API Key',
    createdDescription: '请立即复制并妥善保存，关闭后无法再次查看完整 Key。',
    formDescription: '配置名称、业务标识、可用模型范围与有效期。',
    copied: '已复制',
    example: '调用示例',
    done: '完成'
  },
  form: {
    name: '名称',
    namePlaceholder: 'production-app',
    app: 'App / 业务标识',
    appPlaceholder: 'my-app',
    appHint: '会展示在日志与统计中，便于按业务筛选。',
    allowedModels: '允许的模型（逗号分隔，留空允许全部）',
    expiresAt: '过期时间（可选）',
    enabled: '启用该 Key',
    enabledHint: '停用后客户端立即无法访问'
  },
  delete: {
    title: '删除 API Key',
    description: '确定删除 {name}？使用该 Key 的客户端将立即无法访问。'
  }
};
