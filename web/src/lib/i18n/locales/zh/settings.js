/** 设置页。 */
export default {
  title: '设置 · aigw',
  heading: 'Settings',
  description: '网关级运行配置。修改立即生效，无需重启服务。',
  autoMatch: {
    title: '模型自动匹配',
    description: '控制请求模型名没有匹配到虚拟模型时网关的行为。',
    label: '自动模型匹配',
    on: '已开启',
    off: '已关闭',
    body: '开启后，当请求的模型名没有对应虚拟模型时，网关会按 Provider 顺序查找第一个包含此模型名的 Provider 模型（对外 ID 形如 provider/model），并直接转发到该上游。',
    enabledToast: '已开启自动模型匹配',
    disabledToast: '已关闭自动模型匹配'
  },
  example: {
    title: '匹配示例',
    requestModel: '请求模型',
    withoutVirtualModel: '且没有同名虚拟模型时：',
    step1: '1. 依次遍历已启用的 Provider 模型；',
    step2: '2. 取第一个对外 ID 包含该字符串的模型，例如',
    step2Tail: '；',
    step3: '3. 使用该模型的 Provider 协议、Path、重试与代理配置发起上游请求。'
  },
  note: '已存在的虚拟模型始终优先；自动匹配只在其后兜底，且不会写入任何虚拟模型。请求日志中的 ',
  noteKey: ' 会标记为 ',
  noteTail: '，便于审计。'
};
