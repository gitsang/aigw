<script>
  import { goto } from '$app/navigation';
  import { api, setToken } from '$lib/api';

  let token = '';
  let busy = false;
  let error = '';
  const next = typeof location !== 'undefined' ? new URLSearchParams(location.search).get('next') || '/logs' : '/logs';

  async function submit(event) {
    event.preventDefault();
    error = '';
    busy = true;
    try {
      setToken(token.trim());
      await api('/api/admin/session');
      goto(next);
    } catch (e) {
      setToken('');
      error = e.message || '登录失败';
    } finally {
      busy = false;
    }
  }
</script>

<svelte:head><title>登录 · aigw</title></svelte:head>

<div class="login">
  <form class="card" on:submit={submit}>
    <div class="logo">A</div>
    <h1>aigw 管理控制台</h1>
    <p>输入服务端配置的 <code>server.admin_token</code>。</p>
    <div class="field">
      <label for="token">Admin Token</label>
      <input id="token" class="input" type="password" bind:value={token} autocomplete="current-password" placeholder="admin token" />
    </div>
    {#if error}<div class="error">{error}</div>{/if}
    <button class="btn primary full" type="submit" disabled={busy || !token.trim()}>
      {busy ? '验证中…' : '进入控制台'}
    </button>
  </form>
</div>

<style>
  .login { min-height: 100vh; display: grid; place-items: center; padding: 20px; background: radial-gradient(circle at 20% 10%, #dbeafe, transparent 35%), radial-gradient(circle at 90% 80%, #ede9fe, transparent 40%), #f8fafc; }
  .card { width: min(400px, 100%); background: #fff; border: 1px solid #e2e8f0; border-radius: 18px; padding: 32px; box-shadow: 0 24px 60px rgba(15,23,42,.08); display: flex; flex-direction: column; gap: 16px; }
  .logo { width: 44px; height: 44px; border-radius: 12px; background: linear-gradient(135deg, #38bdf8, #2563eb); color: #fff; display: grid; place-items: center; font-weight: 800; font-size: 22px; }
  h1 { margin: 2px 0 0; font-size: 20px; }
  p { margin: -6px 0 4px; color: #64748b; font-size: 13px; line-height: 1.6; }
  .full { justify-content: center; padding: 10px; margin-top: 4px; }
  .error { color: #b91c1c; background: #fef2f2; border: 1px solid #fecaca; border-radius: 8px; padding: 9px 11px; font-size: 12.5px; }
  code { background: #f1f5f9; padding: 1px 5px; border-radius: 4px; }
</style>
