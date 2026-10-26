<script>
  import Modal from './Modal.svelte';
  export let open = false;
  export let title = '确认操作';
  export let message = '此操作不可撤销，确定继续吗？';
  export let confirmText = '确认删除';
  export let busy = false;
  export let onconfirm = () => {};
  export let onclose = () => {};
</script>
<Modal {open} {title} {onclose}>
  <p class="msg">{message}</p>
  <svelte:fragment slot="footer">
    <button class="btn ghost" type="button" on:click={onclose} disabled={busy}>取消</button>
    <button class="btn danger" type="button" on:click={onconfirm} disabled={busy}>{busy ? '处理中…' : confirmText}</button>
  </svelte:fragment>
</Modal>
<style>.msg { margin: 0; color: #475569; line-height: 1.7; }</style>
