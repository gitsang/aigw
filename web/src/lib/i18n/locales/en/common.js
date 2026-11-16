/**
 * Shared console vocabulary: actions, states, units and language names.
 *
 * Keep this namespace small and generic; page-specific wording belongs to that
 * page's own namespace file.
 */
export default {
  language: {
    en: 'English',
    zh: '简体中文',
    label: 'Language'
  },
  action: {
    add: 'Add',
    save: 'Save',
    cancel: 'Cancel',
    close: 'Close',
    confirm: 'Confirm',
    delete: 'Delete',
    edit: 'Edit',
    create: 'Create',
    copy: 'Copy',
    refresh: 'Refresh',
    reload: 'Reload',
    search: 'Search',
    reset: 'Reset'
  },
  state: {
    loading: 'Loading…',
    saving: 'Saving…',
    processing: 'Processing…',
    empty: 'Nothing here yet',
    unknown: 'Unknown',
    enabled: 'Enabled',
    disabled: 'Disabled',
    on: 'On',
    off: 'Off'
  },
  unit: {
    tokens: 'tokens',
    ms: '{value}ms',
    seconds: '{value}s',
    minutes: '{value} min',
    hours: '{value} h',
    days: '{value} d'
  }
};
