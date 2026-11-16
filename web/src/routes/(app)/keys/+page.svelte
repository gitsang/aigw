<script>
  import { onMount } from 'svelte';
  import { api, fmtTime } from '$lib/api';
  import { t } from '$lib/i18n/index.svelte.js';
  import { toast } from 'svelte-sonner';

  import PlusIcon from '@lucide/svelte/icons/plus';
  import PencilIcon from '@lucide/svelte/icons/pencil';
  import Trash2Icon from '@lucide/svelte/icons/trash-2';
  import KeyRoundIcon from '@lucide/svelte/icons/key-round';
  import CopyIcon from '@lucide/svelte/icons/copy';
  import CheckIcon from '@lucide/svelte/icons/check';
  import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
  import InboxIcon from '@lucide/svelte/icons/inbox';
  import TerminalIcon from '@lucide/svelte/icons/terminal';

  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Label } from '$lib/components/ui/label/index.js';
  import { Switch } from '$lib/components/ui/switch/index.js';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import * as Table from '$lib/components/ui/table/index.js';
  import * as Empty from '$lib/components/ui/empty/index.js';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import { Alert, AlertDescription } from '$lib/components/ui/alert/index.js';
  import { Separator } from '$lib/components/ui/separator/index.js';
  import { Spinner } from '$lib/components/ui/spinner/index.js';

  import PageHeader from '$lib/components/app/page-header.svelte';
  import StatusBadge from '$lib/components/app/status-badge.svelte';
  import ConfirmDialog from '$lib/components/app/confirm-dialog.svelte';

  const blank = () => ({ name: '', app: '', allowed_models: [], enabled: true, expires_at: null });

  let keys = $state([]);
  let loading = $state(true);
  let error = $state('');
  let modalOpen = $state(false);
  let editingId = $state(null);
  let form = $state(blank());
  let modelsText = $state('');
  let saving = $state(false);
  let createdKey = $state('');
  let deleteTarget = $state(null);
  let deleting = $state(false);
  let copied = $state(false);

  /** Dialog title and description, keyed by the current dialog mode. */
  const dialogTitle = $derived(
    createdKey
      ? t('keys.dialog.createdTitle')
      : editingId
        ? t('keys.dialog.editTitle')
        : t('keys.dialog.createTitle')
  );
  const dialogDescription = $derived(
    createdKey ? t('keys.dialog.createdDescription') : t('keys.dialog.formDescription')
  );
  const deleteTitle = $derived(t('keys.delete.title'));
  const deleteDescription = $derived(
    t('keys.delete.description', { name: deleteTarget?.name || '' })
  );

  async function load() {
    loading = true;
    error = '';
    try {
      keys = await api('/api/admin/keys');
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function openCreate() {
    editingId = null;
    form = blank();
    modelsText = '';
    createdKey = '';
    modalOpen = true;
  }

  function openEdit(item) {
    editingId = item.id;
    form = {
      name: item.name,
      app: item.app || '',
      allowed_models: item.allowed_models || [],
      enabled: item.enabled,
      expires_at: item.expires_at || null
    };
    modelsText = (item.allowed_models || []).join(', ');
    createdKey = '';
    modalOpen = true;
  }

  async function save() {
    saving = true;
    try {
      const payload = {
        ...form,
        allowed_models: modelsText
          .split(',')
          .map((value) => value.trim())
          .filter(Boolean),
        expires_at: form.expires_at ? new Date(form.expires_at).toISOString() : null
      };
      if (editingId) {
        await api(`/api/admin/keys/${editingId}`, { method: 'PUT', body: JSON.stringify(payload) });
        toast.success(t('keys.toast.updated'));
        modalOpen = false;
        await load();
      } else {
        const data = await api('/api/admin/keys', { method: 'POST', body: JSON.stringify(payload) });
        createdKey = data.key;
        await load();
      }
    } catch (e) {
      toast.error(e.message);
    } finally {
      saving = false;
    }
  }

  async function copyKey() {
    try {
      await navigator.clipboard.writeText(createdKey);
      copied = true;
      toast.success(t('keys.toast.copied'));
      setTimeout(() => (copied = false), 1600);
    } catch {
      toast.error(t('keys.toast.copyFailed'));
    }
  }

  async function remove() {
    deleting = true;
    try {
      await api(`/api/admin/keys/${deleteTarget.id}`, { method: 'DELETE' });
      toast.success(t('keys.toast.deleted'));
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
  <title>{t('keys.title')}</title>
</svelte:head>

<PageHeader
  title={t('keys.heading')}
  description={t('keys.description')}
>
  <Button onclick={openCreate}>
    <PlusIcon />
    {t('keys.create')}
  </Button>
</PageHeader>

{#if error}
  <Alert variant="destructive">
    <TriangleAlertIcon />
    <AlertDescription>{error}</AlertDescription>
  </Alert>
{/if}

<Card.Root class="gap-0">
  <Card.Header class="border-b">
    <div class="flex items-center justify-between gap-2">
      <Card.Title class="text-sm">{t('keys.list.title')}</Card.Title>
      <StatusBadge tone="neutral">{t('keys.list.count', { count: keys.length })}</StatusBadge>
    </div>
  </Card.Header>
  <Card.Content class="px-0">
    <Table.Root>
      <Table.Header>
        <Table.Row>
          <Table.Head>{t('keys.column.name')}</Table.Head>
          <Table.Head>{t('keys.column.app')}</Table.Head>
          <Table.Head>{t('keys.column.keyPrefix')}</Table.Head>
          <Table.Head>{t('keys.column.allowedModels')}</Table.Head>
          <Table.Head>{t('keys.column.status')}</Table.Head>
          <Table.Head>{t('keys.column.lastUsed')}</Table.Head>
          <Table.Head>{t('keys.column.expiresAt')}</Table.Head>
          <Table.Head>{t('keys.column.createdAt')}</Table.Head>
          <Table.Head class="text-right">{t('keys.column.actions')}</Table.Head>
        </Table.Row>
      </Table.Header>
      <Table.Body>
        {#if loading}
          {#each Array(4) as _, i (i)}
            <Table.Row>
              {#each Array(9) as __, j (j)}
                <Table.Cell><Skeleton class="h-4 w-full" /></Table.Cell>
              {/each}
            </Table.Row>
          {/each}
        {:else if !keys.length}
          <Table.Row class="hover:bg-transparent">
            <Table.Cell colspan={9}>
              <Empty.Root class="border-0 py-10">
                <Empty.Header>
                  <Empty.Media variant="icon"><KeyRoundIcon /></Empty.Media>
                  <Empty.Title>{t('keys.empty.title')}</Empty.Title>
                  <Empty.Description>
                    {t('keys.empty.descriptionBefore')}
                    <code class="font-mono">Authorization: Bearer aigw_...</code>
                    {t('keys.empty.descriptionAfter')}
                  </Empty.Description>
                </Empty.Header>
                <Empty.Content>
                  <Button variant="outline" onclick={openCreate}>
                    <PlusIcon />
                    {t('keys.create')}
                  </Button>
                </Empty.Content>
              </Empty.Root>
            </Table.Cell>
          </Table.Row>
        {:else}
          {#each keys as item (item.id)}
            <Table.Row>
              <Table.Cell class="font-medium">{item.name}</Table.Cell>
              <Table.Cell class="text-[11px]">{item.app || '—'}</Table.Cell>
              <Table.Cell class="font-mono text-[11px] text-muted-foreground">
                {item.key_prefix}…
              </Table.Cell>
              <Table.Cell>
                {#if item.allowed_models?.length}
                  <div class="flex min-w-0 flex-wrap gap-1">
                    {#each item.allowed_models as model (model)}
                      <StatusBadge tone="neutral" class="shrink-0 font-mono">{model}</StatusBadge>
                    {/each}
                  </div>
                {:else}
                  <span class="text-[11px] text-muted-foreground">{t('keys.allModels')}</span>
                {/if}
              </Table.Cell>
              <Table.Cell>
                <StatusBadge tone={item.enabled ? 'success' : 'neutral'} dot>
                  {item.enabled ? t('common.state.enabled') : t('common.state.disabled')}
                </StatusBadge>
              </Table.Cell>
              <Table.Cell class="whitespace-nowrap text-[11px] tabular-nums">
                {item.last_used_at ? fmtTime(item.last_used_at) : t('keys.never')}
              </Table.Cell>
              <Table.Cell class="whitespace-nowrap text-[11px] tabular-nums">
                {item.expires_at ? fmtTime(item.expires_at) : t('keys.noExpiry')}
              </Table.Cell>
              <Table.Cell class="whitespace-nowrap text-[11px] tabular-nums text-muted-foreground">
                {fmtTime(item.created_at)}
              </Table.Cell>
              <Table.Cell>
                <div class="flex items-center justify-end gap-1">
                  <Button variant="ghost" size="xs" onclick={() => openEdit(item)}>
                    <PencilIcon />
                  </Button>
                  <Button
                    variant="ghost"
                    size="xs"
                    class="text-destructive hover:bg-destructive/10 hover:text-destructive"
                    onclick={() => (deleteTarget = item)}
                  >
                    <Trash2Icon />
                  </Button>
                </div>
              </Table.Cell>
            </Table.Row>
          {/each}
        {/if}
      </Table.Body>
    </Table.Root>
  </Card.Content>
</Card.Root>

<Dialog.Root bind:open={modalOpen}>
  <Dialog.Content class={createdKey ? 'sm:max-w-xl' : 'sm:max-w-md'}>
    <Dialog.Header>
      <Dialog.Title>
        {dialogTitle}
      </Dialog.Title>
      <Dialog.Description>
        {dialogDescription}
      </Dialog.Description>
    </Dialog.Header>

    {#if createdKey}
      <div class="space-y-3">
        <div class="flex items-center gap-1.5 rounded-lg border bg-muted/50 p-2">
          <code class="min-w-0 flex-1 break-all font-mono text-[11px]">{createdKey}</code>
          <Button size="sm" variant="outline" onclick={copyKey}>
            {#if copied}
              <CheckIcon />
              {t('keys.dialog.copied')}
            {:else}
              <CopyIcon />
              {t('common.action.copy')}
            {/if}
          </Button>
        </div>
        <div class="space-y-1.5">
          <div class="flex items-center gap-1.5 text-[11px] font-medium text-muted-foreground">
            <TerminalIcon class="size-3.5" />
            {t('keys.dialog.example')}
          </div>
          <pre
            class="overflow-x-auto rounded-lg border bg-muted/50 p-2.5 font-mono text-[10.5px] leading-relaxed text-muted-foreground">{`curl http://localhost:8080/v1/chat/completions \\
  -H "Authorization: Bearer ${createdKey}" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"deepseek-flash","messages":[{"role":"user","content":"hi"}]}'`}</pre>
        </div>
      </div>
    {:else}
      <div class="grid gap-3">
        <div class="grid gap-1.5">
          <Label for="k-name">{t('keys.form.name')}</Label>
          <Input
            id="k-name"
            bind:value={form.name}
            placeholder={t('keys.form.namePlaceholder')}
          />
        </div>
        <div class="grid gap-1.5">
          <Label for="k-app">{t('keys.form.app')}</Label>
          <Input id="k-app" bind:value={form.app} placeholder={t('keys.form.appPlaceholder')} />
          <p class="text-[10.5px] text-muted-foreground">{t('keys.form.appHint')}</p>
        </div>
        <div class="grid gap-1.5">
          <Label for="k-models">{t('keys.form.allowedModels')}</Label>
          <Input
            id="k-models"
            class="font-mono text-[11px]"
            bind:value={modelsText}
            placeholder="deepseek-flash, gpt-4o-mini"
          />
        </div>
        <div class="grid gap-1.5">
          <Label for="k-expires">{t('keys.form.expiresAt')}</Label>
          <Input id="k-expires" type="datetime-local" bind:value={form.expires_at} />
        </div>
        <Separator />
        <div class="flex items-center justify-between gap-2">
          <div class="space-y-0.5">
            <div class="text-xs font-medium">{t('keys.form.enabled')}</div>
            <div class="text-[10.5px] text-muted-foreground">{t('keys.form.enabledHint')}</div>
          </div>
          <Switch bind:checked={form.enabled} />
        </div>
      </div>
    {/if}

    <Dialog.Footer>
      {#if createdKey}
        <Button onclick={() => (modalOpen = false)}>{t('keys.dialog.done')}</Button>
      {:else}
        <Button variant="outline" onclick={() => (modalOpen = false)} disabled={saving}>
          {t('common.action.cancel')}
        </Button>
        <Button onclick={save} disabled={saving || !form.name}>
          {#if saving}
            <Spinner class="size-3.5" />
            {t('common.state.saving')}
          {:else}
            <CheckIcon />
            {t('common.action.save')}
          {/if}
        </Button>
      {/if}
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

<ConfirmDialog
  bind:open={() => !!deleteTarget, (value) => !value && (deleteTarget = null)}
  title={deleteTitle}
  description={deleteDescription}
  busy={deleting}
  onconfirm={remove}
/>
