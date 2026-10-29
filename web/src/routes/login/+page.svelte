<script>
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api, setToken } from '$lib/api';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Label } from '$lib/components/ui/label/index.js';
  import { Alert, AlertDescription } from '$lib/components/ui/alert/index.js';
  import { Spinner } from '$lib/components/ui/spinner/index.js';
  import ThemeToggle from '$lib/components/app/theme-toggle.svelte';
  import ShieldCheckIcon from '@lucide/svelte/icons/shield-check';
  import ZapIcon from '@lucide/svelte/icons/zap';
  import ShuffleIcon from '@lucide/svelte/icons/shuffle';
  import CoinsIcon from '@lucide/svelte/icons/coins';
  import ArrowRightIcon from '@lucide/svelte/icons/arrow-right';
  import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';

  const next = $derived(page.url.searchParams.get('next') || '/logs');

  const highlights = [
    { icon: ZapIcon, label: '统一 LLM 入口，多协议互转' },
    { icon: ShuffleIcon, label: '加权负载均衡与自动 fallback' },
    { icon: CoinsIcon, label: '逐请求 token 与成本核算' }
  ];

  let token = $state('');
  let busy = $state(false);
  let error = $state('');

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

<svelte:head>
  <title>登录 · aigw</title>
</svelte:head>

<div class="grid min-h-svh place-items-center bg-muted/30 p-4">
  <div class="absolute right-4 top-4">
    <ThemeToggle />
  </div>

  <Card.Root class="w-full max-w-3xl gap-0 overflow-hidden py-0">
    <div class="grid md:grid-cols-[1.05fr_1fr]">
      <div
        class="relative hidden flex-col justify-between gap-10 border-r border-border/60 bg-gradient-to-br from-emerald-600 via-emerald-700 to-teal-800 p-8 text-emerald-50 md:flex"
      >
        <div
          class="pointer-events-none absolute inset-0 opacity-25 [background-image:radial-gradient(circle_at_1px_1px,rgba(255,255,255,0.55)_1px,transparent_0)] [background-size:16px_16px]"
        >
        </div>
        <div class="relative space-y-5">
          <div class="flex items-center gap-2.5">
            <div
              class="grid size-9 place-items-center rounded-lg bg-white/15 text-base font-bold ring-1 ring-white/25 backdrop-blur"
            >
              A
            </div>
            <div class="leading-tight">
              <div class="text-sm font-semibold tracking-tight">aigw</div>
              <div class="text-[10px] uppercase tracking-[0.18em] text-emerald-100/70">
                AI Gateway
              </div>
            </div>
          </div>
          <div class="space-y-1.5">
            <h1 class="text-xl font-semibold leading-tight tracking-tight">
              一个入口，<br />管理所有 LLM 流量
            </h1>
            <p class="text-xs leading-relaxed text-emerald-50/80">
              OpenAI / Anthropic 双协议互通，健康检查、重试与成本观测开箱即用。
            </p>
          </div>
        </div>
        <ul class="relative space-y-2.5">
          {#each highlights as item (item.label)}
            <li class="flex items-center gap-2 text-xs text-emerald-50/90">
              <item.icon class="size-3.5 shrink-0 text-emerald-200" />
              <span>{item.label}</span>
            </li>
          {/each}
        </ul>
      </div>

      <div class="flex flex-col justify-center gap-5 p-6 sm:p-8">
        <div class="space-y-1">
          <div class="flex items-center gap-2">
            <ShieldCheckIcon class="size-4 text-primary" />
            <h2 class="text-sm font-semibold tracking-tight">管理控制台登录</h2>
          </div>
          <p class="text-xs text-muted-foreground">
            输入服务端配置的 <code class="rounded bg-muted px-1 py-0.5 font-mono">server.admin_token</code>。
          </p>
        </div>

        <form class="space-y-4" onsubmit={submit}>
          <div class="grid gap-1.5">
            <Label for="token">Admin Token</Label>
            <Input
              id="token"
              type="password"
              bind:value={token}
              autocomplete="current-password"
              placeholder="admin token"
              class="h-9"
            />
          </div>

          {#if error}
            <Alert variant="destructive">
              <TriangleAlertIcon />
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          {/if}

          <Button type="submit" class="h-9 w-full gap-2" disabled={busy || !token.trim()}>
            {#if busy}
              <Spinner class="size-3.5" />
              验证中…
            {:else}
              进入控制台
              <ArrowRightIcon />
            {/if}
          </Button>
        </form>

        <div class="flex items-center gap-2 text-[11px] text-muted-foreground">
          <span class="size-1.5 rounded-full bg-emerald-500"></span>
          Token 仅保存在浏览器本地，不会写入服务端。
        </div>
      </div>
    </div>
  </Card.Root>
</div>
