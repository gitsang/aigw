<script>
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { setToken } from '$lib/api';
  import { navItems } from '$lib/nav.js';
  import { t } from '$lib/i18n/index.svelte.js';
  import * as Sidebar from '$lib/components/ui/sidebar/index.js';
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
                  {t('app.brand')}
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
      <Sidebar.GroupLabel>{t('app.sidebarGroup')}</Sidebar.GroupLabel>
      <Sidebar.GroupContent>
        <Sidebar.Menu>
          {#each navItems as item (item.href)}
            <Sidebar.MenuItem>
              <Sidebar.MenuButton
                isActive={current === item.href || current.startsWith(`${item.href}/`)}
                tooltipContent={t(item.labelKey)}
              >
                {#snippet child({ props })}
                  <a href={item.href} {...props}>
                    <item.icon />
                    <span>{t(item.labelKey)}</span>
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
        <Sidebar.MenuButton size="sm" tooltipContent={t('app.logout')} onclick={logout}>
          <LogOutIcon />
          <span>{t('app.logout')}</span>
        </Sidebar.MenuButton>
      </Sidebar.MenuItem>
    </Sidebar.Menu>
  </Sidebar.Footer>
  <Sidebar.Rail />
</Sidebar.Root>
