<script>
  import { onMount } from 'svelte';
  import { api, fmtLatency, fmtTime } from '$lib/api';
  import { toast } from '$lib/toast';
  import Modal from '$lib/components/Modal.svelte';
  import Confirm from '$lib/components/Confirm.svelte';
  import Badge from '$lib/components/Badge.svelte';

  const APIS = [
    ['chat.completions', 'Chat Completions'],
    ['responses', 'Responses'],
    ['messages', 'Messages']
  ];
  const PROTOCOL_NAMES = {
    'chat.completions': 'Chat Completions',
    responses: 'Responses',
    messages: 'Messages'
  };
  const apiLabel = (v) => PROTOCOL_NAMES[v] || v;

  const blankTarget = (role = 'lb') => ({
    role, provider_id: '', provider_model_id: '', upstream_model: '',
    weight: 50, priority: 0, enabled: true
  });
  const blankRule = () => ({
    name: '', virtual_model: '', api_types: APIS.map(([v]) => v), enabled: true,
    retry_count: 3, retry_policy: 'exponential', fallback_policy: 'fallback-first',
    retry_delay_ms: 250, max_backoff_ms: 5000, targets: [blankTarget('lb'), blankTarget('fallback')]
  });

  let rules = [];
  let providers = [];
  let providerModels = {};
  let loading = true;
  let error = '';
  let modalOpen = false;
  let editingId = null;
  let form = blankRule();
  let saving = false;
  let deleteTarget = null;
  let deleting = false;

  async function load() {
    loading = true;
    error = '';
    try {
      [rules, providers] = await Promise.all([api('/api/admin/routes'), api('/api/admin/providers')]);
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function openCreate() {
    editingId = null;
    form = blankRule();
    for (const p of providers) await ensureModels(p.id);
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

  async function openEdit(rule) {
    editingId = rule.id;
    form = {
      name: rule.name, virtual_model: rule.virtual_model, api_types: [...rule.api_types],
      enabled: rule.enabled, retry_count: rule.retry_count, retry_policy: rule.retry_policy,
      fallback_policy: rule.fallback_policy, retry_delay_ms: rule.retry_delay_ms,
      max_backoff_ms: rule.max_backoff_ms,
      targets: rule.targets.map((t) => ({
        id: t.id, role: t.role, provider_id: t.provider_id,
        provider_model_id: t.provider_model_id || '',
        upstream_model: t.upstream_model,
        weight: t.weight, priority: t.priority, enabled: t.enabled
      }))
    };
    for (const t of form.targets) if (t.provider_id) await ensureModels(t.provider_id);
    modalOpen = true;
  }

  async function addTarget(role) {
    const t = blankTarget(role);
    if (providers.length) {
      t.provider_id = providers[0].id;
      const list = await ensureModels(t.provider_id);
      t.provider_model_id = list[0]?.id || '';
    }
    form.targets = [...form.targets, t];
  }

  function removeTarget(index) {
    form.targets = form.targets.filter((_, i) => i !== index);
  }

  function toggleApi(value) {
    form.api_types = form.api_types.includes(value)
      ? form.api_types.filter((v) => v !== value)
      : [...form.api_types, value];
  }

  async function ensureModels(providerID) {
    if (!providerID) return [];
    if (!providerModels[providerID]) {
      try {
        providerModels[providerID] = await api(`/api/admin/providers/${providerID}/models`);
        providerModels = providerModels;
      } catch (e) {
        providerModels[providerID] = [];
        providerModels = providerModels;
        toast(`加载模型失败：${e.message}`, 'error');
      }
    }
    return providerModels[providerID];
  }

  async function onProviderChange(target) {
    target.provider_model_id = '';
    target.upstream_model = '';
    const list = await ensureModels(target.provider_id);
    if (list.length) target.provider_model_id = list[0].id;
  }

  function modelsFor(providerID) { return providerModels[providerID] || []; }
  function modelOf(target) {
    return modelsFor(target.provider_id).find((m) => m.id === target.provider_model_id);
  }
  function targetProtocols(target) {
    const m = modelOf(target);
    return m?.api_types?.length ? m.api_types.map(apiLabel).join(' / ') : '跟随全局默认';
  }
  function targetPathHint(target) {
    const m = modelOf(target);
    const overrides = m?.path_overrides || {};
    const keys = Object.keys(overrides);
    if (keys.length) return keys.map((k) => `${apiLabel(k)} → ${overrides[k]}`).join('  ');
    return '使用全局默认 Path';
  }

  async function save() {
    if (!form.virtual_model.trim()) return toast('请填写对外模型名', 'error');
    if (!form.targets.length) return toast('至少配置一个 target', 'error');
    if (form.targets.some((t) => !t.provider_id || !t.provider_model_id)) return toast('每个 target 都需要 provider 和模型', 'error');
    saving = true;
    try {
      const payload = { ...form, targets: form.targets.map((t) => ({ ...t })) };
      if (editingId) await api(`/api/admin/routes/${editingId}`, { method: 'PUT', body: JSON.stringify(payload) });
      else await api('/api/admin/routes', { method: 'POST', body: JSON.stringify(payload) });
      toast(editingId ? 'Routing 规则已更新' : 'Routing 规则已创建');
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
      await api(`/api/admin/routes/${deleteTarget.id}`, { method: 'DELETE' });
      toast('Routing 规则已删除');
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

<svelte:head><title>Routing · aigw</title></svelte:head>

<header class="page-head">
  <div>
    <h1>Routing</h1>
    <p>虚拟模型 → 加权 LB target → fallback。健康检查每分钟执行，不健康 LB 自动移出，全部下线时使用 fallback。</p>
  </div>
  <button class="btn primary" on:click={openCreate} disabled={!providers.length}>+ 添加规则</button>
</header>

{#if !providers.length && !loading}
  <div class="alert">请先在 <a href="/providers">Model Provider</a> 添加至少一个上游服务。</div>
{/if}
{#if error}<div class="alert">{error}</div>{/if}

<section class="rules">
  {#if loading}
    <div class="panel empty"><div class="spinner" style="margin:auto"></div></div>
  {:else if !rules.length}
    <div class="panel empty">还没有 Routing 规则。创建一条规则，将对外模型映射到上游 provider。</div>
  {:else}
    {#each rules as rule (rule.id)}
      <article class="panel rule">
        <div class="rule-head">
          <div>
            <div class="model-line">
              <span class="mono vmodel">{rule.virtual_model}</span>
              <Badge tone={rule.enabled ? 'green' : 'neutral'} dot>{rule.enabled ? '启用' : '停用'}</Badge>
              <Badge tone="violet">{rule.fallback_policy}</Badge>
              <Badge tone="amber">{rule.retry_policy} · {rule.retry_count}x</Badge>
            </div>
            <div class="rule-name">{rule.name} · {rule.retry_delay_ms}ms base / {rule.max_backoff_ms}ms max backoff</div>
            <div class="apis">{#each rule.api_types as t}<span class="chip">{APIS.find(([v]) => v === t)?.[1] || t}</span>{/each}</div>
          </div>
          <div class="rule-actions">
            <button class="btn sm" on:click={() => openEdit(rule)}>编辑</button>
            <button class="btn sm danger-lite" on:click={() => (deleteTarget = rule)}>删除</button>
          </div>
        </div>

        <div class="targets">
          <div class="target-head"><span>角色</span><span>Provider / Upstream</span><span>接口</span><span>权重</span><span>健康</span></div>
          {#each rule.targets as t}
            <div class="target" class:off={!t.enabled}>
              <div><Badge tone={t.role === 'fallback' ? 'amber' : 'blue'}>{t.role === 'fallback' ? 'Fallback' : 'LB'}</Badge></div>
              <div class="prov">
                <b>{t.provider_name}</b>
                <span class="mono">{t.upstream_model}</span>
              </div>
              <div class="mono api-cell">
                {t.supported_api_types?.length ? t.supported_api_types.map(apiLabel).join(' / ') : apiLabel(t.api_type)}
                <div class="sub">{t.path_overrides && Object.keys(t.path_overrides).length ? Object.entries(t.path_overrides).map(([k,v]) => `${apiLabel(k)} → ${v}`).join('  ') : '全局默认 Path'}</div>
              </div>
              <div class="mono weight">{t.role === 'fallback' ? '—' : `${t.weight}`}</div>
              <div>
                {#if t.role === 'fallback'}
                  <Badge tone="neutral">仅兜底</Badge>
                {:else if t.health?.healthy}
                  <Badge tone="green" dot>健康</Badge>
                {:else}
                  <Badge tone="red" dot>不健康</Badge>
                {/if}
                {#if t.health?.checked_at}
                  <div class="sub">{fmtLatency(t.health.latency_ms)} · {fmtTime(t.health.checked_at)}</div>
                {/if}
                {#if t.health && !t.health.healthy && t.health.error}
                  <div class="health-error" title={t.health.error}>{t.health.error}</div>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      </article>
    {/each}
  {/if}
</section>

<Modal open={modalOpen} title={editingId ? '编辑 Routing 规则' : '添加 Routing 规则'} wide onclose={() => (modalOpen = false)}>
  <div class="form-grid">
    <div class="field"><label for="r-name">规则名称</label><input id="r-name" class="input" bind:value={form.name} placeholder="deepseek-flash-lb" /></div>
    <div class="field"><label for="r-model">对外模型名 (virtual model)</label><input id="r-model" class="input mono" bind:value={form.virtual_model} placeholder="deepseek-flash" /></div>
    <div class="field wide"><span class="label">支持的入口 API</span>
      <div class="check-row">
        {#each APIS as [value, label]}
          <label class="check"><input type="checkbox" checked={form.api_types.includes(value)} on:change={() => toggleApi(value)} /><span class="mono">{label}</span></label>
        {/each}
      </div>
    </div>
    <div class="field"><label for="r-fallback">Fallback 策略</label>
      <select id="r-fallback" class="select" bind:value={form.fallback_policy}>
        <option value="fallback-first">fallback-first（每轮先 LB 全部尝试，再 fallback；整体循环重试）</option>
        <option value="retry-first">retry-first（单个路由重试 N 次，再换下一个，最后 fallback）</option>
      </select>
    </div>
    <div class="field"><label for="r-retry">重试次数</label><input id="r-retry" class="input" type="number" min="1" max="20" bind:value={form.retry_count} /></div>
    <div class="field"><label for="r-policy">退避策略</label>
      <select id="r-policy" class="select" bind:value={form.retry_policy}><option value="exponential">指数退避</option><option value="fixed">固定退避</option></select>
    </div>
    <div class="field"><label for="r-delay">基础退避 (ms)</label><input id="r-delay" class="input" type="number" min="0" bind:value={form.retry_delay_ms} /></div>
    <div class="field"><label for="r-backoff">最大退避 (ms)</label><input id="r-backoff" class="input" type="number" min="0" bind:value={form.max_backoff_ms} /></div>
    <label class="check"><input type="checkbox" bind:checked={form.enabled} /> 启用该规则</label>
  </div>

  <div class="target-editor">
    <div class="te-head">
      <h3>Targets</h3>
      <div><button class="btn sm" type="button" on:click={() => addTarget('lb')}>+ LB</button><button class="btn sm" type="button" on:click={() => addTarget('fallback')}>+ Fallback</button></div>
    </div>
    <p class="te-note">
      这里只选择 <b>Provider + 模型</b>。模型支持的 API 类型和 Path 在 Provider 的「模型管理」中配置；
      未配置时使用「全局 API 配置」的默认值。请求会优先使用与入口相同的协议，否则使用模型支持的第一个协议自动转换。
    </p>
    {#each form.targets as t, i}
      <div class="te-row">
        <div class="field"><label for={`t-role-${i}`}>角色</label>
          <select id={`t-role-${i}`} class="select" bind:value={t.role}>
            <option value="lb">LB</option><option value="fallback">Fallback</option>
          </select>
        </div>
        <div class="field"><label for={`t-provider-${i}`}>Provider</label>
          <select id={`t-provider-${i}`} class="select" bind:value={t.provider_id} on:change={() => onProviderChange(t)}>
            <option value="">选择…</option>
            {#each providers as p}<option value={p.id}>{p.name}</option>{/each}
          </select>
        </div>
        <div class="field"><label for={`t-model-${i}`}>模型</label>
          <select id={`t-model-${i}`} class="select" bind:value={t.provider_model_id}>
            <option value="">选择模型…</option>
            {#each modelsFor(t.provider_id) as m}<option value={m.id}>{m.model_id}</option>{/each}
          </select>
        </div>
        <div class="target-summary">
          <div class="ts-row"><span class="ts-label">协议</span><span>{targetProtocols(t)}</span></div>
          <div class="ts-row"><span class="ts-label">Path</span><span class="mono">{targetPathHint(t)}</span></div>
          {#if t.provider_id && !modelsFor(t.provider_id).length}
            <div class="ts-warn">该 Provider 还没有模型，请先到 Provider 详情添加。</div>
          {/if}
        </div>
        <div class="field weight"><label for={`t-weight-${i}`}>权重</label><input id={`t-weight-${i}`} class="input" type="number" min="1" bind:value={t.weight} /></div>
        <button class="btn sm danger-lite remove" type="button" on:click={() => removeTarget(i)} title="删除 target">✕</button>
      </div>
    {/each}
  </div>

  <svelte:fragment slot="footer">
    <button class="btn ghost" on:click={() => (modalOpen = false)} disabled={saving}>取消</button>
    <button class="btn primary" on:click={save} disabled={saving}>{saving ? '保存中…' : '保存'}</button>
  </svelte:fragment>
</Modal>

<Confirm open={!!deleteTarget} title="删除 Routing 规则" message={`确定删除规则 ${deleteTarget?.virtual_model || ''}？使用该模型的客户端将立即失败。`} busy={deleting} onconfirm={remove} onclose={() => (deleteTarget = null)} />

<style>
  .page-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 15px; margin-bottom: 20px; }
  h1 { margin: 0; font-size: 23px; letter-spacing: -.03em; }
  .page-head p { margin: 6px 0 0; color: #64748b; font-size: 13px; max-width: 720px; line-height: 1.55; }
  .alert { background: #fffbeb; border: 1px solid #fde68a; color: #92400e; border-radius: 9px; padding: 11px 14px; margin-bottom: 15px; font-size: 13px; }
  .alert a { color: #1d4ed8; text-decoration: underline; }
  .rules { display: flex; flex-direction: column; gap: 14px; }
  .rule { padding: 17px 18px; }
  .rule-head { display: flex; justify-content: space-between; gap: 14px; }
  .model-line { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
  .vmodel { font-size: 16px; font-weight: 700; }
  .rule-name { color: #64748b; font-size: 11.5px; margin-top: 6px; }
  .apis { display: flex; gap: 5px; margin-top: 9px; flex-wrap: wrap; }
  .rule-actions { display: flex; gap: 7px; }
  .danger-lite { color: #b91c1c; }
  .targets { margin-top: 14px; border: 1px solid #edf2f7; border-radius: 10px; overflow: hidden; }
  .target-head, .target { display: grid; grid-template-columns: 78px minmax(170px, 1.4fr) minmax(140px, 1fr) 60px minmax(100px, .7fr); gap: 10px; align-items: center; padding: 9px 12px; }
  .target-head { background: #f8fafc; color: #94a3b8; font-size: 10.5px; font-weight: 650; text-transform: uppercase; letter-spacing: .05em; }
  .target { border-top: 1px solid #edf2f7; font-size: 12px; }
  .target.off { opacity: .55; }
  .prov b { display: block; font-size: 12.5px; }
  .prov span { color: #64748b; font-size: 11px; }
  .sub { color: #94a3b8; font-size: 10.5px; margin-top: 3px; }
  .health-error { color: #b91c1c; font-size: 10px; max-width: 260px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; margin-top: 3px; }
  .form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; margin-bottom: 20px; }
  .wide { grid-column: 1 / -1; }
  .check-row { display: flex; gap: 20px; flex-wrap: wrap; padding: 8px 0; }
  .check { display: flex; align-items: center; gap: 7px; font-size: 12.5px; color: #334155; }
  .label { font-size: 12px; color: #475569; font-weight: 650; }
  .target-editor { border-top: 1px solid #e2e8f0; padding-top: 15px; }
  .te-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
  .te-head h3 { margin: 0; font-size: 13.5px; }
  .te-note { margin: 0 0 12px; padding: 9px 12px; border-radius: 8px; background: #f1f7ff; border: 1px solid #dbeafe; color: #475569; font-size: 11.5px; line-height: 1.65; }
  .te-note b { color: #1d4ed8; font-weight: 650; }
  .te-head > div { display: flex; gap: 6px; }
  .te-row { display: grid; grid-template-columns: 90px 1fr 1.25fr 1.35fr 70px 32px; gap: 8px; align-items: end; padding: 10px; background: #f8fafc; border-radius: 9px; margin-bottom: 8px; }
  .target-summary { min-width: 0; padding-bottom: 8px; display: flex; flex-direction: column; gap: 3px; }
  .ts-row { display: flex; gap: 6px; font-size: 11px; color: #475569; line-height: 1.4; }
  .ts-label { color: #94a3b8; flex: 0 0 30px; }
  .ts-row .mono { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .ts-warn { color: #b45309; font-size: 10.5px; }
  .remove { align-self: end; margin-bottom: 1px; padding: 8px; }
  @media (max-width: 900px) {
    .te-row { grid-template-columns: 1fr 1fr; }
    .target-head { display: none; }
    .target { grid-template-columns: 70px 1fr; }
    .form-grid { grid-template-columns: 1fr; }
    .wide { grid-column: auto; }
  }
</style>
