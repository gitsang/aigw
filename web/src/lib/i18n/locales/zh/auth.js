/** 登录页。 */
export default {
  title: '登录 · aigw',
  heroTitleLine1: '一个入口，',
  heroTitleLine2: '管理所有 LLM 流量',
  heroSubtitle: 'OpenAI / Anthropic 双协议互通，健康检查、重试与成本观测开箱即用。',
  highlight: {
    unified: '统一 LLM 入口，多协议互转',
    routing: '加权负载均衡与自动 fallback',
    cost: '逐请求 token 与成本核算'
  },
  heading: '管理控制台登录',
  hint: '输入服务端配置的',
  hintConfigKey: 'server.admin_token',
  tokenLabel: 'Admin Token',
  tokenPlaceholder: 'admin token',
  submit: '进入控制台',
  verifying: '验证中…',
  failed: '登录失败',
  localOnly: 'Token 仅保存在浏览器本地，不会写入服务端。',
  unauthorized: '未授权，请重新输入 Admin Token'
};
