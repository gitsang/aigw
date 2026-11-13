<script>
  import { onMount } from 'svelte';
  import { api, fmtLatency, fmtTime } from '$lib/api';
  import { toast } from 'svelte-sonner';

  import PlusIcon from '@lucide/svelte/icons/plus';
  import PencilIcon from '@lucide/svelte/icons/pencil';
  import Trash2Icon from '@lucide/svelte/icons/trash-2';
  import RouteIcon from '@lucide/svelte/icons/route';
  import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
  import CheckIcon from '@lucide/svelte/icons/check';

  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Label } from '$lib/components/ui/label/index.js';
  import { Switch } from '$lib/components/ui/switch/index.js';
  import * as Select from '$lib/components/ui/select/index.js';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import * as Empty from '$lib/components/ui/empty/index.js';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import { Alert, AlertDescription } from '$lib/components/ui/alert/index.js';
  import { Separator } from '$lib/components/ui/separator/index.js';
  import { Spinner } from '$lib/components/ui/spinner/index.js';

  import PageHeader from '$lib/components/app/page-header.svelte';
  import StatusBadge from '$lib/components/app/status-badge.svelte';
  import ConfirmDialog from '$lib/components/app/confirm-dialog.svelte';

  const APIS = [
    ['chat.completions', 'Chat Completions'],
    ['responses', 'Responses'],
    ['messages', 'Messages']
  ];
  const PROTOCOL_NAMES = Object.fromEntries(APIS);
  const apiLabel = (value) => PROTOCOL_NAMES[value] || value;

  const blankTarget = (role = 'lb') => ({
    role,
    provider_id: '',
    provider_model_id: '',
    upstream_model: '',
    weight: 50,
    priority: 0,
    enabled: true
  });
  const blankVirtualModel = () => ({
    name: '',
    virtual_model: '',
    enabled: true,
    retry_count: 3,
    retry_policy: 'exponential',
    fallback_policy: 'fallback-first',
    retry_delay_ms: 250,
    max_backoff_ms: 5000,
    targets: [blankTarget('lb'), blankTarget('fallback')]
  });

  let virtualModels = $state([]);
  let providers = $state([]);
  let providerModels = $state({});
  let loading = $state(true);
  let error = $state('');

  let modalOpen = $state(false);
  let editingId = $state(null);
  let form = $state(blankVirtualModel());
  let saving = $state(false);

  let deleteTarget = $state(null);
  let deleting = $state(false);

  async function load() {
    loading = true;
    error = '';
    try {
      [virtualModels, providers] = await Promise.all([
        api('/api/admin/virtual-models'),
        api('/api/admin/providers')
      ]);
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function openCreate() {
    editingId = null;
    form = blankVirtualModel();
    for (const provider of providers) await ensureModels(provider.id);
    if (providers.length) {
      form.targets[0].provider_id = providers[0].id;
      form.targets[0].provider_model_id = providerModels[providers[0].id]?.[0]?.id || '';
      if (providers[1]) {
        form.targets[1].provider_id = providers[1].id;
        form.targets[1].provider_model_id = providerModels[providers[1].id]?.[0]?.id || '';
      }
    }
    modalOpen = true;
  }

  async function openEdit(vm) {
    editingId = vm.id;
    form = {
      name: vm.name,
      virtual_model: vm.virtual_model,
      enabled: vm.enabled,
      retry_count: vm.retry_count,
      retry_policy: vm.retry_policy,
      fallback_policy: vm.fallback_policy,
      retry_delay_ms: vm.retry_delay_ms,
      max_backoff_ms: vm.max_backoff_ms,
      targets: vm.targets.map((target) => ({
        id: target.id,
        role: target.role,
        provider_id: target.provider_id,
        provider_model_id: target.provider_model_id || '',
        upstream_model: target.upstream_model,
        weight: target.weight,
        priority: target.priority,
        enabled: target.enabled
      }))
    };
    for (const target of form.targets) {
      if (target.provider_id) await ensureModels(target.provider_id);
    }
    modalOpen = true;
  }

  async function addTarget(role) {
    const target = blankTarget(role);
    if (providers.length) {
      target.provider_id = providers[0].id;
      const list = await ensureModels(target.provider_id);
      target.provider_model_id = list[0]?.id || '';
    }
    form.targets = [...form.targets, target];
  }

  function removeTarget(index) {
    form.targets = form.targets.filter((_, i) => i !== index);
  }

  async function ensureModels(providerID) {
    if (!providerID) return [];
    if (!providerModels[providerID]) {
      try {
        providerModels[providerID] = await api(`/api/admin/providers/${providerID}/models`);
      } catch (e) {
        providerModels[providerID] = [];
        toast.error(`加载模型失败：${e.message}`);
      }
    }
    return providerModels[providerID];
  }

  function modelsFor(providerID) {
    return providerModels[providerID] || [];
  }
  function modelOf(target) {
    return modelsFor(target.provider_id).find((model) => model.id === target.provider_model_id);
  }
  function providerOf(target) {
    return providers.find((provider) => provider.id === target.provider_id);
  }
  function updateTarget(index, patch) {
    form.targets = form.targets.map((target, i) =>
      i === index ? { ...target, ...patch } : target
    );
  }
  /** Protocols the selected upstream model (or its provider) can speak, in the
   * fixed order chat.completions → responses → messages. This is read-only: a
   * target always forwards the inbound protocol when the upstream supports it
   * and otherwise falls back to the first entry of this list. */
  function supportedProtocols(target) {
    const model = modelOf(target);
    const provider = providerOf(target);
    const declared = model?.api_types?.length ? model.api_types : provider?.api_types || [];
    return APIS.map(([value]) => value).filter((value) => declared.includes(value));
  }
  function targetPathHint(target) {
    const model = modelOf(target);
    const provider = providerOf(target);
    const parts = [];
    for (const [key, value] of Object.entries(model?.path_overrides || {})) {
      parts.push(`${apiLabel(key)}: ${value}`);
    }
    for (const [key, value] of Object.entries(provider?.path_overrides || {})) {
      if (!model?.path_overrides?.[key]) parts.push(`${apiLabel(key)}: ${value}`);
    }
    return parts.length ? parts.join('  ') : '内置默认 Path';
  }
  async function onProviderChange(index) {
    updateTarget(index, { provider_model_id: '', upstream_model: '' });
    const target = form.targets[index];
    const list = await ensureModels(target.provider_id);
    const next = { ...form.targets[index] };
    if (list.length) next.provider_model_id = list[0].id;
    updateTarget(index, next);
  }

  async function save() {
    if (!form.virtual_model.trim()) return toast.error('请填写虚拟模型 ID');
    if (!form.targets.length) return toast.error('至少配置一个 target');
    if (form.targets.some((target) => !target.provider_id || !target.provider_model_id)) {
      return toast.error('每个 target 都需要 provider 和模型');
    }
    saving = true;
    try {
      const payload = { ...form, targets: form.targets.map((target) => ({ ...target })) };
      if (editingId) {
        await api(`/api/admin/virtual-models/${editingId}`, { method: 'PUT', body: JSON.stringify(payload) });
      } else {
        await api('/api/admin/virtual-models', { method: 'POST', body: JSON.stringify(payload) });
      }
      toast.success(editingId ? '虚拟模型已更新' : '虚拟模型已创建');
      modalOpen = false;
      await load();
    } catch (e) {
      toast.error(e.message);
    } finally {
      saving = false;
    }
  }

  async function remove() {
    deleting = true;
    try {
      await api(`/api/admin/virtual-models/${deleteTarget.id}`, { method: 'DELETE' });
      toast.success('虚拟模型已删除');
      deleteTarget = null;
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
  <title>Virtual Model · aigw</title>
</svelte:head>

<PageHeader
  title="Virtual Model"
  description="虚拟模型 ID（客户端请求的 model）→ 加权 LB target → fallback。健康检查每分钟执行，不健康的 LB 自动移出，全部下线时使用 fallback。"
>
  <Button onclick={openCreate} disabled={!providers.length}>
    <PlusIcon />
    添加虚拟模型
  </Button>
</PageHeader>

{#if !providers.length && !loading}
  <Alert>
    <TriangleAlertIcon />
    <AlertDescription>
      请先在 <a href="/providers" class="font-medium underline underline-offset-2">Model Provider</a>
      添加至少一个上游服务。
    </AlertDescription>
  </Alert>
{/if}
{#if error}
  <Alert variant="destructive">
    <TriangleAlertIcon />
    <AlertDescription>{error}</AlertDescription>
  </Alert>
{/if}

{#if loading}
  <div class="space-y-2.5">
    {#each Array(2) as _, i (i)}
      <Skeleton class="h-[184px] w-full rounded-lg" />
    {/each}
  </div>
{:else if !virtualModels.length}
  <Card.Root>
    <Card.Content>
      <Empty.Root class="py-10">
        <Empty.Header>
          <Empty.Media variant="icon"><RouteIcon /></Empty.Media>
          <Empty.Title>还没有虚拟模型</Empty.Title>
          <Empty.Description>
            创建一个虚拟模型，把虚拟模型 ID 映射到一个或多个上游 provider。
          </Empty.Description>
        </Empty.Header>
        <Empty.Content>
          <Button variant="outline" onclick={openCreate} disabled={!providers.length}>
            <PlusIcon />
            创建虚拟模型
          </Button>
        </Empty.Content>
      </Empty.Root>
    </Card.Content>
  </Card.Root>
{:else}
  <section class="space-y-2.5">
    {#each virtualModels as vm (vm.id)}
      <Card.Root class="gap-2">
        <Card.Header>
          <div class="flex flex-wrap items-start justify-between gap-2">
            <div class="min-w-0 space-y-1.5">
              <div class="flex flex-wrap items-center gap-1.5">
                <span class="font-mono text-sm font-semibold tracking-tight">
                  {vm.virtual_model}
                </span>
                <StatusBadge tone={vm.enabled ? 'success' : 'neutral'} dot>
                  {vm.enabled ? '启用' : '停用'}
                </StatusBadge>
                <StatusBadge tone="violet">{vm.fallback_policy}</StatusBadge>
                <StatusBadge tone="warning">{vm.retry_policy} · {vm.retry_count}x</StatusBadge>
              </div>
              <div class="text-[11px] text-muted-foreground">
                {vm.name} · {vm.retry_delay_ms}ms base / {vm.max_backoff_ms}ms max backoff
              </div>
            </div>
            <div class="flex shrink-0 items-center gap-1">
              <Button variant="outline" size="sm" onclick={() => openEdit(vm)}>
                <PencilIcon />
                编辑
              </Button>
              <Button
                variant="ghost"
                size="sm"
                class="text-destructive hover:bg-destructive/10 hover:text-destructive"
                onclick={() => (deleteTarget = vm)}
              >
                <Trash2Icon />
                删除
              </Button>
            </div>
          </div>
        </Card.Header>

        <Card.Content>
          <div class="overflow-x-auto">
            <div class="min-w-[720px] overflow-hidden rounded-lg border">
              <div
                class="grid grid-cols-[70px_minmax(180px,1.4fr)_minmax(190px,1.1fr)_60px_minmax(140px,0.9fr)] gap-2 bg-muted/50 px-2.5 py-1.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground"
              >
                <span>角色</span>
                <span>Provider / Upstream</span>
                <span>上游协议</span>
                <span class="text-right">权重</span>
                <span>健康</span>
              </div>
              {#each vm.targets as target, index (target.id ?? index)}
                <div
                  class="grid grid-cols-[70px_minmax(180px,1.4fr)_minmax(190px,1.1fr)_60px_minmax(140px,0.9fr)] items-center gap-2 border-t px-2.5 py-1.5 text-[11px]"
                  class:opacity-60={!target.enabled}
                >
                  <div>
                    <StatusBadge tone={target.role === 'fallback' ? 'warning' : 'primary'}>
                      {target.role === 'fallback' ? 'Fallback' : 'LB'}
                    </StatusBadge>
                  </div>
                  <div class="min-w-0">
                    <div class="truncate font-medium">{target.provider_name}</div>
                    <div class="truncate font-mono text-[10px] text-muted-foreground">
                      {target.upstream_model}
                    </div>
                  </div>
                  <div class="min-w-0">
                    <div class="truncate font-mono text-[10.5px]">
                      {target.supported_api_types?.length
                        ? target.supported_api_types.map(apiLabel).join(' · ')
                        : '继承模型 / Provider'}
                    </div>
                    <div class="truncate text-[10px] text-muted-foreground">
                      {target.path_overrides && Object.keys(target.path_overrides).length
                        ? Object.entries(target.path_overrides)
                            .map(([key, value]) => `${apiLabel(key)} → ${value}`)
                            .join('  ')
                        : 'Provider / 内置 Path'}
                    </div>
                  </div>
                  <div class="text-right font-mono tabular-nums">
                    {target.role === 'fallback' ? '—' : target.weight}
                  </div>
                  <div class="min-w-0 space-y-0.5">
                    {#if target.role === 'fallback'}
                      <StatusBadge tone="neutral">仅兜底</StatusBadge>
                    {:else if target.health?.healthy}
                      <StatusBadge tone="success" dot>健康</StatusBadge>
                    {:else}
                      <StatusBadge tone="danger" dot>不健康</StatusBadge>
                    {/if}
                    {#if target.health?.checked_at}
                      <div class="truncate text-[10px] text-muted-foreground">
                        {fmtLatency(target.health.latency_ms)} · {fmtTime(target.health.checked_at)}
                      </div>
                    {/if}
                    {#if target.health && !target.health.healthy && target.health.error}
                      <div
                        class="truncate text-[10px] text-destructive"
                        title={target.health.error}
                      >
                        {target.health.error}
                      </div>
                    {/if}
                  </div>
                </div>
              {/each}
            </div>
          </div>
        </Card.Content>
      </Card.Root>
    {/each}
  </section>
{/if}

<Dialog.Root bind:open={modalOpen}>
  <Dialog.Content class="max-h-[88vh] overflow-y-auto sm:max-w-4xl">
    <Dialog.Header>
      <Dialog.Title>{editingId ? '编辑虚拟模型' : '添加虚拟模型'}</Dialog.Title>
      <Dialog.Description>
        虚拟模型映射到多个上游 target，按权重负载均衡，失败时按策略重试与兜底。
      </Dialog.Description>
    </Dialog.Header>

    <div class="grid gap-3 sm:grid-cols-2">
      <div class="grid gap-1.5">
        <Label for="r-name">
          名称
          <span class="text-[10px] font-normal text-muted-foreground">（写入日志 virtual_model_name）</span>
        </Label>
        <Input id="r-name" bind:value={form.name} placeholder="deepseek-flash-lb" />
      </div>
      <div class="grid gap-1.5">
        <Label for="r-model">
          虚拟模型 ID
          <span class="text-[10px] font-normal text-muted-foreground">（客户端请求的 model）</span>
        </Label>
        <Input
          id="r-model"
          class="font-mono"
          bind:value={form.virtual_model}
          placeholder="deepseek-flash"
        />
      </div>
    </div>

    <div class="grid gap-3 sm:grid-cols-2">
      <div class="grid gap-1.5 sm:col-span-2">
        <Label for="r-fallback">Fallback 策略</Label>
        <Select.Root type="single" bind:value={form.fallback_policy}>
          <Select.Trigger class="w-full" id="r-fallback">
            <Select.Value placeholder="fallback-first" />
          </Select.Trigger>
          <Select.Content>
            <Select.Item value="fallback-first" label="fallback-first">
              fallback-first · 每轮先尝试全部 LB，再 fallback，整体循环重试
            </Select.Item>
            <Select.Item value="retry-first" label="retry-first">
              retry-first · 单个 target 重试 N 次后切换，最后 fallback
            </Select.Item>
          </Select.Content>
        </Select.Root>
      </div>
      <div class="grid gap-1.5">
        <Label for="r-retry">重试次数</Label>
        <Input id="r-retry" type="number" min="1" max="20" bind:value={form.retry_count} />
      </div>
      <div class="grid gap-1.5">
        <Label for="r-policy">退避策略</Label>
        <Select.Root type="single" bind:value={form.retry_policy}>
          <Select.Trigger class="w-full" id="r-policy">
            <Select.Value placeholder="exponential" />
          </Select.Trigger>
          <Select.Content>
            <Select.Item value="exponential" label="指数退避">指数退避</Select.Item>
            <Select.Item value="fixed" label="固定退避">固定退避</Select.Item>
          </Select.Content>
        </Select.Root>
      </div>
      <div class="grid gap-1.5">
        <Label for="r-delay">基础退避 (ms)</Label>
        <Input id="r-delay" type="number" min="0" bind:value={form.retry_delay_ms} />
      </div>
      <div class="grid gap-1.5">
        <Label for="r-backoff">最大退避 (ms)</Label>
        <Input id="r-backoff" type="number" min="0" bind:value={form.max_backoff_ms} />
      </div>
    </div>

    <div class="flex items-center justify-between gap-2 rounded-lg border px-3 py-2">
      <div class="space-y-0.5">
        <div class="text-xs font-medium">启用该虚拟模型</div>
        <div class="text-[10.5px] text-muted-foreground">
          停用后该虚拟模型立即不可用
        </div>
      </div>
      <Switch bind:checked={form.enabled} />
    </div>

    <Separator />

    <div class="space-y-2">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 class="text-xs font-semibold">Targets</h3>
          <p class="mt-0.5 text-[10.5px] leading-relaxed text-muted-foreground">
            每个 target 不需要选择上游协议：入口请求按<b class="text-foreground">原协议</b>转发，
            上游不支持时按 <span class="font-mono">chat.completions → responses → messages</span>
            的固定顺序取第一个可用协议并自动转换。协议由所选模型 / Provider 的配置决定，Path 同源。
          </p>
        </div>
        <div class="flex items-center gap-1.5">
          <Button variant="outline" size="sm" onclick={() => addTarget('lb')}>
            <PlusIcon />
            LB
          </Button>
          <Button variant="outline" size="sm" onclick={() => addTarget('fallback')}>
            <PlusIcon />
            Fallback
          </Button>
        </div>
      </div>

      {#each form.targets as target, index (index)}
        <div class="space-y-2 rounded-lg border bg-muted/30 p-2.5">
          <div class="flex items-center justify-between gap-2">
            <StatusBadge tone={target.role === 'fallback' ? 'warning' : 'primary'}>
              {target.role === 'fallback' ? 'Fallback' : 'LB'}
            </StatusBadge>
            <Button
              variant="ghost"
              size="xs"
              class="text-destructive hover:bg-destructive/10 hover:text-destructive"
              onclick={() => removeTarget(index)}
              aria-label="删除 target"
            >
              <Trash2Icon />
            </Button>
          </div>

          <div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-[110px_1fr_1.3fr_90px]">
            <div class="grid gap-1">
              <Label for={`t-role-${index}`} class="text-[10px] text-muted-foreground">角色</Label>
              <Select.Root type="single" bind:value={target.role}>
                <Select.Trigger class="w-full" id={`t-role-${index}`}>
                  <Select.Value placeholder="LB" />
                </Select.Trigger>
                <Select.Content>
                  <Select.Item value="lb" label="LB">LB</Select.Item>
                  <Select.Item value="fallback" label="Fallback">Fallback</Select.Item>
                </Select.Content>
              </Select.Root>
            </div>
            <div class="grid gap-1">
              <Label for={`t-provider-${index}`} class="text-[10px] text-muted-foreground">
                Provider
              </Label>
              <Select.Root
                type="single"
                bind:value={target.provider_id}
                onValueChange={() => onProviderChange(index)}
              >
                <Select.Trigger class="w-full" id={`t-provider-${index}`}>
                  <Select.Value placeholder="选择…" />
                </Select.Trigger>
                <Select.Content>
                  {#each providers as provider (provider.id)}
                    <Select.Item value={provider.id} label={provider.name}>
                      {provider.name}
                    </Select.Item>
                  {/each}
                </Select.Content>
              </Select.Root>
            </div>
            <div class="grid gap-1">
              <Label for={`t-model-${index}`} class="text-[10px] text-muted-foreground">模型</Label>
              <Select.Root
                type="single"
                bind:value={target.provider_model_id}
              >
                <Select.Trigger class="w-full" id={`t-model-${index}`}>
                  <Select.Value placeholder="选择模型…" />
                </Select.Trigger>
                <Select.Content>
                  {#each modelsFor(target.provider_id) as model (model.id)}
                    <Select.Item value={model.id} label={model.model_id}>{model.model_id}</Select.Item>
                  {/each}
                </Select.Content>
              </Select.Root>
            </div>
            <div class="grid gap-1">
              <Label for={`t-weight-${index}`} class="text-[10px] text-muted-foreground">权重</Label>
              <Input id={`t-weight-${index}`} type="number" min="1" bind:value={target.weight} />
            </div>
          </div>

          <div class="space-y-1">
            <div class="flex flex-wrap items-center gap-1.5">
              <span class="text-[10px] uppercase tracking-wide text-muted-foreground">
                上游协议
              </span>
              {#each supportedProtocols(target) as name, i (name)}
                <StatusBadge tone={i === 0 ? 'info' : 'neutral'}>{apiLabel(name)}</StatusBadge>
              {/each}
              {#if !supportedProtocols(target).length}
                <span class="text-[10px] text-muted-foreground">继承模型 / Provider</span>
              {/if}
              <span class="text-[10px] text-muted-foreground">（自动，无需配置）</span>
            </div>
            <div class="truncate font-mono text-[10px] text-muted-foreground">
              Path: {targetPathHint(target)}
            </div>
            {#if target.provider_id && !modelsFor(target.provider_id).length}
              <div class="text-[10px] text-amber-600 dark:text-amber-400">
                该 Provider 还没有模型，请先在 Provider 详情添加。
              </div>
            {/if}
          </div>
        </div>
      {/each}
    </div>

    <Dialog.Footer>
      <Button variant="outline" onclick={() => (modalOpen = false)} disabled={saving}>取消</Button>
      <Button onclick={save} disabled={saving}>
        {#if saving}
          <Spinner class="size-3.5" />
          保存中…
        {:else}
          <CheckIcon />
          保存
        {/if}
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

<ConfirmDialog
  bind:open={() => !!deleteTarget, (value) => !value && (deleteTarget = null)}
  title="删除虚拟模型"
  description={`确定删除虚拟模型 ${deleteTarget?.virtual_model || ''}？使用该模型的客户端将立即失败。`}
  busy={deleting}
  onconfirm={remove}
/>
