<script>
  import SunIcon from '@lucide/svelte/icons/sun';
  import MoonIcon from '@lucide/svelte/icons/moon';
  import { toggleMode, mode } from 'mode-watcher';
  import { Button } from '$lib/components/ui/button/index.js';
  import { t } from '$lib/i18n/index.svelte.js';

  let { class: className = '' } = $props();
</script>

<Button
  variant="ghost"
  size="icon-sm"
  class={className}
  onclick={toggleMode}
  aria-label={t('app.theme.toggle')}
  title={t('app.theme.toggle')}
>
  {#if mode.current === 'dark'}
    <SunIcon />
  {:else}
    <MoonIcon />
  {/if}
</Button>
