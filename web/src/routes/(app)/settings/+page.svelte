<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { toast } from 'svelte-sonner';

  import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
  import SettingsIcon from '@lucide/svelte/icons/settings';
  import RouteIcon from '@lucide/svelte/icons/route';
  import LayersIcon from '@lucide/svelte/icons/layers';

  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Switch } from '$lib/components/ui/switch/index.js';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import { Alert, AlertDescription } from '$lib/components/ui/alert/index.js';
  import { Separator } from '$lib/components/ui/separator/index.js';
  import { Spinner } from '$lib/components/ui/spinner/index.js';

  import PageHeader from '$lib/components/app/page-header.svelte';
  import StatusBadge from '$lib/components/app/status-badge.svelte';

  let settings = $state({ auto_model_match: false });
  let loading = $state(true);
  let saving = $state(false);
  let error = $state('');

  async function load() {
    loading = true;
    error = '';
    try {
      settings = await api('/api/admin/settings');
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function toggleAutoModelMatch(next) {
    if (saving) return;
    const previous = settings.auto_model_match;
    settings = { ...settings, auto_model_match: next };
    saving = true;
    try {
      settings = await api('/api/admin/settings', {
        method: 'PUT',
        body: JSON.stringify(settings)
      });
      toast.success(next ? '已开启自动模型匹配' : '已关闭自动模型匹配');
    } catch (e) {
      settings = { ...settings, auto_model_match: previous };
      toast.error(e.message);
    } finally {
      saving = false;
    }
  }

  onMount(load);
</script>

<svelte:head>
  <title>Settings · aigw</title>
</svelte:head>

<PageHeader
  title="Settings"
  description="网关级运行配置。修改立即生效，无需重启服务。"
/>

{#if error}
  <Alert variant="destructive">
    <TriangleAlertIcon />
    <AlertDescription>{error}</AlertDescription>
  </Alert>
{/if}

{#if loading}
  <Skeleton class="h-48 w-full rounded-lg" />
{:else}
  <Card.Root class="gap-0">
    <Card.Header class="border-b">
      <div class="flex items-center gap-1.5">
        <SettingsIcon class="size-3.5 text-muted-foreground" />
        <Card.Title class="text-sm">模型自动匹配</Card.Title>
      </div>
      <Card.Description class="text-[11px]">
        控制请求模型名没有匹配到虚拟模型时网关的行为。
      </Card.Description>
    </Card.Header>
    <Card.Content class="space-y-3 pt-3">
      <div class="flex flex-wrap items-start justify-between gap-3 rounded-lg border p-3">
        <div class="min-w-0 space-y-1">
          <div class="flex flex-wrap items-center gap-1.5">
            <span class="text-xs font-medium">自动模型匹配</span>
            <StatusBadge tone={settings.auto_model_match ? 'success' : 'neutral'} dot>
              {settings.auto_model_match ? '已开启' : '已关闭'}
            </StatusBadge>
            {#if saving}
              <Spinner class="size-3" />
            {/if}
          </div>
          <p class="max-w-2xl text-[11px] leading-relaxed text-muted-foreground">
            开启后，当请求的模型名没有对应虚拟模型时，网关会按 Provider 顺序查找第一个
            <span class="font-mono">包含此模型名</span> 的 Provider 模型（对外 ID 为
            <span class="font-mono">provider/model</span>），并直接转发到该上游。
          </p>
        </div>
        <Switch
          checked={settings.auto_model_match}
          onCheckedChange={toggleAutoModelMatch}
          disabled={saving}
          aria-label="自动模型匹配"
        />
      </div>

      <div class="rounded-lg border bg-muted/30 p-3">
        <div class="mb-1.5 flex items-center gap-1.5">
          <RouteIcon class="size-3.5 text-muted-foreground" />
          <span class="text-xs font-medium">匹配示例</span>
        </div>
        <ul class="space-y-1 text-[11px] leading-relaxed text-muted-foreground">
          <li>
            请求模型
            <code class="rounded bg-background px-1 font-mono text-[10.5px]"
              >deepseek/deepseek-flash</code
            >
            且没有同名虚拟模型时：
          </li>
          <li class="pl-4">
            1. 依次遍历已启用的 Provider 模型；
          </li>
          <li class="pl-4">
            2. 取第一个对外 ID 包含该字符串的模型，例如
            <code class="rounded bg-background px-1 font-mono text-[10.5px]"
              >yllm/deepseek/deepseek-flash</code
            >；
          </li>
          <li class="pl-4">
            3. 使用该模型的 Provider 协议、Path、重试与代理配置发起上游请求。
          </li>
        </ul>
      </div>

      <Separator />

      <Alert>
        <LayersIcon />
        <AlertDescription>
          已存在的虚拟模型始终优先；自动匹配只在其后兜底，且不会写入任何虚拟模型。
          请求日志中的 <span class="font-mono">virtual_model_name</span> 会标记为
          <span class="font-mono">auto-match</span>，便于审计。
        </AlertDescription>
      </Alert>
    </Card.Content>
    <Card.Footer class="justify-end border-t">
      <Button variant="outline" size="sm" onclick={load} disabled={loading || saving}>
        重新加载
      </Button>
    </Card.Footer>
  </Card.Root>
{/if}
