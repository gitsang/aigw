<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { toast } from 'svelte-sonner';

  import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
  import SettingsIcon from '@lucide/svelte/icons/settings';
  import RouteIcon from '@lucide/svelte/icons/route';
  import LayersIcon from '@lucide/svelte/icons/layers';

  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Switch } from '$lib/components/ui/switch/index.js';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import { Alert, AlertDescription } from '$lib/components/ui/alert/index.js';
  import { Separator } from '$lib/components/ui/separator/index.js';
  import { Spinner } from '$lib/components/ui/spinner/index.js';

  import PageHeader from '$lib/components/app/page-header.svelte';
  import StatusBadge from '$lib/components/app/status-badge.svelte';
  import { t } from '$lib/i18n/index.svelte.js';

  let settings = $state({ auto_model_match: false });
  let loading = $state(true);
  let saving = $state(false);
  let error = $state('');

  async function load() {
    loading = true;
    error = '';
    try {
      settings = await api('/api/admin/settings');
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function toggleAutoModelMatch(next) {
    if (saving) return;
    const previous = settings.auto_model_match;
    settings = { ...settings, auto_model_match: next };
    saving = true;
    try {
      settings = await api('/api/admin/settings', {
        method: 'PUT',
        body: JSON.stringify(settings)
      });
      toast.success(next ? t('settings.autoMatch.enabledToast') : t('settings.autoMatch.disabledToast'));
    } catch (e) {
      settings = { ...settings, auto_model_match: previous };
      toast.error(e.message);
    } finally {
      saving = false;
    }
  }

  onMount(load);
</script>

<svelte:head>
  <title>{t('settings.title')}</title>
</svelte:head>

<PageHeader
  title={t('settings.heading')}
  description={t('settings.description')}
/>

{#if error}
  <Alert variant="destructive">
    <TriangleAlertIcon />
    <AlertDescription>{error}</AlertDescription>
  </Alert>
{/if}

{#if loading}
  <Skeleton class="h-48 w-full rounded-lg" />
{:else}
  <Card.Root class="gap-0">
    <Card.Header class="border-b">
      <div class="flex items-center gap-1.5">
        <SettingsIcon class="size-3.5 text-muted-foreground" />
        <Card.Title class="text-sm">{t('settings.autoMatch.title')}</Card.Title>
      </div>
      <Card.Description class="text-[11px]">
        {t('settings.autoMatch.description')}
      </Card.Description>
    </Card.Header>
    <Card.Content class="space-y-3 pt-3">
      <div class="flex flex-wrap items-start justify-between gap-3 rounded-lg border p-3">
        <div class="min-w-0 space-y-1">
          <div class="flex flex-wrap items-center gap-1.5">
            <span class="text-xs font-medium">{t('settings.autoMatch.label')}</span>
            <StatusBadge tone={settings.auto_model_match ? 'success' : 'neutral'} dot>
              {settings.auto_model_match ? t('settings.autoMatch.on') : t('settings.autoMatch.off')}
            </StatusBadge>
            {#if saving}
              <Spinner class="size-3" />
            {/if}
          </div>
          <p class="max-w-2xl text-[11px] leading-relaxed text-muted-foreground">
            {t('settings.autoMatch.body')}
          </p>
        </div>
        <Switch
          checked={settings.auto_model_match}
          onCheckedChange={toggleAutoModelMatch}
          disabled={saving}
          aria-label={t('settings.autoMatch.label')}
        />
      </div>

      <div class="rounded-lg border bg-muted/30 p-3">
        <div class="mb-1.5 flex items-center gap-1.5">
          <RouteIcon class="size-3.5 text-muted-foreground" />
          <span class="text-xs font-medium">{t('settings.example.title')}</span>
        </div>
        <ul class="space-y-1 text-[11px] leading-relaxed text-muted-foreground">
          <li>
            {t('settings.example.requestModel')}
            <code class="rounded bg-background px-1 font-mono text-[10.5px]"
              >deepseek/deepseek-flash</code
            >
            {t('settings.example.withoutVirtualModel')}
          </li>
          <li class="pl-4">
            {t('settings.example.step1')}
          </li>
          <li class="pl-4">
            {t('settings.example.step2')}
            <code class="rounded bg-background px-1 font-mono text-[10.5px]"
              >yllm/deepseek/deepseek-flash</code
            >;
          </li>
          <li class="pl-4">
            {t('settings.example.step3')}
          </li>
        </ul>
      </div>

      <Separator />

      <Alert>
        <LayersIcon />
        <AlertDescription>
          {t('settings.note')}
          <span class="font-mono">virtual_model_name</span>
          {t('settings.noteKey')}
          <span class="font-mono">auto-match</span>
          {t('settings.noteTail')}
        </AlertDescription>
      </Alert>
    </Card.Content>
    <Card.Footer class="justify-end border-t">
      <Button variant="outline" size="sm" onclick={load} disabled={loading || saving}>
        {t('common.action.reload')}
      </Button>
    </Card.Footer>
  </Card.Root>
{/if}
