<script>
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { setToken } from '$lib/api';

  const nav = [
    { href: '/logs', label: '日志与统计', icon: '▤' },
    { href: '/providers', label: 'Model Provider', icon: '◈' },
    { href: '/routing', label: 'Routing', icon: '⇄' },
    { href: '/keys', label: 'API Keys', icon: '⚿' }
  ];

  function logout() {
    setToken('');
    goto('/login');
  }
</script>

<aside>
  <div class="brand">
    <div class="logo">A</div>
    <div>
      <div class="name">aigw</div>
      <div class="sub">AI Gateway</div>
    </div>
  </div>
  <nav>
    {#each nav as item}
      <a href={item.href} class:active={$page.url.pathname.startsWith(item.href)}>
        <span class="icon">{item.icon}</span>{item.label}
      </a>
    {/each}
  </nav>
  <div class="foot">
    <a href="/healthz" target="_blank" rel="noreferrer" class="doc">服务健康状态</a>
    <button class="logout" on:click={logout}>退出管理</button>
  </div>
</aside>

<style>
  aside {
    width: 232px; min-height: 100vh; background: #0b1220; color: #cbd5e1;
    display: flex; flex-direction: column; padding: 18px 14px; position: sticky; top: 0;
  }
  .brand { display: flex; gap: 11px; align-items: center; padding: 6px 8px 22px; }
  .logo {
    width: 36px; height: 36px; border-radius: 10px; background: linear-gradient(135deg, #38bdf8, #2563eb);
    color: #fff; display: grid; place-items: center; font-weight: 800; font-size: 18px;
  }
  .name { color: #fff; font-weight: 750; font-size: 17px; letter-spacing: -.02em; }
  .sub { font-size: 10.5px; color: #64748b; letter-spacing: .06em; text-transform: uppercase; }
  nav { display: flex; flex-direction: column; gap: 4px; }
  nav a {
    display: flex; align-items: center; gap: 10px; padding: 10px 12px; border-radius: 9px;
    font-size: 13px; color: #94a3b8; font-weight: 550;
  }
  nav a:hover { background: #111c30; color: #e2e8f0; }
  nav a.active { background: #172554; color: #bfdbfe; }
  .icon { width: 18px; text-align: center; font-size: 14px; }
  .foot { margin-top: auto; display: flex; flex-direction: column; gap: 8px; padding: 10px 8px 2px; }
  .doc { font-size: 11.5px; color: #64748b; }
  .logout { background: none; border: 0; color: #94a3b8; text-align: left; padding: 0; cursor: pointer; font-size: 11.5px; }
  .logout:hover { color: #fecaca; }
</style>
