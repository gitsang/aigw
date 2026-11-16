<script>
  import '../app.css';
  import { onMount } from 'svelte';
  import { ModeWatcher } from 'mode-watcher';
  import { Toaster } from '$lib/components/ui/sonner/index.js';
  import { initLocale } from '$lib/i18n/index.svelte.js';

  let { children } = $props();

  // Language is a client-only preference (the build is a static SPA).
  onMount(initLocale);
</script>

<ModeWatcher />
{@render children?.()}
<Toaster position="bottom-right" closeButton />
