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

  import { t } from '$lib/i18n/index.svelte.js';
  import {
    blankProvider,
    providerToForm,
    formToPayload,
    formSignature,
    validateProvider,
    mapToRows,
    rowsToMap,
    formatDuration
  } from '$lib/provider-form.js';
  import { API_PROTOCOLS } from '$lib/api-schema.js';

  // Labels are translation keys resolved at render time so they follow the locale.
  const AUTH_OPTIONS = [
    { value: 'bearer', labelKey: 'providers.basic.auth.bearer' },
    { value: 'x-api-key', labelKey: 'providers.basic.auth.xApiKey' },
    { value: 'x-goog-api-key', labelKey: 'providers.basic.auth.xGoogApiKey' },
    { value: 'none', labelKey: 'providers.basic.auth.none' }
  ];
  const PROXY_OPTIONS = [
    { value: 'none', labelKey: 'providers.proxy.options.none' },
    { value: 'http', labelKey: 'providers.proxy.options.http' },
    { value: 'socks5', labelKey: 'providers.proxy.options.socks5' }
  ];
  const BACKOFF_OPTIONS = [
    { value: 'exponential', labelKey: 'providers.request.backoffExponential' },
    { value: 'linear', labelKey: 'providers.request.backoffLinear' }
  ];
  const TABS = [
    { value: 'basic', labelKey: 'providers.tabs.basic', icon: ServerIcon },
    { value: 'request', labelKey: 'providers.tabs.request', icon: RepeatIcon },
    { value: 'proxy', labelKey: 'providers.tabs.proxy', icon: GlobeIcon },
    { value: 'models', labelKey: 'providers.tabs.models', icon: BoxIcon }
  ];

  // The same `{ value, label }` pair feeds both `Select.Root items` (which lets
  // bits-ui render the closed trigger from data) and the option list, so the
  // trigger and the opened menu can never disagree, and both follow the locale.
  const authItems = $derived(
    AUTH_OPTIONS.map((option) => ({ value: option.value, label: t(option.labelKey) }))
  );
  const proxyItems = $derived(
    PROXY_OPTIONS.map((option) => ({ value: option.value, label: t(option.labelKey) }))
  );
  const backoffItems = $derived(
    BACKOFF_OPTIONS.map((option) => ({ value: option.value, label: t(option.labelKey) }))
  );

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
  // Any nested edit (API protocol toggles, header rows) changes this signature.
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
    if (dirty && !window.confirm(t('providers.action.unsavedSwitchConfirm'))) return;
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
      toast.success(t('providers.action.createdToast'));
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
      toast.success(t('providers.action.savedToast'));
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
    toast.success(t('providers.action.revertToast'));
  }

  async function remove() {
    deleting = true;
    try {
      await api(`/api/admin/providers/${deleteTarget.id}`, { method: 'DELETE' });
      toast.success(t('providers.action.deletedToast'));
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
  <title>{t('providers.title')}</title>
</svelte:head>

<PageHeader title={t('providers.heading')} description={t('providers.description')}>
  {#if selected}
    <Button variant="outline" size="sm" onclick={reset} disabled={saving || !dirty}>
      <RotateCcwIcon />
      {t('providers.action.revert')}
    </Button>
    <Button size="sm" onclick={save} disabled={saving || !dirty}>
      {#if saving}
        <Spinner class="size-3.5" />
        {t('common.state.saving')}
      {:else}
        <SaveIcon />
        {t('common.action.save')}
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
          <span class="text-xs font-semibold">{t('providers.list.title')}</span>
          <StatusBadge tone="neutral">{providers.length}</StatusBadge>
        </div>
        <Button
          variant="ghost"
          size="icon-xs"
          onclick={openCreate}
          aria-label={t('providers.action.addProvider')}
        >
          <PlusIcon />
        </Button>
      </div>
      <InputGroup.Root>
        <InputGroup.Addon><SearchIcon /></InputGroup.Addon>
        <InputGroup.Input
          class="text-[11px]"
          placeholder={t('providers.list.searchPlaceholder')}
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
          {providers.length ? t('providers.list.noResults') : t('providers.list.empty')}
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
        {t('providers.action.addProvider')}
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
            <Empty.Title>{t('providers.list.empty')}</Empty.Title>
            <Empty.Description>{t('providers.empty.noProvidersDescription')}</Empty.Description>
          </Empty.Header>
          <Empty.Content>
            <Button variant="outline" onclick={openCreate}>
              <PlusIcon />
              {t('providers.action.addProvider')}
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
                {form.enabled ? t('providers.status.enabled') : t('providers.status.disabled')}
              </StatusBadge>
              {#if dirty}
                <StatusBadge tone="warning">{t('providers.status.unsaved')}</StatusBadge>
              {/if}
            </div>
            <Card.Description class="truncate font-mono text-[10.5px]">
              {form.base_url}
            </Card.Description>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <div class="flex items-center gap-1.5">
              <span class="text-[10.5px] text-muted-foreground">{t('providers.status.enabled')}</span>
              <Switch bind:checked={form.enabled} />
            </div>
            <Button
              variant="ghost"
              size="icon-sm"
              class="text-destructive hover:bg-destructive/10 hover:text-destructive"
              onclick={() => (deleteTarget = selected)}
              aria-label={t('providers.action.deleteProvider')}
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
                {t(item.labelKey)}
              </Tabs.Trigger>
            {/each}
          </Tabs.List>
        </div>

        <div class="p-3">
          <!-- ---------------------------------------------------- basics -->
          <Tabs.Content value="basic" class="space-y-4">
            <div class="grid gap-3 sm:grid-cols-2">
              <div class="grid gap-1.5">
                <Label for="p-name">{t('providers.basic.name')}</Label>
                <Input
                  id="p-name"
                  bind:value={form.name}
                  placeholder={t('providers.basic.namePlaceholder')}
                />
              </div>
              <div class="grid gap-1.5">
                <Label for="p-slug">{t('providers.basic.id')}</Label>
                <Input
                  id="p-slug"
                  class="font-mono"
                  bind:value={form.slug}
                  placeholder={t('providers.basic.idPlaceholder')}
                />
              </div>
              <div class="grid gap-1.5 sm:col-span-2">
                <Label for="p-base">{t('providers.basic.baseUrl')}</Label>
                <Input
                  id="p-base"
                  bind:value={form.base_url}
                  placeholder={t('providers.basic.baseUrlPlaceholder')}
                />
              </div>
              <div class="grid gap-1.5 sm:col-span-2">
                <Label for="p-desc">{t('providers.basic.description')}</Label>
                <Input
                  id="p-desc"
                  bind:value={form.description}
                  placeholder={t('providers.basic.descriptionPlaceholder')}
                />
              </div>
              <div class="grid gap-1.5">
                <Label for="p-auth">{t('providers.basic.authType')}</Label>
                <Select.Root type="single" bind:value={form.auth_type} items={authItems}>
                  <Select.Trigger class="w-full" id="p-auth">
                    <Select.Value placeholder={t('providers.basic.authPlaceholder')} />
                  </Select.Trigger>
                  <Select.Content>
                    {#each authItems as option (option.value)}
                      <Select.Item value={option.value} label={option.label}>
                        {option.label}
                      </Select.Item>
                    {/each}
                  </Select.Content>
                </Select.Root>
              </div>
              <div class="grid gap-1.5">
                <Label for="p-key">
                  {t('providers.basic.apiKey')}{selected.has_api_key
                    ? t('providers.basic.apiKeyConfigured', { last4: selected.api_key_last4 })
                    : ''}
                </Label>
                {#if form.auth_type === 'none'}
                  <Input
                    id="p-key"
                    placeholder={t('providers.basic.apiKeyNotRequired')}
                    disabled
                  />
                {:else}
                  <div class="relative">
                    <Input
                      id="p-key"
                      type={showApiKey ? 'text' : 'password'}
                      bind:value={form.api_key}
                      placeholder={selected.has_api_key
                        ? t('providers.keepExisting')
                        : t('providers.basic.apiKeyPlaceholder')}
                      autocomplete="new-password"
                      class="pr-8"
                    />
                    <button
                      type="button"
                      class="absolute inset-y-0 right-0 grid w-8 place-items-center text-muted-foreground hover:text-foreground"
                      onclick={() => (showApiKey = !showApiKey)}
                      aria-label={showApiKey
                        ? t('providers.basic.hideApiKey')
                        : t('providers.basic.showApiKey')}
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
                <div class="text-xs font-medium">{t('providers.protocols.title')}</div>
                <p class="mt-0.5 text-[10.5px] text-muted-foreground">
                  {t('providers.protocols.description')}
                </p>
              </div>
              <div class="space-y-1.5">
                {#each API_PROTOCOLS as protocol (protocol.value)}
                  {@const entry = form.api_schema[protocol.value]}
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
                      bind:value={entry.endpoint}
                      placeholder={protocol.defaultEndpoint}
                      disabled={!entry.enabled}
                      aria-label={t('providers.protocols.endpointAria', {
                        protocol: protocol.label
                      })}
                    />
                  </div>
                {/each}
              </div>
            </div>
          </Tabs.Content>

          <!-- ---------------------------------------------------- models -->
          <Tabs.Content value="models">
            {#if selected}
              <ProviderModels
                providerId={selected.id}
                providerName={selected.name}
                bind:models={providerModels}
              />
            {/if}
          </Tabs.Content>

          <!-- ---------------------------------------------------- request -->
          <Tabs.Content value="request" class="space-y-4">
            <div class="grid gap-3 sm:grid-cols-2">
              <div class="grid gap-1.5">
                <Label for="p-timeout">{t('providers.request.timeout')}</Label>
                <Input id="p-timeout" type="number" min="1" bind:value={form.timeout_ms} />
                <p class="text-[10.5px] text-muted-foreground">
                  {t('providers.request.timeoutHint', {
                    duration: formatDuration(form.timeout_ms)
                  })}
                </p>
              </div>
            </div>

            <div class="space-y-3 rounded-lg border bg-muted/30 p-2.5">
              <div class="flex items-center justify-between gap-2">
                <div>
                  <div class="text-xs font-medium">{t('providers.request.retryTitle')}</div>
                  <p class="mt-0.5 text-[10.5px] text-muted-foreground">
                    {t('providers.request.retryDescription')}
                  </p>
                </div>
                {#if form.retry_count > 0}
                  <StatusBadge tone="primary">
                    {t('providers.request.attempts', { count: Number(form.retry_count) + 1 })}
                  </StatusBadge>
                {:else}
                  <StatusBadge tone="neutral">{t('providers.request.retryDisabled')}</StatusBadge>
                {/if}
              </div>

              <div class="grid gap-3 sm:grid-cols-2">
                <div class="grid gap-1.5">
                  <Label for="p-retry">{t('providers.request.retryCount')}</Label>
                  <Input
                    id="p-retry"
                    type="number"
                    min="0"
                    max="20"
                    bind:value={form.retry_count}
                  />
                </div>
                <div class="grid gap-1.5">
                  <Label for="p-backoff">{t('providers.request.backoff')}</Label>
                  <Select.Root type="single" bind:value={form.retry_backoff} items={backoffItems}>
                    <Select.Trigger class="w-full" id="p-backoff">
                      <Select.Value placeholder={backoffItems[0]?.label} />
                    </Select.Trigger>
                    <Select.Content>
                      {#each backoffItems as option (option.value)}
                        <Select.Item value={option.value} label={option.label}>
                          {option.label}
                        </Select.Item>
                      {/each}
                    </Select.Content>
                  </Select.Root>
                </div>

                {#if form.retry_backoff === 'exponential'}
                  <div class="grid gap-1.5">
                    <Label for="p-retry-initial">{t('providers.request.retryInitial')}</Label>
                    <Input
                      id="p-retry-initial"
                      type="number"
                      min="0"
                      bind:value={form.retry_initial_ms}
                    />
                  </div>
                  <div class="grid gap-1.5">
                    <Label for="p-retry-max">{t('providers.request.retryMax')}</Label>
                    <Input id="p-retry-max" type="number" min="0" bind:value={form.retry_max_ms} />
                  </div>
                {:else}
                  <div class="grid gap-1.5 sm:col-span-2">
                    <Label for="p-retry-linear">{t('providers.request.retryLinearInterval')}</Label>
                    <Input
                      id="p-retry-linear"
                      type="number"
                      min="0"
                      bind:value={form.retry_linear_interval_ms}
                    />
                    <p class="text-[10.5px] text-muted-foreground">
                      {t('providers.request.retryLinearHint', {
                        duration: formatDuration(form.retry_linear_interval_ms)
                      })}
                    </p>
                  </div>
                {/if}
              </div>
            </div>

            <HeaderRows
              label={t('providers.headers.label')}
              description={t('providers.headers.description')}
              bind:rows={headerRows}
            />
          </Tabs.Content>

          <!-- ---------------------------------------------------- proxy -->
          <Tabs.Content value="proxy" class="space-y-4">
            <div class="grid gap-3 sm:grid-cols-2">
              <div class="grid gap-1.5">
                <Label for="p-proxy-type">{t('providers.proxy.type')}</Label>
                <Select.Root type="single" bind:value={form.proxy_type} items={proxyItems}>
                  <Select.Trigger class="w-full" id="p-proxy-type">
                    <Select.Value placeholder={proxyItems[0]?.label} />
                  </Select.Trigger>
                  <Select.Content>
                    {#each proxyItems as option (option.value)}
                      <Select.Item value={option.value} label={option.label}>
                        {option.label}
                      </Select.Item>
                    {/each}
                  </Select.Content>
                </Select.Root>
                <p class="text-[10.5px] text-muted-foreground">{t('providers.proxy.typeHint')}</p>
              </div>
              <div class="grid gap-1.5">
                <Label for="p-proxy-url">{t('providers.proxy.url')}</Label>
                <Input
                  id="p-proxy-url"
                  class="font-mono text-[11px]"
                  bind:value={form.proxy_url}
                  placeholder={form.proxy_type === 'socks5'
                    ? t('providers.proxy.urlPlaceholderSocks5')
                    : t('providers.proxy.urlPlaceholderHttp')}
                  disabled={!proxyEnabled}
                />
              </div>
              <div class="grid gap-1.5">
                <Label for="p-proxy-user">{t('providers.proxy.username')}</Label>
                <Input
                  id="p-proxy-user"
                  bind:value={form.proxy_username}
                  placeholder={t('providers.optional')}
                  disabled={!proxyEnabled}
                />
              </div>
              <div class="grid gap-1.5">
                <Label for="p-proxy-pass">
                  {t('providers.proxy.password')}
                  {selected.has_proxy_password
                    ? t('providers.proxy.passwordConfigured', {
                        last4: selected.proxy_password_last4
                      })
                    : ''}
                </Label>
                <div class="relative">
                  <Input
                    id="p-proxy-pass"
                    type={showProxyPassword ? 'text' : 'password'}
                    bind:value={form.proxy_password}
                    placeholder={selected.has_proxy_password
                      ? t('providers.keepExisting')
                      : t('providers.optional')}
                    disabled={!proxyEnabled}
                    autocomplete="new-password"
                    class="pr-8"
                  />
                  <button
                    type="button"
                    class="absolute inset-y-0 right-0 grid w-8 place-items-center text-muted-foreground hover:text-foreground disabled:opacity-40"
                    onclick={() => (showProxyPassword = !showProxyPassword)}
                    disabled={!proxyEnabled}
                    aria-label={showProxyPassword
                      ? t('providers.proxy.hidePassword')
                      : t('providers.proxy.showPassword')}
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
                  {t('providers.proxy.encryptedNotice')}
                </AlertDescription>
              </Alert>
            {:else}
              <Alert>
                <GlobeIcon />
                <AlertDescription>
                  {t('providers.proxy.disabledNotice')}
                </AlertDescription>
              </Alert>
            {/if}
          </Tabs.Content>


        </div>
      </Tabs.Root>

      <Card.Footer class="justify-end border-t">
        <div class="flex items-center gap-1.5">
          <Button variant="ghost" size="sm" onclick={reset} disabled={saving || !dirty}
            >{t('providers.action.revert')}</Button
          >
          <Button size="sm" onclick={save} disabled={saving || !dirty}>
            {#if saving}
              <Spinner class="size-3.5" />
              {t('common.state.saving')}
            {:else}
              <SaveIcon />
              {t('common.action.save')}
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
            <Empty.Title>{t('providers.empty.selectTitle')}</Empty.Title>
            <Empty.Description>{t('providers.empty.selectDescription')}</Empty.Description>
          </Empty.Header>
        </Empty.Root>
      </Card.Content>
    </Card.Root>
  {/if}
</div>

<Dialog.Root bind:open={createOpen}>
  <Dialog.Content class="sm:max-w-md">
    <Dialog.Header>
      <Dialog.Title>{t('providers.create.title')}</Dialog.Title>
      <Dialog.Description>
        {t('providers.create.description')}
      </Dialog.Description>
    </Dialog.Header>
    <div class="grid gap-1.5">
      <Label for="c-name">{t('providers.basic.name')}</Label>
      <Input
        id="c-name"
        bind:value={createName}
        placeholder={t('providers.basic.namePlaceholder')}
      />
    </div>
    <Dialog.Footer>
      <Button variant="outline" onclick={() => (createOpen = false)}
        >{t('common.action.cancel')}</Button
      >
      <Button onclick={createProvider} disabled={!createName.trim()}>
        <PlusIcon />
        {t('common.action.create')}
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

<ConfirmDialog
  bind:open={() => !!deleteTarget, (value) => !value && (deleteTarget = null)}
  title={t('providers.delete.title')}
  description={t('providers.delete.description', { name: deleteTarget?.name || '' })}
  busy={deleting}
  onconfirm={remove}
/>
