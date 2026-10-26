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
    padding: 12px 16px; border-radius: 10px; background: #fff; color: #0f172a;
    border: 1px solid #e2e8f0; box-shadow: 0 12px 30px rgba(15, 23, 42, .12);
    font-size: 13px; max-width: 380px;
  }
  .toast.error { background: #fff7f7; border-color: #fecaca; color: #b91c1c; }
  .toast.success { background: #f2fdf8; border-color: #bbf7d0; color: #047857; }
</style>
