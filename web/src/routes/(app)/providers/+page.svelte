<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { toast } from '$lib/toast';
  import Modal from '$lib/components/Modal.svelte';
  import Confirm from '$lib/components/Confirm.svelte';
  import Badge from '$lib/components/Badge.svelte';

  const blank = () => ({
    name: '', base_url: '', auth_type: 'bearer', api_key: '', headers: {},
    health_check_url: '', health_check_method: 'POST', timeout_ms: 120000,
    input_price_per_million: 0, output_price_per_million: 0, enabled: true
  });

  let providers = [];
  let loading = true;
  let error = '';
  let modalOpen = false;
  let editingId = null;
  let form = blank();
  let headersText = '{}';
  const headersPlaceholder = '{"HTTP-Referer": "https://example.com"}';
  let saving = false;
  let deleteTarget = null;
  let deleting = false;

  async function load() {
    loading = true;
    error = '';
    try {
      providers = await api('/api/admin/providers');
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function openCreate() {
    editingId = null;
    form = blank();
    headersText = '{}';
    modalOpen = true;
  }

  function openEdit(p) {
    editingId = p.id;
    form = {
      name: p.name, base_url: p.base_url, auth_type: p.auth_type, api_key: '',
      headers: p.headers || {}, health_check_url: p.health_check_url || '',
      health_check_method: p.health_check_method || 'POST', timeout_ms: p.timeout_ms,
      input_price_per_million: p.input_price_per_million, output_price_per_million: p.output_price_per_million,
      enabled: p.enabled
    };
    headersText = JSON.stringify(p.headers || {}, null, 2);
    modalOpen = true;
  }

  async function save() {
    saving = true;
    try {
      let headers = {};
      if (headersText.trim()) {
        headers = JSON.parse(headersText);
        if (Array.isArray(headers) || typeof headers !== 'object' || headers === null) throw new Error('headers 必须是 JSON 对象');
      }
      const payload = { ...form, headers };
      if (editingId) await api(`/api/admin/providers/${editingId}`, { method: 'PUT', body: JSON.stringify(payload) });
      else await api('/api/admin/providers', { method: 'POST', body: JSON.stringify(payload) });
      toast(editingId ? 'Provider 已更新' : 'Provider 已创建');
      modalOpen = false;
      await load();
    } catch (e) {
      toast(e.message, 'error');
    } finally {
      saving = false;
    }
  }

  async function remove() {
    deleting = true;
    try {
      await api(`/api/admin/providers/${deleteTarget.id}`, { method: 'DELETE' });
      toast('Provider 已删除');
      deleteTarget = null;
      await load();
    } catch (e) {
      toast(e.message, 'error');
    } finally {
      deleting = false;
    }
  }

  const authLabel = { bearer: 'Bearer', 'x-api-key': 'x-api-key', none: '无认证' };
  onMount(load);
</script>

<svelte:head><title>Model Provider · aigw</title></svelte:head>

<header class="page-head">
  <div>
    <h1>Model Provider</h1>
    <p>配置上游 LLM 服务、凭据、健康检查方式与 token 计费单价。API Key 加密存储。</p>
  </div>
  <button class="btn primary" on:click={openCreate}>+ 添加 Provider</button>
</header>

{#if error}<div class="alert">{error}</div>{/if}

<section class="grid">
  {#if loading}
    <div class="empty" style="grid-column:1/-1"><div class="spinner" style="margin:auto"></div></div>
  {:else if !providers.length}
    <div class="panel empty" style="grid-column:1/-1">还没有 Provider，先添加一个上游服务。</div>
  {:else}
    {#each providers as p (p.id)}
      <article class="panel provider">
        <div class="top">
          <div>
            <div class="name">{p.name}</div>
            <div class="url mono">{p.base_url}</div>
          </div>
          <Badge tone={p.enabled ? 'green' : 'neutral'} dot>{p.enabled ? '启用' : '停用'}</Badge>
        </div>
        <dl>
          <div><dt>认证</dt><dd>{authLabel[p.auth_type] || p.auth_type} {#if p.has_api_key}<span class="mono muted">••••{p.api_key_last4}</span>{/if}</dd></div>
          <div><dt>超时</dt><dd>{Math.round((p.timeout_ms || 0) / 1000)}s</dd></div>
          <div><dt>健康 Target</dt><dd><span class={p.healthy_targets === p.total_targets ? 'ok' : 'warn'}>{p.healthy_targets}/{p.total_targets}</span></dd></div>
          <div><dt>单价 / 1M</dt><dd class="mono">${p.input_price_per_million} in · ${p.output_price_per_million} out</dd></div>
        </dl>
        <div class="actions">
          <button class="btn sm" on:click={() => openEdit(p)}>编辑</button>
          <button class="btn sm danger-lite" on:click={() => (deleteTarget = p)}>删除</button>
        </div>
      </article>
    {/each}
  {/if}
</section>

<Modal open={modalOpen} title={editingId ? '编辑 Provider' : '添加 Provider'} wide onclose={() => (modalOpen = false)}>
  <div class="form-grid">
    <div class="field"><label for="p-name">名称</label><input id="p-name" class="input" bind:value={form.name} placeholder="provider1" /></div>
    <div class="field"><label for="p-url">Base URL</label><input id="p-url" class="input" bind:value={form.base_url} placeholder="https://api.example.com/v1" /></div>
    <div class="field"><label for="p-auth">认证方式</label>
      <select id="p-auth" class="select" bind:value={form.auth_type}>
        <option value="bearer">Authorization: Bearer</option>
        <option value="x-api-key">x-api-key</option>
        <option value="none">无认证</option>
      </select>
    </div>
    <div class="field"><label for="p-key">API Key {editingId ? '（留空不修改）' : ''}</label><input id="p-key" class="input" type="password" bind:value={form.api_key} placeholder={editingId ? '••••••••' : 'sk-...'} autocomplete="new-password" /></div>
    <div class="field"><label for="p-timeout">请求超时 (ms)</label><input id="p-timeout" class="input" type="number" min="1" bind:value={form.timeout_ms} /></div>
    <div class="field"><label for="p-health-method">健康检查方法</label>
      <select id="p-health-method" class="select" bind:value={form.health_check_method}><option value="POST">POST</option><option value="GET">GET</option></select>
    </div>
    <div class="field wide"><label for="p-health">健康检查 URL（可选，默认 POST 对应上游路径）</label><input id="p-health" class="input" bind:value={form.health_check_url} placeholder="/v1/chat/completions 或 /health" /><div class="hint">使用该 target 的 upstream model 发送 max_tokens=1 的最小请求；2xx 且无 error 字段即健康。</div></div>
    <div class="field"><label for="p-in">输入价格 / 1M tokens ($)</label><input id="p-in" class="input" type="number" step="0.000001" min="0" bind:value={form.input_price_per_million} /></div>
    <div class="field"><label for="p-out">输出价格 / 1M tokens ($)</label><input id="p-out" class="input" type="number" step="0.000001" min="0" bind:value={form.output_price_per_million} /></div>
    <div class="field wide"><label for="p-headers">额外 Headers (JSON)</label><textarea id="p-headers" class="textarea mono" bind:value={headersText} placeholder={headersPlaceholder}></textarea></div>
    <label class="check wide"><input type="checkbox" bind:checked={form.enabled} /> 启用该 Provider</label>
  </div>
  <svelte:fragment slot="footer">
    <button class="btn ghost" on:click={() => (modalOpen = false)} disabled={saving}>取消</button>
    <button class="btn primary" on:click={save} disabled={saving || !form.name || !form.base_url}>{saving ? '保存中…' : '保存'}</button>
  </svelte:fragment>
</Modal>

<Confirm open={!!deleteTarget} title="删除 Provider" message={`确定删除 ${deleteTarget?.name || ''}？引用它的 routing target 会阻止删除。`} busy={deleting} onconfirm={remove} onclose={() => (deleteTarget = null)} />

<style>
  .page-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 15px; margin-bottom: 20px; }
  h1 { margin: 0; font-size: 23px; letter-spacing: -.03em; }
  .page-head p { margin: 6px 0 0; color: #64748b; font-size: 13px; max-width: 650px; }
  .alert { background: #fef2f2; color: #b91c1c; border-radius: 9px; padding: 11px 14px; margin-bottom: 15px; font-size: 13px; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(330px, 1fr)); gap: 14px; }
  .provider { padding: 17px; display: flex; flex-direction: column; }
  .top { display: flex; justify-content: space-between; gap: 10px; align-items: flex-start; }
  .name { font-weight: 700; font-size: 15px; }
  .url { color: #64748b; font-size: 11px; margin-top: 4px; word-break: break-all; }
  dl { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin: 16px 0; padding: 12px 0; border-top: 1px solid #edf2f7; border-bottom: 1px solid #edf2f7; }
  dt { font-size: 10.5px; color: #94a3b8; margin-bottom: 3px; }
  dd { margin: 0; font-size: 12px; font-weight: 600; }
  .muted { color: #94a3b8; font-weight: 400; }
  .ok { color: #047857; } .warn { color: #b45309; }
  .actions { display: flex; gap: 8px; justify-content: flex-end; }
  .danger-lite { color: #b91c1c; }
  .danger-lite:hover { background: #fef2f2; border-color: #fecaca; }
  .form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
  .wide { grid-column: 1 / -1; }
  .check { display: flex; align-items: center; gap: 8px; font-size: 13px; color: #334155; }
  @media (max-width: 620px) { .form-grid { grid-template-columns: 1fr; } .wide { grid-column: auto; } }
</style>
