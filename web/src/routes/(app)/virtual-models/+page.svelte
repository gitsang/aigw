<script>
  import { onMount } from 'svelte';
  import { api, fmtLatency, fmtTime } from '$lib/api';
  import { t } from '$lib/i18n/index.svelte.js';
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

  import {
    API_PROTOCOLS,
    apiSchemaLabel,
    apiSchemaProtocols,
    effectiveAPISchema
  } from '$lib/api-schema.js';

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
        toast.error(t('virtualModels.toast.modelsLoadFailed', { error: e.message }));
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
  /** Labels for the select triggers. bits-ui only resolves an item's label once
   * its content has been mounted, so a closed select would otherwise show the
   * raw provider/model id. */
  function providerLabelOf(target) {
    return providerOf(target)?.name || '';
  }
  function modelLabelOf(target) {
    return modelOf(target)?.model_id || '';
  }
  function updateTarget(index, patch) {
    form.targets = form.targets.map((target, i) =>
      i === index ? { ...target, ...patch } : target
    );
  }
  /** Effective API schema previewed for a target: mirrors the gateway
   * resolution between the selected model and its provider. */
  function previewSchema(target) {
    return effectiveAPISchema(modelOf(target)?.api_schema, providerOf(target)?.api_schema);
  }
  /** Protocols the selected upstream model (or its provider) can speak, in the
   * fixed order chat.completions → responses → messages. This is read-only: a
   * target always forwards the inbound protocol when the upstream supports it
   * and otherwise falls back to the first entry of this list. */
  function supportedProtocols(target) {
    return apiSchemaProtocols(previewSchema(target));
  }
  /** Endpoint summary for an effective schema: only non-default endpoints are
   * listed, everything else resolves to the built-in default. */
  function endpointSummary(schema) {
    const parts = [];
    for (const protocol of API_PROTOCOLS) {
      const entry = schema?.[protocol.value];
      if (!entry?.enabled) continue;
      if (entry.endpoint === protocol.defaultEndpoint) continue;
      parts.push(`${apiSchemaLabel(protocol.value)}: ${entry.endpoint}`);
    }
    return parts.length ? parts.join('  ') : t('virtualModels.target.builtinEndpoint');
  }
  function targetEndpointHint(target) {
    return endpointSummary(previewSchema(target));
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
    if (!form.virtual_model.trim()) return toast.error(t('virtualModels.toast.virtualModelRequired'));
    if (!form.targets.length) return toast.error(t('virtualModels.toast.targetRequired'));
    if (form.targets.some((target) => !target.provider_id || !target.provider_model_id)) {
      return toast.error(t('virtualModels.toast.targetIncomplete'));
    }
    saving = true;
    try {
      const payload = { ...form, targets: form.targets.map((target) => ({ ...target })) };
      if (editingId) {
        await api(`/api/admin/virtual-models/${editingId}`, { method: 'PUT', body: JSON.stringify(payload) });
      } else {
        await api('/api/admin/virtual-models', { method: 'POST', body: JSON.stringify(payload) });
      }
      toast.success(
        editingId ? t('virtualModels.toast.updated') : t('virtualModels.toast.created')
      );
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
      toast.success(t('virtualModels.toast.deleted'));
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
  <title>{t('virtualModels.title')}</title>
</svelte:head>

<PageHeader
  title={t('virtualModels.heading')}
  description={t('virtualModels.description')}
>
  <Button onclick={openCreate} disabled={!providers.length}>
    <PlusIcon />
    {t('virtualModels.action.create')}
  </Button>
</PageHeader>

{#if !providers.length && !loading}
  <Alert>
    <TriangleAlertIcon />
    <AlertDescription>
      {t('virtualModels.setup.before')}
      <a href="/providers" class="font-medium underline underline-offset-2"
        >{t('virtualModels.setup.link')}</a
      >
      {t('virtualModels.setup.after')}
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
          <Empty.Title>{t('virtualModels.empty.title')}</Empty.Title>
          <Empty.Description>
            {t('virtualModels.empty.description')}
          </Empty.Description>
        </Empty.Header>
        <Empty.Content>
          <Button variant="outline" onclick={openCreate} disabled={!providers.length}>
            <PlusIcon />
            {t('virtualModels.action.createEmpty')}
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
                  {vm.enabled
                    ? t('virtualModels.state.enabled')
                    : t('virtualModels.state.disabled')}
                </StatusBadge>
                <StatusBadge tone="violet">{vm.fallback_policy}</StatusBadge>
                <StatusBadge tone="warning">{vm.retry_policy} · {vm.retry_count}x</StatusBadge>
              </div>
              <div class="text-[11px] text-muted-foreground">
                {t('virtualModels.card.meta', {
                  name: vm.name,
                  delay: vm.retry_delay_ms,
                  backoff: vm.max_backoff_ms
                })}
              </div>
            </div>
            <div class="flex shrink-0 items-center gap-1">
              <Button variant="outline" size="sm" onclick={() => openEdit(vm)}>
                <PencilIcon />
                {t('common.action.edit')}
              </Button>
              <Button
                variant="ghost"
                size="sm"
                class="text-destructive hover:bg-destructive/10 hover:text-destructive"
                onclick={() => (deleteTarget = vm)}
              >
                <Trash2Icon />
                {t('common.action.delete')}
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
                <span>{t('virtualModels.column.role')}</span>
                <span>{t('virtualModels.column.providerUpstream')}</span>
                <span>{t('virtualModels.column.upstreamProtocol')}</span>
                <span class="text-right">{t('virtualModels.column.weight')}</span>
                <span>{t('virtualModels.column.health')}</span>
              </div>
              {#each vm.targets as target, index (target.id ?? index)}
                {@const protocols = apiSchemaProtocols(target.api_schema)}
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
                      {protocols.length
                        ? protocols.map(apiSchemaLabel).join(' · ')
                        : t('virtualModels.target.inheritProtocol')}
                    </div>
                    <div class="truncate text-[10px] text-muted-foreground">
                      {endpointSummary(target.api_schema)}
                    </div>
                  </div>
                  <div class="text-right font-mono tabular-nums">
                    {target.role === 'fallback' ? '—' : target.weight}
                  </div>
                  <div class="min-w-0 space-y-0.5">
                    {#if target.role === 'fallback'}
                      <StatusBadge tone="neutral">{t('virtualModels.target.fallbackOnly')}</StatusBadge>
                    {:else if target.health?.healthy}
                      <StatusBadge tone="success" dot>{t('virtualModels.health.healthy')}</StatusBadge>
                    {:else}
                      <StatusBadge tone="danger" dot>{t('virtualModels.health.unhealthy')}</StatusBadge>
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
      <Dialog.Title>
        {editingId
          ? t('virtualModels.dialog.editTitle')
          : t('virtualModels.dialog.createTitle')}
      </Dialog.Title>
      <Dialog.Description>
        {t('virtualModels.dialog.description')}
      </Dialog.Description>
    </Dialog.Header>

    <div class="flex items-center justify-between gap-2 rounded-lg border px-3 py-2">
      <div class="space-y-0.5">
        <div class="text-xs font-medium">{t('virtualModels.dialog.enabledLabel')}</div>
        <div class="text-[10.5px] text-muted-foreground">
          {t('virtualModels.dialog.enabledHint')}
        </div>
      </div>
      <Switch bind:checked={form.enabled} />
    </div>

    <div class="grid gap-3 sm:grid-cols-2">
      <div class="grid gap-1.5">
        <Label for="r-name">
          {t('virtualModels.form.name')}
          <span class="text-[10px] font-normal text-muted-foreground"
            >{t('virtualModels.form.nameHint')}</span
          >
        </Label>
        <Input id="r-name" bind:value={form.name} placeholder="deepseek-flash-lb" />
      </div>
      <div class="grid gap-1.5">
        <Label for="r-model">
          {t('virtualModels.form.virtualModelId')}
          <span class="text-[10px] font-normal text-muted-foreground"
            >{t('virtualModels.form.virtualModelIdHint')}</span
          >
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
        <Label for="r-fallback">{t('virtualModels.form.fallbackPolicy')}</Label>
        <Select.Root type="single" bind:value={form.fallback_policy}>
          <Select.Trigger class="w-full" id="r-fallback">
            <Select.Value placeholder="fallback-first" />
          </Select.Trigger>
          <Select.Content>
            <Select.Item value="fallback-first" label="fallback-first">
              {t('virtualModels.policy.fallbackFirst')}
            </Select.Item>
            <Select.Item value="retry-first" label="retry-first">
              {t('virtualModels.policy.retryFirst')}
            </Select.Item>
          </Select.Content>
        </Select.Root>
      </div>
      <div class="grid gap-1.5">
        <Label for="r-retry">{t('virtualModels.form.retryCount')}</Label>
        <Input id="r-retry" type="number" min="1" max="20" bind:value={form.retry_count} />
      </div>
      <div class="grid gap-1.5">
        <Label for="r-policy">{t('virtualModels.form.retryPolicy')}</Label>
        <Select.Root type="single" bind:value={form.retry_policy}>
          <Select.Trigger class="w-full" id="r-policy">
            <Select.Value placeholder="exponential" />
          </Select.Trigger>
          <Select.Content>
            <Select.Item value="exponential" label={t('virtualModels.policy.exponential')}
              >{t('virtualModels.policy.exponential')}</Select.Item
            >
            <Select.Item value="fixed" label={t('virtualModels.policy.fixed')}
              >{t('virtualModels.policy.fixed')}</Select.Item
            >
          </Select.Content>
        </Select.Root>
      </div>
      <div class="grid gap-1.5">
        <Label for="r-delay">{t('virtualModels.form.baseBackoff')}</Label>
        <Input id="r-delay" type="number" min="0" bind:value={form.retry_delay_ms} />
      </div>
      <div class="grid gap-1.5">
        <Label for="r-backoff">{t('virtualModels.form.maxBackoff')}</Label>
        <Input id="r-backoff" type="number" min="0" bind:value={form.max_backoff_ms} />
      </div>
    </div>

    <Separator />

    <div class="space-y-2">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 class="text-xs font-semibold">{t('virtualModels.targets.title')}</h3>
          <p class="mt-0.5 text-[10.5px] leading-relaxed text-muted-foreground">
            {t('virtualModels.targets.descriptionPrefix')}<b class="text-foreground"
              >{t('virtualModels.targets.descriptionBold')}</b
            >{t('virtualModels.targets.descriptionMiddle')}
            <span class="font-mono">chat.completions → responses → messages</span>
            {t('virtualModels.targets.descriptionSuffix')}
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
              aria-label={t('virtualModels.target.deleteAria')}
            >
              <Trash2Icon />
            </Button>
          </div>

          <div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-[110px_1fr_1.3fr_90px]">
            <div class="grid gap-1">
              <Label for={`t-role-${index}`} class="text-[10px] text-muted-foreground"
                >{t('virtualModels.target.role')}</Label
              >
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
                {t('virtualModels.target.provider')}
              </Label>
              <Select.Root
                type="single"
                bind:value={target.provider_id}
                onValueChange={() => onProviderChange(index)}
              >
                <Select.Trigger class="w-full" id={`t-provider-${index}`}>
                  <Select.Value placeholder={t('virtualModels.target.selectProvider')}
                    >{providerLabelOf(target)}</Select.Value
                  >
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
              <Label for={`t-model-${index}`} class="text-[10px] text-muted-foreground"
                >{t('virtualModels.target.model')}</Label
              >
              <Select.Root
                type="single"
                bind:value={target.provider_model_id}
              >
                <Select.Trigger class="w-full" id={`t-model-${index}`}>
                  <Select.Value placeholder={t('virtualModels.target.selectModel')}
                    >{modelLabelOf(target)}</Select.Value
                  >
                </Select.Trigger>
                <Select.Content>
                  {#each modelsFor(target.provider_id) as model (model.id)}
                    <Select.Item value={model.id} label={model.model_id}>{model.model_id}</Select.Item>
                  {/each}
                </Select.Content>
              </Select.Root>
            </div>
            <div class="grid gap-1">
              <Label for={`t-weight-${index}`} class="text-[10px] text-muted-foreground"
                >{t('virtualModels.target.weight')}</Label
              >
              <Input id={`t-weight-${index}`} type="number" min="1" bind:value={target.weight} />
            </div>
          </div>

          <div class="space-y-1">
            <div class="flex flex-wrap items-center gap-1.5">
              <span class="text-[10px] uppercase tracking-wide text-muted-foreground">
                {t('virtualModels.target.upstreamProtocol')}
              </span>
              {#each supportedProtocols(target) as name, i (name)}
                <StatusBadge tone={i === 0 ? 'info' : 'neutral'}>{apiSchemaLabel(name)}</StatusBadge>
              {/each}
              {#if !supportedProtocols(target).length}
                <span class="text-[10px] text-muted-foreground"
                  >{t('virtualModels.target.inheritProtocol')}</span
                >
              {/if}
              <span class="text-[10px] text-muted-foreground"
                >{t('virtualModels.target.autoProtocol')}</span
              >
            </div>
            <div class="truncate font-mono text-[10px] text-muted-foreground">
              {t('virtualModels.target.endpoint')}: {targetEndpointHint(target)}
            </div>
            {#if target.provider_id && !modelsFor(target.provider_id).length}
              <div class="text-[10px] text-amber-600 dark:text-amber-400">
                {t('virtualModels.target.noModels')}
              </div>
            {/if}
          </div>
        </div>
      {/each}
    </div>

    <Dialog.Footer>
      <Button variant="outline" onclick={() => (modalOpen = false)} disabled={saving}
        >{t('common.action.cancel')}</Button
      >
      <Button onclick={save} disabled={saving}>
        {#if saving}
          <Spinner class="size-3.5" />
          {t('common.state.saving')}
        {:else}
          <CheckIcon />
          {t('common.action.save')}
        {/if}
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

<ConfirmDialog
  bind:open={() => !!deleteTarget, (value) => !value && (deleteTarget = null)}
  title={t('virtualModels.delete.title')}
  description={t('virtualModels.delete.description', {
    name: deleteTarget?.virtual_model || ''
  })}
  busy={deleting}
  onconfirm={remove}
/>
