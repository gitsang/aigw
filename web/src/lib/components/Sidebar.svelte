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
    <a href="/healthz" target="_blank" rel="noreferrer" class="doc">
      <span class="status-dot"></span>服务健康状态
    </a>
    <button class="logout" on:click={logout}>退出管理</button>
  </div>
</aside>

<style>
  aside {
    width: 236px; min-height: 100vh; color: #64748b;
    display: flex; flex-direction: column; padding: 18px 14px;
    position: sticky; top: 0; align-self: flex-start;
    background: linear-gradient(180deg, #ffffff 0%, #f8fbff 100%);
    border-right: 1px solid #e7eef7;
  }
  .brand { display: flex; gap: 11px; align-items: center; padding: 6px 8px 24px; }
  .logo {
    width: 38px; height: 38px; border-radius: 11px;
    background: linear-gradient(135deg, #38bdf8, #2563eb);
    color: #fff; display: grid; place-items: center; font-weight: 800; font-size: 19px;
    box-shadow: 0 7px 18px rgba(37, 99, 235, .2);
  }
  .name { color: #0f172a; font-weight: 750; font-size: 17px; letter-spacing: -.02em; }
  .sub { font-size: 10.5px; color: #94a3b8; letter-spacing: .08em; text-transform: uppercase; }
  nav { display: flex; flex-direction: column; gap: 5px; }
  nav a {
    display: flex; align-items: center; gap: 10px; padding: 10px 12px; border-radius: 10px;
    font-size: 13px; color: #64748b; font-weight: 550; transition: .15s ease;
  }
  nav a:hover { background: #f2f7fe; color: #1e293b; }
  nav a.active { background: #eaf2ff; color: #1d4ed8; font-weight: 650; }
  .icon { width: 18px; text-align: center; font-size: 14px; }
  .foot { margin-top: auto; display: flex; flex-direction: column; gap: 10px; padding: 18px 8px 2px; border-top: 1px solid #edf2f8; }
  .doc { font-size: 11.5px; color: #64748b; display: flex; align-items: center; gap: 7px; }
  .status-dot { width: 7px; height: 7px; border-radius: 50%; background: #10b981; box-shadow: 0 0 0 3px #d1fae5; }
  .logout { background: none; border: 0; color: #94a3b8; text-align: left; padding: 0; cursor: pointer; font-size: 11.5px; }
  .logout:hover { color: #dc2626; }
  @media (max-width: 800px) {
    aside { width: 100%; min-height: auto; position: static; border-right: 0; border-bottom: 1px solid #e7eef7; padding: 12px 14px; }
    .brand { padding: 2px 6px 12px; }
    nav { flex-direction: row; overflow-x: auto; }
    nav a { white-space: nowrap; }
    .foot { display: none; }
  }
</style>
