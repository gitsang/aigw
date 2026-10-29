<script>
  import '../app.css';
  import { ModeWatcher } from 'mode-watcher';
  import { Toaster } from '$lib/components/ui/sonner/index.js';

  let { children } = $props();
</script>

<ModeWatcher />
{@render children?.()}
<Toaster position="bottom-right" closeButton />
