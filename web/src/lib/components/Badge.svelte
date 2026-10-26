<script>
  export let tone = 'neutral';
  export let dot = false;
</script>
<span class="badge {tone}">{#if dot}<i></i>{/if}<slot /></span>
<style>
  .badge { display: inline-flex; align-items: center; gap: 5px; border-radius: 999px; padding: 2px 9px; font-size: 11.5px; font-weight: 600; white-space: nowrap; }
  .badge i { width: 6px; height: 6px; border-radius: 50%; background: currentColor; }
  .neutral { background: #f1f5f9; color: #475569; }
  .green { background: #d1fae5; color: #047857; }
  .red { background: #fee2e2; color: #b91c1c; }
  .amber { background: #fef3c7; color: #b45309; }
  .blue { background: #dbeafe; color: #1d4ed8; }
  .violet { background: #ede9fe; color: #6d28d9; }
</style>
