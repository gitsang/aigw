<script>
  import SunIcon from '@lucide/svelte/icons/sun';
  import MoonIcon from '@lucide/svelte/icons/moon';
  import { toggleMode, mode } from 'mode-watcher';
  import { Button } from '$lib/components/ui/button/index.js';

  let { class: className = '' } = $props();
</script>

<Button
  variant="ghost"
  size="icon-sm"
  class={className}
  onclick={toggleMode}
  aria-label="切换主题"
  title="切换主题"
>
  {#if mode.current === 'dark'}
    <SunIcon />
  {:else}
    <MoonIcon />
  {/if}
</Button>
