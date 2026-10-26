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
    <main><slot /></main>
  </div>
{:else}
  <div class="loading"><div class="spinner"></div></div>
{/if}

<style>
  .shell { display: flex; min-height: 100vh; }
  main { flex: 1; min-width: 0; padding: 26px 30px 50px; max-width: 1600px; }
  .loading { min-height: 100vh; display: grid; place-items: center; }
  @media (max-width: 800px) {
    .shell { flex-direction: column; }
    main { padding: 18px 14px 40px; }
  }
</style>
