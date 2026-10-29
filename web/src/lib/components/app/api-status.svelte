<script>
  import { onMount } from 'svelte';
  import { getToken, apiURL } from '$lib/api';
  import StatusBadge from './status-badge.svelte';

  let state = $state('checking');

  async function ping() {
    const token = getToken();
    if (!token) {
      state = 'unauthorized';
      return;
    }
    try {
      const res = await fetch(apiURL('/api/admin/session'), {
        headers: { Authorization: `Bearer ${token}` }
      });
      state = res.ok ? 'ok' : res.status === 401 ? 'unauthorized' : 'down';
    } catch {
      state = 'down';
    }
  }

  onMount(() => {
    ping();
    const timer = setInterval(ping, 20000);
    return () => clearInterval(timer);
  });
</script>

{#if state === 'checking'}
  <StatusBadge tone="neutral" dot class="hidden sm:inline-flex">检测中</StatusBadge>
{:else if state === 'ok'}
  <StatusBadge tone="success" dot class="hidden sm:inline-flex">API 在线</StatusBadge>
{:else if state === 'unauthorized'}
  <StatusBadge tone="warning" dot class="hidden sm:inline-flex">未授权</StatusBadge>
{:else}
  <StatusBadge tone="danger" dot class="hidden sm:inline-flex">API 离线</StatusBadge>
{/if}
