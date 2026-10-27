<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { toast } from '$lib/toast';
  import Modal from '$lib/components/Modal.svelte';
  import Confirm from '$lib/components/Confirm.svelte';
  import Badge from '$lib/components/Badge.svelte';

  const PROTOCOLS = [
    ['chat.completions', 'Chat Completions', '/v1/chat/completions'],
    ['responses', 'Responses', '/v1/responses'],
    ['messages', 'Messages', '/v1/messages']
  ];
  const blank = () => ({
    name: '', base_url: '', auth_type: 'bearer', api_key: '', headers: {},
    health_check_url: '', health_check_method: 'POST', timeout_ms: 120000,
    input_price_per_million: 0, output_price_per_million: 0,
    api_types: ['chat.completions', 'responses', 'messages'], path_overrides: {},
    enabled: true
  });
  const blankModel = () => ({ model_id: '', display_name: '', api_types: ['chat.completions'], path_overrides: {}, enabled: true });

  const PROTOCOL_LABEL = Object.fromEntries(PROTOCOLS.map(([v, label]) => [v, label]));
  const PROTOCOL_DEFAULT = Object.fromEntries(PROTOCOLS.map(([v, , p]) => [v, p]));
  const apiTypeLabel = (name) => PROTOCOL_LABEL[name] || name;
  const apiTypeDefault = (name) => PROTOCOL_DEFAULT[name] || '';
  // Provider-level effective path (used as the model-level placeholder).
  const providerPath = (provider, name) => provider?.path_overrides?.[name] || apiTypeDefault(name);
  function toggleProviderProtocol(name) {
    const has = form.api_types.includes(name);
    form.api_types = has ? form.api_types.filter((v) => v !== name) : [...form.api_types, name];
    if (has) delete form.path_overrides[name];
  }
  function providerProtocolSummary(provider) {
    const types = provider?.api_types?.length ? provider.api_types : PROTOCOLS.map(([v]) => v);
    return types.map(apiTypeLabel).join(' / ');
  }

  let providers = [];
  let loading = true;
  let error = '';

  let modalOpen = false;
  let editingId = null;
  let form = blank();
  let headersText = '{}';
  const headersPlaceholder = '{"HTTP-Referer": "https://example.com"}';
  let saving = false;

  let modelsOpen = false;
  let modelsProvider = null;
  let models = [];
  let modelsLoading = false;
  let modelsError = '';
  let modelFormOpen = false;
  let modelEditingId = null;
  let modelForm = blankModel();
  let savingModel = false;
  let discovering = false;

  let deleteTarget = null;
  let deleting = false;
  let deleteModel = null;

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
      api_types: [...(p.api_types || [])], path_overrides: { ...(p.path_overrides || {}) },
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

  // ---------- provider models ----------
  async function openModels(p) {
    modelsProvider = p;
    modelsOpen = true;
    modelFormOpen = false;
    await loadModels();
  }
  async function loadModels() {
    if (!modelsProvider) return;
    modelsLoading = true;
    modelsError = '';
    try {
      models = await api(`/api/admin/providers/${modelsProvider.id}/models`);
    } catch (e) {
      modelsError = e.message;
    } finally {
      modelsLoading = false;
    }
  }
  async function discover() {
    discovering = true;
    try {
      const res = await api(`/api/admin/providers/${modelsProvider.id}/models/discover`, { method: 'POST' });
      models = res.models;
      toast(`已同步 ${res.discovered} 个模型`);
      await load();
    } catch (e) {
      toast(`同步失败：${e.message}`, 'error');
    } finally {
      discovering = false;
    }
  }
  function openAddModel() {
    modelEditingId = null;
    modelForm = blankModel();
    modelForm.api_types = [];
    modelForm.path_overrides = {};
    modelFormOpen = true;
  }
  function openEditModel(m) {
    modelEditingId = m.id;
    modelForm = {
      model_id: m.model_id, display_name: m.display_name || m.model_id,
      api_types: [...(m.api_types || [])],
      path_overrides: { ...(m.path_overrides || {}) },
      enabled: m.enabled
    };
    modelFormOpen = true;
  }
  function toggleModelProtocol(name) {
    const has = modelForm.api_types.includes(name);
    modelForm.api_types = has ? modelForm.api_types.filter((v) => v !== name) : [...modelForm.api_types, name];
    if (has) delete modelForm.path_overrides[name];
  }
  async function saveModel() {
    savingModel = true;
    try {
      const payload = { ...modelForm, display_name: modelForm.display_name || modelForm.model_id };
      if (modelEditingId) {
        await api(`/api/admin/providers/${modelsProvider.id}/models/${modelEditingId}`, { method: 'PUT', body: JSON.stringify(payload) });
        toast('模型已更新');
      } else {
        await api(`/api/admin/providers/${modelsProvider.id}/models`, { method: 'POST', body: JSON.stringify(payload) });
        toast('模型已添加');
      }
      modelFormOpen = false;
      await loadModels();
      await load();
    } catch (e) {
      toast(e.message, 'error');
    } finally {
      savingModel = false;
    }
  }
  async function removeModel() {
    try {
      await api(`/api/admin/providers/${modelsProvider.id}/models/${deleteModel.id}`, { method: 'DELETE' });
      toast('模型已删除');
      deleteModel = null;
      await loadModels();
      await load();
    } catch (e) {
      toast(e.message, 'error');
    }
  }

  const authLabel = { bearer: 'Bearer', 'x-api-key': 'x-api-key', none: '无认证' };
  onMount(load);
</script>

<svelte:head><title>Model Provider · aigw</title></svelte:head>

<header class="page-head">
  <div>
    <h1>Model Provider</h1>
    <p>配置上游服务与凭据，在 Provider 详情里维护模型清单。每个模型可覆盖支持的 API 类型和 Path。</p>
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
          <div class="wide-cell"><dt>支持的 API 类型</dt><dd>{providerProtocolSummary(p)}</dd></div>
        </dl>
        <div class="actions">
          <button class="btn sm primary-lite" on:click={() => openModels(p)}>模型管理</button>
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
    <div class="field wide"><label for="p-health">健康检查 URL（可选，默认使用模型解析后的 Path）</label><input id="p-health" class="input" bind:value={form.health_check_url} placeholder="/v1/chat/completions 或 /health" /><div class="hint">使用 target 的 upstream model 发送 max_tokens=1 的最小请求；2xx 且无 error 字段即健康。</div></div>
    <div class="field"><label for="p-in">输入价格 / 1M tokens ($)</label><input id="p-in" class="input" type="number" step="0.000001" min="0" bind:value={form.input_price_per_million} /></div>
    <div class="field"><label for="p-out">输出价格 / 1M tokens ($)</label><input id="p-out" class="input" type="number" step="0.000001" min="0" bind:value={form.output_price_per_million} /></div>
    <div class="field wide protocol-config">
      <span class="field-label">该 Provider 支持的 API 类型与默认 Path</span>
      <div class="protocol-grid">
        {#each PROTOCOLS as [value, label, defaultPath]}
          <div class="protocol-item" class:off={!form.api_types.includes(value)}>
            <label class="check">
              <input type="checkbox" checked={form.api_types.includes(value)} on:change={() => toggleProviderProtocol(value)} />
              <span>{label}</span>
            </label>
            {#if form.api_types.includes(value)}
              <input class="input mono" bind:value={form.path_overrides[value]} placeholder={defaultPath} aria-label={`${label} Path`} />
            {:else}
              <span class="proto-off mono">{defaultPath}</span>
            {/if}
          </div>
        {/each}
      </div>
      <div class="hint">模型未单独配置时继承这里；模型配置会覆盖 Provider 配置。</div>
    </div>
    <div class="field wide"><label for="p-headers">额外 Headers (JSON)</label><textarea id="p-headers" class="textarea mono" bind:value={headersText} placeholder={headersPlaceholder}></textarea></div>
    <label class="check wide"><input type="checkbox" bind:checked={form.enabled} /> 启用该 Provider</label>
  </div>
  <svelte:fragment slot="footer">
    <button class="btn ghost" on:click={() => (modalOpen = false)} disabled={saving}>取消</button>
    <button class="btn primary" on:click={save} disabled={saving || !form.name || !form.base_url}>{saving ? '保存中…' : '保存'}</button>
  </svelte:fragment>
</Modal>

<Modal open={modelsOpen} title={`模型管理 · ${modelsProvider?.name || ''}`} wide onclose={() => (modelsOpen = false)}>
  <div class="models-head">
    <div>
      <p class="models-hint">从 <code>/models</code> 同步模型，或手动添加。模型可覆盖支持的 API 类型和各自的 Path。</p>
    </div>
    <div class="models-actions">
      <button class="btn sm" type="button" on:click={discover} disabled={discovering}>{discovering ? '同步中…' : '从 /models 同步'}</button>
      <button class="btn sm primary" type="button" on:click={openAddModel}>+ 手动添加</button>
    </div>
  </div>
  {#if modelsError}<div class="alert">{modelsError}</div>{/if}
  {#if modelFormOpen}
    <div class="model-form">
      <div class="mf-grid">
        <div class="field"><label for="m-id">Model ID（上游真实名称）</label><input id="m-id" class="input mono" bind:value={modelForm.model_id} placeholder="deepseek/deepseek-flash" /></div>
        <div class="field"><label for="m-name">显示名称</label><input id="m-name" class="input" bind:value={modelForm.display_name} placeholder="可选" /></div>
      </div>
      <div class="proto-select">
        <span class="pf-label">支持的 API 类型</span>
        {#each PROTOCOLS as [value, label]}
          <label class="check"><input type="checkbox" checked={modelForm.api_types.includes(value)} on:change={() => toggleModelProtocol(value)} /> <span>{label}</span></label>
        {/each}
        <span class="inherit-hint">全部不勾选 = 继承 Provider 配置</span>
      </div>
      {#if modelForm.api_types.length}
        <div class="path-overrides">
          {#each modelForm.api_types as t}
            <div class="field"><label for={`mo-${t}`}>{apiTypeLabel(t)} Path 覆盖</label><input id={`mo-${t}`} class="input mono" bind:value={modelForm.path_overrides[t]} placeholder={providerPath(modelsProvider, t)} /></div>
          {/each}
        </div>
      {/if}
      <div class="mf-actions">
        <button class="btn ghost sm" type="button" on:click={() => (modelFormOpen = false)}>取消</button>
        <button class="btn primary sm" type="button" on:click={saveModel} disabled={savingModel || !modelForm.model_id}>{savingModel ? '保存中…' : (modelEditingId ? '保存修改' : '添加模型')}</button>
      </div>
    </div>
  {/if}
  <div class="model-list">
    {#if modelsLoading}
      <div class="empty"><div class="spinner" style="margin:auto"></div></div>
    {:else if !models.length}
      <div class="empty">还没有模型。点击「从 /models 同步」或手动添加。</div>
    {:else}
      {#each models as m (m.id)}
        <div class="model-row">
          <div class="model-main">
            <div class="model-title">
              <b class="mono">{m.model_id}</b>
              {#if m.display_name && m.display_name !== m.model_id}<span class="muted">{m.display_name}</span>{/if}
              <Badge tone={m.source === 'discovered' ? 'blue' : 'violet'}>{m.source === 'discovered' ? '自动同步' : '手动'}</Badge>
              {#if !m.enabled}<Badge tone="neutral">停用</Badge>{/if}
            </div>
            <div class="model-meta">
              <span>协议：{m.api_types?.length ? m.api_types.map(apiTypeLabel).join(' / ') : `继承 Provider（${providerProtocolSummary(modelsProvider)}）`}</span>
              {#if m.path_overrides && Object.keys(m.path_overrides).length}
                {#each Object.entries(m.path_overrides) as [k, v]}<span class="mono override">{apiTypeLabel(k)}: {v}</span>{/each}
              {/if}
            </div>
          </div>
          <div class="model-row-actions">
            <button class="btn sm" on:click={() => openEditModel(m)}>编辑</button>
            <button class="btn sm danger-lite" on:click={() => (deleteModel = m)}>删除</button>
          </div>
        </div>
      {/each}
    {/if}
  </div>
  <svelte:fragment slot="footer">
    <button class="btn primary" on:click={() => (modelsOpen = false)}>完成</button>
  </svelte:fragment>
</Modal>

<Confirm open={!!deleteTarget} title="删除 Provider" message={`确定删除 ${deleteTarget?.name || ''}？其下模型会一并删除，引用它的 routing target 会阻止删除。`} busy={deleting} onconfirm={remove} onclose={() => (deleteTarget = null)} />
<Confirm open={!!deleteModel} title="删除模型" message={`确定删除模型 ${deleteModel?.model_id || ''}？引用它的 routing target 将失效。`} onconfirm={removeModel} onclose={() => (deleteModel = null)} />

<style>
  .page-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 15px; margin-bottom: 20px; }
  h1 { margin: 0; font-size: 23px; letter-spacing: -.03em; }
  .page-head p { margin: 6px 0 0; color: #64748b; font-size: 13px; max-width: 720px; line-height: 1.55; }
  .alert { background: #fef2f2; color: #b91c1c; border-radius: 9px; padding: 11px 14px; margin-bottom: 15px; font-size: 13px; }

  .protocol-config { gap: 9px; padding: 12px; border: 1px solid #e2e8f0; border-radius: 10px; background: #f8fafc; }
  .field-label { font-size: 12px; color: #475569; font-weight: 650; }
  .protocol-grid { display: flex; flex-direction: column; gap: 8px; }
  .protocol-item { display: grid; grid-template-columns: 150px 1fr; gap: 10px; align-items: center; }
  .protocol-item.off { opacity: .6; }
  .proto-off { font-size: 11.5px; color: #94a3b8; }
  .wide-cell { grid-column: 1 / -1; }
  .inherit-hint { font-size: 10.5px; color: #94a3b8; }

  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(340px, 1fr)); gap: 14px; }
  .provider { padding: 17px; display: flex; flex-direction: column; }
  .top { display: flex; justify-content: space-between; gap: 10px; align-items: flex-start; }
  .name { font-weight: 700; font-size: 15px; }
  .url { color: #64748b; font-size: 11px; margin-top: 4px; word-break: break-all; }
  dl { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin: 16px 0; padding: 12px 0; border-top: 1px solid #edf2f7; border-bottom: 1px solid #edf2f7; }
  dt { font-size: 10.5px; color: #94a3b8; margin-bottom: 3px; }
  dd { margin: 0; font-size: 12px; font-weight: 600; }
  .muted { color: #94a3b8; font-weight: 400; }
  .ok { color: #047857; } .warn { color: #b45309; }
  .actions { display: flex; gap: 8px; justify-content: flex-end; flex-wrap: wrap; }
  .danger-lite { color: #b91c1c; }
  .danger-lite:hover { background: #fef2f2; border-color: #fecaca; }
  .primary-lite { color: #1d4ed8; border-color: #bfdbfe; background: #eff6ff; }
  .form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
  .wide { grid-column: 1 / -1; }
  .check { display: flex; align-items: center; gap: 8px; font-size: 13px; color: #334155; }

  .models-head { display: flex; justify-content: space-between; gap: 14px; align-items: flex-start; margin-bottom: 13px; }
  .models-hint { margin: 0; color: #64748b; font-size: 12px; line-height: 1.6; }
  .models-actions { display: flex; gap: 7px; flex-shrink: 0; }
  .model-form { background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 10px; padding: 13px; margin-bottom: 13px; display: flex; flex-direction: column; gap: 11px; }
  .mf-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
  .proto-select { display: flex; gap: 18px; flex-wrap: wrap; align-items: center; }
  .pf-label { font-size: 12px; color: #475569; font-weight: 650; }
  .path-overrides { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 9px; }
  .mf-actions { display: flex; justify-content: flex-end; gap: 7px; }
  .model-list { border: 1px solid #eef2f7; border-radius: 10px; overflow: hidden; }
  .model-row { display: flex; justify-content: space-between; gap: 12px; align-items: center; padding: 11px 13px; border-bottom: 1px solid #edf2f7; }
  .model-row:last-child { border-bottom: 0; }
  .model-title { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
  .model-title b { font-size: 12.5px; }
  .model-meta { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 5px; color: #64748b; font-size: 11px; }
  .override { background: #f1f5f9; border-radius: 4px; padding: 1px 5px; }
  .model-row-actions { display: flex; gap: 6px; flex-shrink: 0; }
  code { background: #f1f5f9; padding: 1px 5px; border-radius: 4px; }
  @media (max-width: 700px) {
    .form-grid, .mf-grid, .path-overrides { grid-template-columns: 1fr; }
    .models-head { flex-direction: column; }
    .wide { grid-column: auto; }
  }
</style>
