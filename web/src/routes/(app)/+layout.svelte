<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';

  import { getToken } from '$lib/api';
  import { navItemFor } from '$lib/nav.js';
  import * as Sidebar from '$lib/components/ui/sidebar/index.js';
  import * as Breadcrumb from '$lib/components/ui/breadcrumb/index.js';
  import { Separator } from '$lib/components/ui/separator/index.js';
  import { Spinner } from '$lib/components/ui/spinner/index.js';
  import AppSidebar from '$lib/components/app/app-sidebar.svelte';
  import ThemeToggle from '$lib/components/app/theme-toggle.svelte';

  let { children } = $props();

  let ready = $state(false);

  onMount(() => {
    if (!getToken()) goto('/login');
    else ready = true;
  });

  const current = $derived(navItemFor(page.url.pathname)?.label ?? '控制台');
</script>

{#if ready}
  <Sidebar.Provider>
    <AppSidebar />
    <Sidebar.Inset class="min-w-0 overflow-x-hidden">
      <header
        class="sticky top-0 z-20 flex h-12 shrink-0 items-center gap-2 border-b border-border/80 bg-background/95 px-3 backdrop-blur supports-[backdrop-filter]:bg-background/80"
      >
        <Sidebar.Trigger />
        <Separator orientation="vertical" class="h-4" />
        <Breadcrumb.Root>
          <Breadcrumb.List class="text-xs">
            <Breadcrumb.Item class="hidden sm:block">
              <Breadcrumb.Link href="/logs">控制台</Breadcrumb.Link>
            </Breadcrumb.Item>
            <Breadcrumb.Separator class="hidden sm:block" />
            <Breadcrumb.Item>
              <Breadcrumb.Page class="font-medium text-foreground">{current}</Breadcrumb.Page>
            </Breadcrumb.Item>
          </Breadcrumb.List>
        </Breadcrumb.Root>
        <div class="ml-auto flex items-center gap-1">
          <ThemeToggle />
        </div>
      </header>

      <div class="flex-1 px-3 py-4 md:px-5 md:py-5">
        <div class="mx-auto flex w-full max-w-[1500px] flex-col gap-4">
          {@render children?.()}
        </div>
      </div>
    </Sidebar.Inset>
  </Sidebar.Provider>
{:else}
  <div class="grid min-h-svh place-items-center bg-muted/30">
    <Spinner class="size-5 text-muted-foreground" />
  </div>
{/if}
