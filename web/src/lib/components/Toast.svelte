<script>
  import { toasts } from '$lib/toast';
</script>

<div class="toast-stack">
  {#each $toasts as t (t.id)}
    <div class="toast {t.type}">{t.message}</div>
  {/each}
</div>

<style>
  .toast-stack {
    position: fixed; right: 20px; bottom: 20px; z-index: 100;
    display: flex; flex-direction: column; gap: 8px;
  }
  .toast {
    padding: 10px 13px; border-radius: 2px; background: #fff; color: #0f172a;
    border: 1px solid #dce4ee; box-shadow: 0 10px 24px rgba(15, 23, 42, .1);
    font-size: 13px; max-width: 380px;
  }
  .toast.error { background: #fff7f7; border-color: #fecaca; color: #b91c1c; }
  .toast.success { background: #f2fdf8; border-color: #bbf7d0; color: #047857; }
</style>
