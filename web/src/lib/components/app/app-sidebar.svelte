<script>
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { setToken } from '$lib/api';
  import { navItems } from '$lib/nav.js';
  import * as Sidebar from '$lib/components/ui/sidebar/index.js';
  import ActivityIcon from '@lucide/svelte/icons/activity';
  import LogOutIcon from '@lucide/svelte/icons/log-out';

  const current = $derived(page.url.pathname);

  function logout() {
    setToken('');
    goto('/login');
  }
</script>

<Sidebar.Root collapsible="icon">
  <Sidebar.Header class="border-b border-sidebar-border">
    <Sidebar.Menu>
      <Sidebar.MenuItem>
        <Sidebar.MenuButton size="lg" class="data-active:bg-transparent" tooltipContent="aigw">
          {#snippet child({ props })}
            <a href="/logs" {...props}>
              <div
                class="grid size-8 shrink-0 place-items-center rounded-md bg-primary text-primary-foreground shadow-sm"
              >
                <span class="text-sm font-bold">A</span>
              </div>
              <div class="grid flex-1 text-left leading-tight">
                <span class="truncate text-sm font-semibold text-sidebar-foreground">aigw</span>
                <span
                  class="truncate text-[10px] uppercase tracking-[0.14em] text-muted-foreground"
                >
                  AI Gateway
                </span>
              </div>
            </a>
          {/snippet}
        </Sidebar.MenuButton>
      </Sidebar.MenuItem>
    </Sidebar.Menu>
  </Sidebar.Header>

  <Sidebar.Content>
    <Sidebar.Group>
      <Sidebar.GroupLabel>控制台</Sidebar.GroupLabel>
      <Sidebar.GroupContent>
        <Sidebar.Menu>
          {#each navItems as item (item.href)}
            <Sidebar.MenuItem>
              <Sidebar.MenuButton
                isActive={current === item.href || current.startsWith(`${item.href}/`)}
                tooltipContent={item.label}
              >
                {#snippet child({ props })}
                  <a href={item.href} {...props}>
                    <item.icon />
                    <span>{item.label}</span>
                  </a>
                {/snippet}
              </Sidebar.MenuButton>
            </Sidebar.MenuItem>
          {/each}
        </Sidebar.Menu>
      </Sidebar.GroupContent>
    </Sidebar.Group>
  </Sidebar.Content>

  <Sidebar.Footer class="border-t border-sidebar-border">
    <Sidebar.Menu>
      <Sidebar.MenuItem>
        <Sidebar.MenuButton size="sm" tooltipContent="服务健康状态">
          {#snippet child({ props })}
            <a href="/healthz" target="_blank" rel="noreferrer" {...props}>
              <ActivityIcon />
              <span>服务健康状态</span>
              <span
                class="ml-auto size-1.5 shrink-0 rounded-full bg-emerald-500 shadow-[0_0_0_3px] shadow-emerald-500/20"
              ></span>
            </a>
          {/snippet}
        </Sidebar.MenuButton>
      </Sidebar.MenuItem>
      <Sidebar.MenuItem>
        <Sidebar.MenuButton size="sm" tooltipContent="退出管理" onclick={logout}>
          <LogOutIcon />
          <span>退出管理</span>
        </Sidebar.MenuButton>
      </Sidebar.MenuItem>
    </Sidebar.Menu>
  </Sidebar.Footer>
  <Sidebar.Rail />
</Sidebar.Root>
