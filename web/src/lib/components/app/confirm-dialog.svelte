<script>
  import * as AlertDialog from '$lib/components/ui/alert-dialog/index.js';
  import { t } from '$lib/i18n/index.svelte.js';

  let {
    open = $bindable(false),
    title = '',
    description = '',
    confirmText = '',
    busy = false,
    onconfirm = () => {}
  } = $props();

  const heading = $derived(title || t('app.confirm.title'));
  const body = $derived(description || t('app.confirm.description'));
  const action = $derived(confirmText || t('app.confirm.confirmText'));
</script>

<AlertDialog.Root bind:open>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{heading}</AlertDialog.Title>
      <AlertDialog.Description>{body}</AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel disabled={busy}>{t('app.confirm.cancel')}</AlertDialog.Cancel>
      <AlertDialog.Action variant="destructive" onclick={onconfirm} disabled={busy}>
        {busy ? t('app.confirm.processing') : action}
      </AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
