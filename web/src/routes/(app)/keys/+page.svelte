<script>
  import { onMount } from 'svelte';
  import { api, fmtTime } from '$lib/api';
  import { toast } from '$lib/toast';
  import Modal from '$lib/components/Modal.svelte';
  import Confirm from '$lib/components/Confirm.svelte';
  import Badge from '$lib/components/Badge.svelte';

  const blank = () => ({ name: '', app: '', allowed_models: [], enabled: true, expires_at: null });
  let keys = [];
  let loading = true;
  let error = '';
  let modalOpen = false;
  let editingId = null;
  let form = blank();
  let modelsText = '';
  let saving = false;
  let createdKey = '';
  let deleteTarget = null;
  let deleting = false;
  let copied = false;

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

  function openEdit(k) {
    editingId = k.id;
    form = {
      name: k.name, app: k.app || '', allowed_models: k.allowed_models || [],
      enabled: k.enabled, expires_at: k.expires_at || null
    };
    modelsText = (k.allowed_models || []).join(', ');
    createdKey = '';
    modalOpen = true;
  }

  async function save() {
    saving = true;
    try {
      const payload = {
        ...form,
        allowed_models: modelsText.split(',').map((s) => s.trim()).filter(Boolean),
        expires_at: form.expires_at ? new Date(form.expires_at).toISOString() : null
      };
      if (editingId) {
        await api(`/api/admin/keys/${editingId}`, { method: 'PUT', body: JSON.stringify(payload) });
        toast('API Key 已更新');
        modalOpen = false;
        await load();
      } else {
        const data = await api('/api/admin/keys', { method: 'POST', body: JSON.stringify(payload) });
        createdKey = data.key;
        await load();
      }
    } catch (e) {
      toast(e.message, 'error');
    } finally {
      saving = false;
    }
  }

  async function copyKey() {
    try {
      await navigator.clipboard.writeText(createdKey);
      copied = true;
      setTimeout(() => (copied = false), 1600);
    } catch {
      toast('复制失败，请手动选择复制', 'error');
    }
  }

  async function remove() {
    deleting = true;
    try {
      await api(`/api/admin/keys/${deleteTarget.id}`, { method: 'DELETE' });
      toast('API Key 已删除');
      deleteTarget = null;
      await load();
    } catch (e) {
      toast(e.message, 'error');
    } finally {
      deleting = false;
    }
  }

  onMount(load);
</script>

<svelte:head><title>API Keys · aigw</title></svelte:head>

<header class="page-head">
  <div>
    <h1>API Keys</h1>
    <p>客户端使用这些 Key 调用网关。Key 仅展示一次，数据库只保存 SHA-256 哈希。</p>
  </div>
  <button class="btn primary" on:click={openCreate}>+ 创建 API Key</button>
</header>

{#if error}<div class="alert">{error}</div>{/if}

<section class="panel">
  <div class="table-wrap">
    <table class="data">
      <thead><tr><th>Name</th><th>App</th><th>Key Prefix</th><th>允许模型</th><th>状态</th><th>最后使用</th><th>过期时间</th><th>创建时间</th><th></th></tr></thead>
      <tbody>
        {#if loading}
          <tr><td colspan="9"><div class="empty"><div class="spinner" style="margin:auto"></div></div></td></tr>
        {:else if !keys.length}
          <tr><td colspan="9"><div class="empty">还没有 API Key。创建后即可通过 <code>Authorization: Bearer aigw_...</code> 调用网关。</div></td></tr>
        {:else}
          {#each keys as k (k.id)}
            <tr>
              <td><b>{k.name}</b></td>
              <td>{k.app || '—'}</td>
              <td class="mono">{k.key_prefix}…</td>
              <td>
                {#if k.allowed_models?.length}
                  <div class="chips">{#each k.allowed_models as m}<span class="chip mono">{m}</span>{/each}</div>
                {:else}<span class="any">全部模型</span>{/if}
              </td>
              <td><Badge tone={k.enabled ? 'green' : 'neutral'} dot>{k.enabled ? '启用' : '停用'}</Badge></td>
              <td class="nowrap">{k.last_used_at ? fmtTime(k.last_used_at) : '从未'}</td>
              <td class="nowrap">{k.expires_at ? fmtTime(k.expires_at) : '永不过期'}</td>
              <td class="nowrap">{fmtTime(k.created_at)}</td>
              <td class="row-actions">
                <button class="btn sm" on:click={() => openEdit(k)}>编辑</button>
                <button class="btn sm danger-lite" on:click={() => (deleteTarget = k)}>删除</button>
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>
</section>

<Modal open={modalOpen} title={editingId ? '编辑 API Key' : '创建 API Key'} onclose={() => (modalOpen = false)}>
  {#if createdKey}
    <div class="created">
      <h3>API Key 已创建</h3>
      <p>请立即复制并妥善保存。关闭后无法再次查看完整 Key。</p>
      <div class="key-box">
        <code>{createdKey}</code>
        <button class="btn sm" on:click={copyKey}>{copied ? '已复制' : '复制'}</button>
      </div>
      <div class="usage mono">curl http://localhost:8080/v1/chat/completions \<br />&nbsp;&nbsp;-H "Authorization: Bearer {createdKey}" \<br />&nbsp;&nbsp;-H "Content-Type: application/json" \<br />&nbsp;&nbsp;-d '&#123;"model":"deepseek-flash","messages":[&#123;"role":"user","content":"hi"&#125;]&#125;'</div>
    </div>
  {:else}
    <div class="form">
      <div class="field"><label for="k-name">名称</label><input id="k-name" class="input" bind:value={form.name} placeholder="production-app" /></div>
      <div class="field"><label for="k-app">App / 业务标识</label><input id="k-app" class="input" bind:value={form.app} placeholder="my-app" /><div class="hint">会展示在日志与统计中，便于按业务筛选。</div></div>
      <div class="field"><label for="k-models">允许的模型（逗号分隔，留空允许全部）</label><input id="k-models" class="input mono" bind:value={modelsText} placeholder="deepseek-flash, gpt-4o-mini" /></div>
      <div class="field"><label for="k-expires">过期时间（可选）</label><input id="k-expires" class="input" type="datetime-local" bind:value={form.expires_at} /></div>
      <label class="check"><input type="checkbox" bind:checked={form.enabled} /> 启用该 Key</label>
    </div>
  {/if}
  <svelte:fragment slot="footer">
    {#if createdKey}
      <button class="btn primary" on:click={() => (modalOpen = false)}>完成</button>
    {:else}
      <button class="btn ghost" on:click={() => (modalOpen = false)} disabled={saving}>取消</button>
      <button class="btn primary" on:click={save} disabled={saving || !form.name}>{saving ? '保存中…' : '保存'}</button>
    {/if}
  </svelte:fragment>
</Modal>

<Confirm open={!!deleteTarget} title="删除 API Key" message={`确定删除 ${deleteTarget?.name || ''}？使用该 Key 的客户端将立即无法访问。`} busy={deleting} onconfirm={remove} onclose={() => (deleteTarget = null)} />

<style>
  .page-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 15px; margin-bottom: 20px; }
  h1 { margin: 0; font-size: 23px; letter-spacing: -.03em; }
  .page-head p { margin: 6px 0 0; color: #64748b; font-size: 13px; }
  .alert { background: #fef2f2; color: #b91c1c; border-radius: 9px; padding: 11px 14px; margin-bottom: 15px; font-size: 13px; }
  .chips { display: flex; gap: 4px; flex-wrap: wrap; }
  .any { color: #94a3b8; font-size: 11.5px; }
  .nowrap { white-space: nowrap; }
  .row-actions { display: flex; gap: 5px; justify-content: flex-end; }
  .danger-lite { color: #b91c1c; }
  .danger-lite:hover { background: #fef2f2; border-color: #fecaca; }
  .form { display: flex; flex-direction: column; gap: 15px; }
  .check { display: flex; align-items: center; gap: 8px; font-size: 13px; color: #334155; }
  .created h3 { margin: 0 0 7px; font-size: 15px; }
  .created p { margin: 0 0 14px; color: #64748b; font-size: 12.5px; }
  .key-box { display: flex; gap: 9px; align-items: center; background: #f1f5f9; border: 1px solid #e2e8f0; border-radius: 9px; padding: 11px; }
  .key-box code { flex: 1; word-break: break-all; font-size: 12px; }
  .usage { margin-top: 14px; background: #f8fafc; color: #475569; border: 1px solid #e2e8f0; padding: 13px; border-radius: 9px; font-size: 11px; line-height: 1.8; overflow-x: auto; }
</style>
