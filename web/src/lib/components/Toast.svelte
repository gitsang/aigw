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
    padding: 12px 16px; border-radius: 10px; background: #0f172a; color: #fff;
    box-shadow: 0 10px 30px rgba(15, 23, 42, .25); font-size: 13px; max-width: 380px;
  }
  .toast.error { background: #b91c1c; }
  .toast.success { background: #047857; }
</style>
