<script>
  import { onMount } from 'svelte';
  import { api, fmtTime } from '$lib/api';
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
        toast.success('API Key 已更新');
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
      toast.success('已复制到剪贴板');
      setTimeout(() => (copied = false), 1600);
    } catch {
      toast.error('复制失败，请手动选择复制');
    }
  }

  async function remove() {
    deleting = true;
    try {
      await api(`/api/admin/keys/${deleteTarget.id}`, { method: 'DELETE' });
      toast.success('API Key 已删除');
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
  <title>API Keys · aigw</title>
</svelte:head>

<PageHeader
  title="API Keys"
  description="客户端使用这些 Key 调用网关。Key 仅展示一次，数据库只保存 SHA-256 哈希。"
>
  <Button onclick={openCreate}>
    <PlusIcon />
    创建 API Key
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
      <Card.Title class="text-sm">密钥列表</Card.Title>
      <StatusBadge tone="neutral">{keys.length} 个</StatusBadge>
    </div>
  </Card.Header>
  <Card.Content class="px-0">
    <Table.Root>
      <Table.Header>
        <Table.Row>
          <Table.Head>名称</Table.Head>
          <Table.Head>App</Table.Head>
          <Table.Head>Key Prefix</Table.Head>
          <Table.Head>允许模型</Table.Head>
          <Table.Head>状态</Table.Head>
          <Table.Head>最后使用</Table.Head>
          <Table.Head>过期时间</Table.Head>
          <Table.Head>创建时间</Table.Head>
          <Table.Head class="text-right">操作</Table.Head>
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
                  <Empty.Title>还没有 API Key</Empty.Title>
                  <Empty.Description>
                    创建后即可通过 <code class="font-mono">Authorization: Bearer aigw_...</code>
                    调用网关。
                  </Empty.Description>
                </Empty.Header>
                <Empty.Content>
                  <Button variant="outline" onclick={openCreate}>
                    <PlusIcon />
                    创建 API Key
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
                  <span class="text-[11px] text-muted-foreground">全部模型</span>
                {/if}
              </Table.Cell>
              <Table.Cell>
                <StatusBadge tone={item.enabled ? 'success' : 'neutral'} dot>
                  {item.enabled ? '启用' : '停用'}
                </StatusBadge>
              </Table.Cell>
              <Table.Cell class="whitespace-nowrap text-[11px] tabular-nums">
                {item.last_used_at ? fmtTime(item.last_used_at) : '从未'}
              </Table.Cell>
              <Table.Cell class="whitespace-nowrap text-[11px] tabular-nums">
                {item.expires_at ? fmtTime(item.expires_at) : '永不过期'}
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
        {createdKey ? 'API Key 已创建' : editingId ? '编辑 API Key' : '创建 API Key'}
      </Dialog.Title>
      <Dialog.Description>
        {createdKey
          ? '请立即复制并妥善保存，关闭后无法再次查看完整 Key。'
          : '配置名称、业务标识、可用模型范围与有效期。'}
      </Dialog.Description>
    </Dialog.Header>

    {#if createdKey}
      <div class="space-y-3">
        <div class="flex items-center gap-1.5 rounded-lg border bg-muted/50 p-2">
          <code class="min-w-0 flex-1 break-all font-mono text-[11px]">{createdKey}</code>
          <Button size="sm" variant="outline" onclick={copyKey}>
            {#if copied}
              <CheckIcon />
              已复制
            {:else}
              <CopyIcon />
              复制
            {/if}
          </Button>
        </div>
        <div class="space-y-1.5">
          <div class="flex items-center gap-1.5 text-[11px] font-medium text-muted-foreground">
            <TerminalIcon class="size-3.5" />
            调用示例
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
          <Label for="k-name">名称</Label>
          <Input id="k-name" bind:value={form.name} placeholder="production-app" />
        </div>
        <div class="grid gap-1.5">
          <Label for="k-app">App / 业务标识</Label>
          <Input id="k-app" bind:value={form.app} placeholder="my-app" />
          <p class="text-[10.5px] text-muted-foreground">会展示在日志与统计中，便于按业务筛选。</p>
        </div>
        <div class="grid gap-1.5">
          <Label for="k-models">允许的模型（逗号分隔，留空允许全部）</Label>
          <Input
            id="k-models"
            class="font-mono text-[11px]"
            bind:value={modelsText}
            placeholder="deepseek-flash, gpt-4o-mini"
          />
        </div>
        <div class="grid gap-1.5">
          <Label for="k-expires">过期时间（可选）</Label>
          <Input id="k-expires" type="datetime-local" bind:value={form.expires_at} />
        </div>
        <Separator />
        <div class="flex items-center justify-between gap-2">
          <div class="space-y-0.5">
            <div class="text-xs font-medium">启用该 Key</div>
            <div class="text-[10.5px] text-muted-foreground">停用后客户端立即无法访问</div>
          </div>
          <Switch bind:checked={form.enabled} />
        </div>
      </div>
    {/if}

    <Dialog.Footer>
      {#if createdKey}
        <Button onclick={() => (modalOpen = false)}>完成</Button>
      {:else}
        <Button variant="outline" onclick={() => (modalOpen = false)} disabled={saving}>取消</Button>
        <Button onclick={save} disabled={saving || !form.name}>
          {#if saving}
            <Spinner class="size-3.5" />
            保存中…
          {:else}
            <CheckIcon />
            保存
          {/if}
        </Button>
      {/if}
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

<ConfirmDialog
  bind:open={() => !!deleteTarget, (value) => !value && (deleteTarget = null)}
  title="删除 API Key"
  description={`确定删除 ${deleteTarget?.name || ''}？使用该 Key 的客户端将立即无法访问。`}
  busy={deleting}
  onconfirm={remove}
/>
