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
  .login { min-height: 100vh; display: grid; place-items: center; padding: 20px; background: #eef2f7; }
  .card { width: min(420px, 100%); background: #fff; border: 1px solid #cfd9e5; border-radius: 2px; padding: 24px; box-shadow: 0 16px 40px rgba(15,23,42,.07); display: flex; flex-direction: column; gap: 14px; }
  .logo { width: 40px; height: 40px; border-radius: 2px; background: #2563eb; color: #fff; display: grid; place-items: center; font-weight: 800; font-size: 20px; }
  h1 { margin: 2px 0 0; font-size: 20px; }
  p { margin: -6px 0 4px; color: #64748b; font-size: 13px; line-height: 1.6; }
  .full { justify-content: center; padding: 10px; margin-top: 4px; }
  .error { color: #b91c1c; background: #fef2f2; border: 1px solid #fecaca; border-radius: 2px; padding: 8px 10px; font-size: 12.5px; }
  code { background: #f1f5f9; padding: 1px 4px; border-radius: 2px; }
</style>
