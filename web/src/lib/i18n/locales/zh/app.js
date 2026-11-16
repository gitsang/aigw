/** 外壳界面：侧边栏、顶栏、对话框与共用表单部件。 */
export default {
  brand: 'AI Gateway',
  sidebarGroup: '控制台',
  sidebar: {
    toggle: '切换侧边栏'
  },
  logout: '退出管理',
  theme: {
    toggle: '切换主题'
  },
  confirm: {
    title: '确认操作',
    description: '此操作不可撤销，确定继续吗？',
    confirmText: '确认删除',
    cancel: '取消',
    processing: '处理中…'
  },
  headerRows: {
    label: '额外 Header',
    add: '添加',
    nameColumn: 'Name',
    valueColumn: 'Value',
    namePlaceholder: 'X-Custom-Header',
    valuePlaceholder: 'value',
    nameAria: '{label} 名称 {index}',
    valueAria: '{label} 值 {index}',
    removeAria: '删除该 Header',
    empty: '暂无 Header'
  },
  error: {
    unauthorized: '未授权，请重新输入 Admin Token'
  }
};
