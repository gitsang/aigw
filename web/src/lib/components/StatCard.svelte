<script>
  export let label = '';
  export let value = '';
  export let hint = '';
  export let tone = 'blue';
</script>
<div class="card">
  <div class="label">{label}</div>
  <div class="value {tone}">{value}</div>
  {#if hint}<div class="hint">{hint}</div>{/if}
</div>
<style>
  .card { background: #fff; border: 1px solid #e2e8f0; border-radius: 14px; padding: 16px 18px; min-width: 0; }
  .label { font-size: 12px; color: #64748b; font-weight: 600; letter-spacing: .02em; }
  .value { font-size: 25px; font-weight: 750; margin-top: 8px; letter-spacing: -.02em; }
  .blue { color: #1d4ed8; } .green { color: #047857; } .violet { color: #6d28d9; } .amber { color: #b45309; }
  .hint { margin-top: 6px; font-size: 11.5px; color: #94a3b8; }
</style>
