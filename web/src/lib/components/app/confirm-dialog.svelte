<script>
  import * as AlertDialog from '$lib/components/ui/alert-dialog/index.js';

  let {
    open = $bindable(false),
    title = '确认操作',
    description = '此操作不可撤销，确定继续吗？',
    confirmText = '确认删除',
    busy = false,
    onconfirm = () => {}
  } = $props();
</script>

<AlertDialog.Root bind:open>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{title}</AlertDialog.Title>
      <AlertDialog.Description>{description}</AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel disabled={busy}>取消</AlertDialog.Cancel>
      <AlertDialog.Action variant="destructive" onclick={onconfirm} disabled={busy}>
        {busy ? '处理中…' : confirmText}
      </AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
