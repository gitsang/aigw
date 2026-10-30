<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { toast } from 'svelte-sonner';

  import PlusIcon from '@lucide/svelte/icons/plus';
  import SearchIcon from '@lucide/svelte/icons/search';
  import BoxesIcon from '@lucide/svelte/icons/boxes';
  import Trash2Icon from '@lucide/svelte/icons/trash-2';
  import SaveIcon from '@lucide/svelte/icons/save';
  import ServerIcon from '@lucide/svelte/icons/server';
  import ShieldCheckIcon from '@lucide/svelte/icons/shield-check';
  import GlobeIcon from '@lucide/svelte/icons/globe';
  import RepeatIcon from '@lucide/svelte/icons/repeat';
  import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
  import EyeIcon from '@lucide/svelte/icons/eye';
  import EyeOffIcon from '@lucide/svelte/icons/eye-off';
  import RotateCcwIcon from '@lucide/svelte/icons/rotate-ccw';
  import LayersIcon from '@lucide/svelte/icons/layers';
  import BoxIcon from '@lucide/svelte/icons/box';

  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Label } from '$lib/components/ui/label/index.js';
  import { Switch } from '$lib/components/ui/switch/index.js';
  import { Checkbox } from '$lib/components/ui/checkbox/index.js';
  import * as Select from '$lib/components/ui/select/index.js';
  import * as Tabs from '$lib/components/ui/tabs/index.js';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import * as Empty from '$lib/components/ui/empty/index.js';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import { Alert, AlertDescription } from '$lib/components/ui/alert/index.js';
  import { Separator } from '$lib/components/ui/separator/index.js';
  import { Spinner } from '$lib/components/ui/spinner/index.js';
  import * as InputGroup from '$lib/components/ui/input-group/index.js';

  import PageHeader from '$lib/components/app/page-header.svelte';
  import StatusBadge from '$lib/components/app/status-badge.svelte';
  import ConfirmDialog from '$lib/components/app/confirm-dialog.svelte';
  import HeaderRows from '$lib/components/app/header-rows.svelte';
  import ProviderModels from '$lib/components/app/provider-models.svelte';

  import {
    API_FORMATS,
    blankProvider,
    providerToForm,
    formToPayload,
    formSignature,
    validateProvider,
    mapToRows,
    rowsToMap,
    formatDuration
  } from '$lib/provider-form.js';

  const AUTH_OPTIONS = [
    { value: 'bearer', label: 'Authorization: Bearer' },
    { value: 'x-api-key', label: 'x-api-key' },
    { value: 'none', label: '无认证' }
  ];
  const PROXY_OPTIONS = [
    { value: 'none', label: '不使用代理' },
    { value: 'http', label: 'HTTP / HTTPS' },
    { value: 'socks5', label: 'SOCKS5' }
  ];
  const TABS = [
    { value: 'basic', label: '基础', icon: ServerIcon },
    { value: 'request', label: '请求', icon: RepeatIcon },
    { value: 'proxy', label: '代理', icon: GlobeIcon },
    { value: 'models', label: '模型', icon: BoxIcon }
  ];

  let providers = $state([]);
  let loading = $state(true);
  let error = $state('');
  let search = $state('');
  let selectedId = $state(null);

  let tab = $state('basic');
  let form = $state(blankProvider());
  let headerRows = $state([]);
  let baseline = $state('');
  let saving = $state(false);
  let showApiKey = $state(false);
  let showProxyPassword = $state(false);

  let providerModels = $state([]);

  let createOpen = $state(false);
  let createName = $state('');
  let deleteTarget = $state(null);
  let deleting = $state(false);

  const selected = $derived(providers.find((p) => p.id === selectedId) ?? null);
  // Any nested edit (API format toggles, header rows) changes this signature.
  const signature = $derived(formSignature(form, headerRows));
  const dirty = $derived(baseline !== '' && signature !== baseline);
  const edited = $derived(
    providers.filter((p) => {
      const q = search.trim().toLowerCase();
      if (!q) return true;
      return (
        p.name.toLowerCase().includes(q) ||
        (p.slug || '').toLowerCase().includes(q) ||
        (p.description || '').toLowerCase().includes(q) ||
        (p.base_url || '').toLowerCase().includes(q)
      );
    })
  );
  const proxyEnabled = $derived(form.proxy_type !== '' && form.proxy_type !== 'none');

  function applyProvider(provider) {
    selectedId = provider.id;
    providerModels = [];
    form = providerToForm(provider);
    headerRows = mapToRows(provider.headers);
    baseline = formSignature(form, headerRows);
    showApiKey = false;
    showProxyPassword = false;
  }

  async function load() {
    loading = true;
    error = '';
    try {
      const items = await api('/api/admin/providers');
      providers = items;
      if (!items.length) {
        selectedId = null;
        form = blankProvider();
        headerRows = [];
        baseline = '';
        return;
      }
      // Keep the current selection when it still exists, otherwise fall back to
      // the first provider so the detail pane is never empty on load.
      const existing = items.find((p) => p.id === selectedId);
      applyProvider(existing ?? items[0]);
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function select(provider) {
    if (provider.id === selectedId) return;
    if (dirty && !window.confirm('当前改动尚未保存，确定切换 Provider 吗？')) return;
    applyProvider(provider);
  }

  function openCreate() {
    createName = '';
    createOpen = true;
  }

  async function createProvider() {
    const name = createName.trim();
    if (!name) return;
    try {
      const created = await api('/api/admin/providers', {
        method: 'POST',
        body: JSON.stringify(formToPayload({ ...blankProvider(), name, base_url: 'https://' }))
      });
      createOpen = false;
      toast.success('Provider 已创建');
      await load();
      const fresh = providers.find((p) => p.id === created.id);
      if (fresh) applyProvider(fresh);
      tab = 'basic';
    } catch (e) {
      toast.error(e.message);
    }
  }

  async function save() {
    if (!selected) return;
    const errors = validateProvider(form);
    if (errors.length) {
      tab = errors[0].tab;
      toast.error(errors[0].message);
      return;
    }
    saving = true;
    try {
      const payload = formToPayload(form);
      payload.headers = rowsToMap(headerRows);
      const updated = await api(`/api/admin/providers/${selected.id}`, {
        method: 'PUT',
        body: JSON.stringify(payload)
      });
      toast.success('Provider 已保存');
      providers = providers.map((p) => (p.id === updated.id ? updated : p));
      applyProvider(updated);
    } catch (e) {
      toast.error(e.message);
    } finally {
      saving = false;
    }
  }

  function reset() {
    if (!selected) return;
    applyProvider(selected);
    toast.success('已还原为已保存的配置');
  }

  async function remove() {
    deleting = true;
    try {
      await api(`/api/admin/providers/${deleteTarget.id}`, { method: 'DELETE' });
      toast.success('Provider 已删除');
      deleteTarget = null;
      selectedId = null;
      await load();
    } catch (e) {
      toast.error(e.message);
    } finally {
      deleting = false;
    }
  }

  onMount(load);
</script>

<svelte:head>
  <title>Model Provider · aigw</title>
</svelte:head>

<PageHeader
  title="Model Provider"
  description="管理上游服务、请求策略、出站代理与模型目录。这里的每项配置都会直接影响网关的实时行为。"
>
  {#if selected}
    <Button variant="outline" size="sm" onclick={reset} disabled={saving || !dirty}>
      <RotateCcwIcon />
      还原
    </Button>
    <Button size="sm" onclick={save} disabled={saving || !dirty}>
      {#if saving}
        <Spinner class="size-3.5" />
        保存中…
      {:else}
        <SaveIcon />
        保存
      {/if}
    </Button>
  {/if}
</PageHeader>

{#if error}
  <Alert variant="destructive">
    <TriangleAlertIcon />
    <AlertDescription>{error}</AlertDescription>
  </Alert>
{/if}

<div class="grid gap-3 lg:grid-cols-[264px_minmax(0,1fr)] lg:items-start">
  <!-- secondary sidebar: provider list -->
  <aside class="flex min-w-0 flex-col overflow-hidden rounded-lg border bg-card lg:sticky lg:top-16">
    <div class="space-y-2 border-b p-2.5">
      <div class="flex items-center justify-between gap-2">
        <div class="flex items-center gap-1.5">
          <BoxesIcon class="size-3.5 text-muted-foreground" />
          <span class="text-xs font-semibold">Provider</span>
          <StatusBadge tone="neutral">{providers.length}</StatusBadge>
        </div>
        <Button variant="ghost" size="icon-xs" onclick={openCreate} aria-label="添加 Provider">
          <PlusIcon />
        </Button>
      </div>
      <InputGroup.Root>
        <InputGroup.Addon><SearchIcon /></InputGroup.Addon>
        <InputGroup.Input
          class="text-[11px]"
          placeholder="搜索名称 / 地址"
          bind:value={search}
        />
      </InputGroup.Root>
    </div>

    <div class="max-h-[min(60vh,32rem)] min-h-0 flex-1 overflow-y-auto p-1.5">
      {#if loading}
        <div class="space-y-1.5">
          {#each Array(3) as _, i (i)}
            <Skeleton class="h-12 w-full rounded-md" />
          {/each}
        </div>
      {:else if !edited.length}
        <div class="px-2 py-8 text-center text-[11px] text-muted-foreground">
          {providers.length ? '没有匹配的 Provider' : '还没有 Provider'}
        </div>
      {:else}
        <ul class="space-y-0.5">
          {#each edited as provider (provider.id)}
            <li>
              <button
                type="button"
                class="w-full rounded-md px-2 py-1.5 text-left transition-colors {provider.id ===
                selectedId
                  ? 'bg-primary/10 ring-1 ring-primary/25'
                  : 'hover:bg-muted/70'}"
                onclick={() => select(provider)}
                aria-current={provider.id === selectedId ? 'true' : undefined}
              >
                <div class="flex items-center gap-1.5">
                  <span
                    class="size-1.5 shrink-0 rounded-full {provider.enabled
                      ? 'bg-emerald-500'
                      : 'bg-muted-foreground/40'}"
                  ></span>
                  <span class="min-w-0 flex-1 truncate text-xs font-medium">{provider.name}</span>
                  {#if provider.slug}
                    <span class="shrink-0 font-mono text-[10px] text-muted-foreground">
                      {provider.slug}
                    </span>
                  {/if}
                </div>
                {#if provider.description}
                  <div class="mt-0.5 truncate pl-3 text-[10px] leading-tight text-muted-foreground/80">
                    {provider.description}
                  </div>
                {/if}
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>

    <div class="border-t p-2">
      <Button variant="outline" size="sm" class="w-full justify-center" onclick={openCreate}>
        <PlusIcon />
        添加 Provider
      </Button>
    </div>
  </aside>

  <!-- detail pane -->
  {#if !providers.length && !loading}
    <Card.Root>
      <Card.Content>
        <Empty.Root class="py-16">
          <Empty.Header>
            <Empty.Media variant="icon"><ServerIcon /></Empty.Media>
            <Empty.Title>还没有 Provider</Empty.Title>
            <Empty.Description>先添加一个上游服务，再配置模型与路由规则。</Empty.Description>
          </Empty.Header>
          <Empty.Content>
            <Button variant="outline" onclick={openCreate}>
              <PlusIcon />
              添加 Provider
            </Button>
          </Empty.Content>
        </Empty.Root>
      </Card.Content>
    </Card.Root>
  {:else if loading && !selected}
    <Skeleton class="h-96 w-full rounded-lg" />
  {:else if selected}
    <Card.Root class="min-w-0 gap-0">
      <Card.Header class="gap-2 border-b">
        <div class="flex flex-wrap items-start justify-between gap-2">
          <div class="min-w-0 space-y-1">
            <div class="flex flex-wrap items-center gap-1.5">
              <Card.Title class="text-sm">{form.name || selected.name}</Card.Title>
              <StatusBadge tone={form.enabled ? 'success' : 'neutral'} dot>
                {form.enabled ? '启用' : '停用'}
              </StatusBadge>
              {#if dirty}
                <StatusBadge tone="warning">未保存</StatusBadge>
              {/if}
            </div>
            <Card.Description class="truncate font-mono text-[10.5px]">
              {form.base_url}
            </Card.Description>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <div class="flex items-center gap-1.5">
              <span class="text-[10.5px] text-muted-foreground">启用</span>
              <Switch bind:checked={form.enabled} />
            </div>
            <Button
              variant="ghost"
              size="icon-sm"
              class="text-destructive hover:bg-destructive/10 hover:text-destructive"
              onclick={() => (deleteTarget = selected)}
              aria-label="删除 Provider"
            >
              <Trash2Icon />
            </Button>
          </div>
        </div>
      </Card.Header>

      <Tabs.Root bind:value={tab}>
        <div class="border-b px-3 pt-2">
          <Tabs.List variant="line">
            {#each TABS as item (item.value)}
              <Tabs.Trigger value={item.value} class="gap-1.5 px-2 pb-1.5">
                <item.icon class="size-3.5" />
                {item.label}
              </Tabs.Trigger>
            {/each}
          </Tabs.List>
        </div>

        <div class="p-3">
          <!-- ---------------------------------------------------- 基础 -->
          <Tabs.Content value="basic" class="space-y-4">
            <div class="grid gap-3 sm:grid-cols-2">
              <div class="grid gap-1.5">
                <Label for="p-name">名称</Label>
                <Input id="p-name" bind:value={form.name} placeholder="deepseek-official" />
              </div>
              <div class="grid gap-1.5">
                <Label for="p-slug">ID</Label>
                <Input
                  id="p-slug"
                  class="font-mono"
                  bind:value={form.slug}
                  placeholder="留空则由名称自动生成"
                />
              </div>
              <div class="grid gap-1.5 sm:col-span-2">
                <Label for="p-base">Base URL</Label>
                <Input id="p-base" bind:value={form.base_url} placeholder="https://api.example.com" />
              </div>
              <div class="grid gap-1.5 sm:col-span-2">
                <Label for="p-desc">描述</Label>
                <Input
                  id="p-desc"
                  bind:value={form.description}
                  placeholder="可选，便于团队识别该上游用途"
                />
              </div>
              <div class="grid gap-1.5">
                <Label for="p-auth">认证方式</Label>
                <Select.Root type="single" bind:value={form.auth_type}>
                  <Select.Trigger class="w-full" id="p-auth">
                    <Select.Value placeholder="选择认证方式" />
                  </Select.Trigger>
                  <Select.Content>
                    {#each AUTH_OPTIONS as option (option.value)}
                      <Select.Item value={option.value} label={option.label}>
                        {option.label}
                      </Select.Item>
                    {/each}
                  </Select.Content>
                </Select.Root>
              </div>
              <div class="grid gap-1.5">
                <Label for="p-key">
                  API Key {selected.has_api_key ? `（已配置 ••••${selected.api_key_last4}）` : ''}
                </Label>
                {#if form.auth_type === 'none'}
                  <Input id="p-key" placeholder="认证方式为「无认证」，无需填写" disabled />
                {:else}
                  <div class="relative">
                    <Input
                      id="p-key"
                      type={showApiKey ? 'text' : 'password'}
                      bind:value={form.api_key}
                      placeholder={selected.has_api_key ? '留空表示不修改' : 'sk-...'}
                      autocomplete="new-password"
                      class="pr-8"
                    />
                    <button
                      type="button"
                      class="absolute inset-y-0 right-0 grid w-8 place-items-center text-muted-foreground hover:text-foreground"
                      onclick={() => (showApiKey = !showApiKey)}
                      aria-label={showApiKey ? '隐藏 API Key' : '显示 API Key'}
                    >
                      {#if showApiKey}
                        <EyeOffIcon class="size-3.5" />
                      {:else}
                        <EyeIcon class="size-3.5" />
                      {/if}
                    </button>
                  </div>
                {/if}
              </div>
            </div>

            <Separator />

            <div class="space-y-2">
              <div>
                <div class="text-xs font-medium">API 格式</div>
                <p class="mt-0.5 text-[10.5px] text-muted-foreground">
                  勾选该上游支持的协议；Path 留空则使用内置默认值。
                </p>
              </div>
              <div class="space-y-1.5">
                {#each API_FORMATS as protocol (protocol.value)}
                  {@const entry = form.api_formats[protocol.value]}
                  <div
                    class="grid grid-cols-[150px_minmax(0,1fr)] items-center gap-2 rounded-md border px-2 py-1.5"
                    class:opacity-60={!entry.enabled}
                  >
                    <label class="flex items-center gap-2 text-xs">
                      <Checkbox bind:checked={entry.enabled} />
                      <span>{protocol.label}</span>
                    </label>
                    <Input
                      class="h-7 font-mono text-[11px]"
                      bind:value={entry.path}
                      placeholder={protocol.defaultPath}
                      disabled={!entry.enabled}
                      aria-label={`${protocol.label} Path`}
                    />
                  </div>
                {/each}
              </div>
            </div>
          </Tabs.Content>

          <!-- ---------------------------------------------------- 模型 -->
          <Tabs.Content value="models">
            {#if selected}
              <ProviderModels
                providerId={selected.id}
                providerName={selected.name}
                bind:models={providerModels}
              />
            {/if}
          </Tabs.Content>

          <!-- ---------------------------------------------------- 请求 -->
          <Tabs.Content value="request" class="space-y-4">
            <div class="grid gap-3 sm:grid-cols-2">
              <div class="grid gap-1.5">
                <Label for="p-timeout">请求超时 (ms)</Label>
                <Input id="p-timeout" type="number" min="1" bind:value={form.timeout_ms} />
                <p class="text-[10.5px] text-muted-foreground">
                  {formatDuration(form.timeout_ms)} · 单次上游调用上限
                </p>
              </div>
            </div>

            <div class="space-y-3 rounded-lg border bg-muted/30 p-2.5">
              <div class="flex items-center justify-between gap-2">
                <div>
                  <div class="text-xs font-medium">重试规则</div>
                  <p class="mt-0.5 text-[10.5px] text-muted-foreground">
                    Provider 级重试；设为 0 时仅由路由规则控制重试。
                  </p>
                </div>
                {#if form.retry_count > 0}
                  <StatusBadge tone="primary">共 {Number(form.retry_count) + 1} 次尝试</StatusBadge>
                {:else}
                  <StatusBadge tone="neutral">未启用</StatusBadge>
                {/if}
              </div>

              <div class="grid gap-3 sm:grid-cols-2">
                <div class="grid gap-1.5">
                  <Label for="p-retry">重试次数</Label>
                  <Input
                    id="p-retry"
                    type="number"
                    min="0"
                    max="20"
                    bind:value={form.retry_count}
                  />
                </div>
                <div class="grid gap-1.5">
                  <Label for="p-backoff">退避策略</Label>
                  <Select.Root type="single" bind:value={form.retry_backoff}>
                    <Select.Trigger class="w-full" id="p-backoff">
                      <Select.Value placeholder="exponential" />
                    </Select.Trigger>
                    <Select.Content>
                      <Select.Item value="exponential" label="指数退避 (exponential)">
                        指数退避 (exponential)
                      </Select.Item>
                      <Select.Item value="linear" label="线性退避 (linear)">
                        线性退避 (linear)
                      </Select.Item>
                    </Select.Content>
                  </Select.Root>
                </div>

                {#if form.retry_backoff === 'exponential'}
                  <div class="grid gap-1.5">
                    <Label for="p-retry-initial">基础退避 (ms)</Label>
                    <Input
                      id="p-retry-initial"
                      type="number"
                      min="0"
                      bind:value={form.retry_initial_ms}
                    />
                  </div>
                  <div class="grid gap-1.5">
                    <Label for="p-retry-max">最大退避 (ms)</Label>
                    <Input id="p-retry-max" type="number" min="0" bind:value={form.retry_max_ms} />
                  </div>
                {:else}
                  <div class="grid gap-1.5 sm:col-span-2">
                    <Label for="p-retry-linear">退避间隔 (ms)</Label>
                    <Input
                      id="p-retry-linear"
                      type="number"
                      min="0"
                      bind:value={form.retry_linear_interval_ms}
                    />
                    <p class="text-[10.5px] text-muted-foreground">
                      每次重试前固定等待 {formatDuration(form.retry_linear_interval_ms)}
                    </p>
                  </div>
                {/if}
              </div>
            </div>

            <HeaderRows
              label="额外 Header"
              description="会附加到每个发往该上游的请求。Content-Type、Authorization 等受保护 Header 会被忽略。"
              bind:rows={headerRows}
            />
          </Tabs.Content>

          <!-- ---------------------------------------------------- 代理 -->
          <Tabs.Content value="proxy" class="space-y-4">
            <div class="grid gap-3 sm:grid-cols-2">
              <div class="grid gap-1.5">
                <Label for="p-proxy-type">代理类型</Label>
                <Select.Root type="single" bind:value={form.proxy_type}>
                  <Select.Trigger class="w-full" id="p-proxy-type">
                    <Select.Value placeholder="不使用代理" />
                  </Select.Trigger>
                  <Select.Content>
                    {#each PROXY_OPTIONS as option (option.value)}
                      <Select.Item value={option.value} label={option.label}>
                        {option.label}
                      </Select.Item>
                    {/each}
                  </Select.Content>
                </Select.Root>
                <p class="text-[10.5px] text-muted-foreground">网关访问该上游时经由此代理出站</p>
              </div>
              <div class="grid gap-1.5">
                <Label for="p-proxy-url">URL</Label>
                <Input
                  id="p-proxy-url"
                  class="font-mono text-[11px]"
                  bind:value={form.proxy_url}
                  placeholder={form.proxy_type === 'socks5'
                    ? 'socks5://127.0.0.1:1080'
                    : 'http://127.0.0.1:7890'}
                  disabled={!proxyEnabled}
                />
              </div>
              <div class="grid gap-1.5">
                <Label for="p-proxy-user">Username</Label>
                <Input
                  id="p-proxy-user"
                  bind:value={form.proxy_username}
                  placeholder="可选"
                  disabled={!proxyEnabled}
                />
              </div>
              <div class="grid gap-1.5">
                <Label for="p-proxy-pass">
                  Password {selected.has_proxy_password
                    ? `（已配置 ••••${selected.proxy_password_last4}）`
                    : ''}
                </Label>
                <div class="relative">
                  <Input
                    id="p-proxy-pass"
                    type={showProxyPassword ? 'text' : 'password'}
                    bind:value={form.proxy_password}
                    placeholder={selected.has_proxy_password ? '留空表示不修改' : '可选'}
                    disabled={!proxyEnabled}
                    autocomplete="new-password"
                    class="pr-8"
                  />
                  <button
                    type="button"
                    class="absolute inset-y-0 right-0 grid w-8 place-items-center text-muted-foreground hover:text-foreground disabled:opacity-40"
                    onclick={() => (showProxyPassword = !showProxyPassword)}
                    disabled={!proxyEnabled}
                    aria-label={showProxyPassword ? '隐藏代理密码' : '显示代理密码'}
                  >
                    {#if showProxyPassword}
                      <EyeOffIcon class="size-3.5" />
                    {:else}
                      <EyeIcon class="size-3.5" />
                    {/if}
                  </button>
                </div>
              </div>
            </div>

            {#if proxyEnabled}
              <Alert>
                <ShieldCheckIcon />
                <AlertDescription>
                  代理密码与 API Key 一样使用 AES-256-GCM 加密存储，接口只回传末 4 位。保存后新请求立即生效。
                </AlertDescription>
              </Alert>
            {:else}
              <Alert>
                <GlobeIcon />
                <AlertDescription>
                  未启用代理，网关直连上游。若该上游需要经代理访问，请选择代理类型并填写 URL。
                </AlertDescription>
              </Alert>
            {/if}
          </Tabs.Content>


        </div>
      </Tabs.Root>

      <Card.Footer class="justify-end border-t">
        <div class="flex items-center gap-1.5">
          <Button variant="ghost" size="sm" onclick={reset} disabled={saving || !dirty}>还原</Button>
          <Button size="sm" onclick={save} disabled={saving || !dirty}>
            {#if saving}
              <Spinner class="size-3.5" />
              保存中…
            {:else}
              <SaveIcon />
              保存
            {/if}
          </Button>
        </div>
      </Card.Footer>
    </Card.Root>
  {:else}
    <Card.Root>
      <Card.Content>
        <Empty.Root class="py-16">
          <Empty.Header>
            <Empty.Media variant="icon"><LayersIcon /></Empty.Media>
            <Empty.Title>选择左侧 Provider</Empty.Title>
            <Empty.Description>或者新建一个上游服务。</Empty.Description>
          </Empty.Header>
        </Empty.Root>
      </Card.Content>
    </Card.Root>
  {/if}
</div>

<Dialog.Root bind:open={createOpen}>
  <Dialog.Content class="sm:max-w-md">
    <Dialog.Header>
      <Dialog.Title>添加 Provider</Dialog.Title>
      <Dialog.Description>
        先填写名称，创建后在详情页补齐地址、认证与策略配置。
      </Dialog.Description>
    </Dialog.Header>
    <div class="grid gap-1.5">
      <Label for="c-name">名称</Label>
      <Input id="c-name" bind:value={createName} placeholder="deepseek-official" />
    </div>
    <Dialog.Footer>
      <Button variant="outline" onclick={() => (createOpen = false)}>取消</Button>
      <Button onclick={createProvider} disabled={!createName.trim()}>
        <PlusIcon />
        创建
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

<ConfirmDialog
  bind:open={() => !!deleteTarget, (value) => !value && (deleteTarget = null)}
  title="删除 Provider"
  description={`确定删除 ${deleteTarget?.name || ''}？其下模型会一并删除，引用它的 routing target 会阻止删除。`}
  busy={deleting}
  onconfirm={remove}
/>
