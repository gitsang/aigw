<script>
  export let open = false;
  export let title = '';
  export let wide = false;
  export let onclose = () => {};
</script>

{#if open}
  <div class="overlay" role="presentation" on:click={(e) => e.target === e.currentTarget && onclose()}>
    <div class="modal" class:wide role="dialog" aria-modal="true" aria-label={title}>
      <header>
        <h2>{title}</h2>
        <button class="icon" type="button" on:click={onclose} aria-label="关闭">✕</button>
      </header>
      <div class="body"><slot /></div>
      {#if $$slots.footer}<footer><slot name="footer" /></footer>{/if}
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed; inset: 0; background: rgba(15, 23, 42, .48); z-index: 50;
    display: flex; align-items: flex-start; justify-content: center; padding: 5vh 16px; overflow-y: auto;
  }
  .modal {
    width: min(640px, 100%); background: #fff; border: 1px solid #cfd9e5; border-radius: 0; box-shadow: 0 18px 50px rgba(15,23,42,.24);
  }
  .modal.wide { width: min(980px, 100%); }
  header { display: flex; justify-content: space-between; align-items: center; padding: 14px 18px; border-bottom: 1px solid #e2e8f0; }
  h2 { margin: 0; font-size: 17px; }
  .body { padding: 16px 18px; }
  footer { padding: 12px 18px; border-top: 1px solid #e2e8f0; display: flex; justify-content: flex-end; gap: 8px; }
  .icon { border: 0; background: #f1f5f9; border-radius: 2px; width: 30px; height: 30px; cursor: pointer; }
</style>
