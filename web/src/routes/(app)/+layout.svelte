<script>
  import Sidebar from '$lib/components/Sidebar.svelte';
  import { getToken } from '$lib/api';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';

  let ready = false;
  onMount(() => {
    if (!getToken()) goto('/login');
    else ready = true;
  });
</script>

{#if ready}
  <div class="shell">
    <Sidebar />
    <main><div class="content"><slot /></div></main>
  </div>
{:else}
  <div class="loading"><div class="spinner"></div></div>
{/if}

<style>
  .shell { display: flex; min-height: 100vh; }
  main { flex: 1; min-width: 0; background: #eef2f7; }
  .content { width: 100%; max-width: 1600px; padding: 20px 24px 40px; }
  .loading { min-height: 100vh; display: grid; place-items: center; }
  @media (max-width: 800px) {
    .shell { flex-direction: column; }
    .content { padding: 14px 12px 28px; }
  }
</style>
