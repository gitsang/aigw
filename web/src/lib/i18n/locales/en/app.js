/** Shell chrome: sidebar, header, dialogs and shared form widgets. */
export default {
  brand: 'AI Gateway',
  sidebarGroup: 'Console',
  logout: 'Sign out',
  theme: {
    toggle: 'Toggle theme'
  },
  confirm: {
    title: 'Confirm action',
    description: 'This action cannot be undone. Continue?',
    confirmText: 'Delete',
    cancel: 'Cancel',
    processing: 'Processing…'
  },
  headerRows: {
    label: 'Extra headers',
    add: 'Add',
    nameColumn: 'Name',
    valueColumn: 'Value',
    namePlaceholder: 'X-Custom-Header',
    valuePlaceholder: 'value',
    nameAria: '{label} name {index}',
    valueAria: '{label} value {index}',
    removeAria: 'Remove this header',
    empty: 'No headers yet'
  },
  error: {
    unauthorized: 'Unauthorized, please re-enter the Admin Token'
  }
};
