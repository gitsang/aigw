<script>
  import { cn } from '$lib/utils.js';

  let { title = '', description = '', class: className, children } = $props();
</script>

<div
  class={cn('flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between', className)}
>
  <div class="min-w-0 space-y-1">
    <h1 class="text-base font-semibold tracking-tight text-foreground">{title}</h1>
    {#if description}
      <p class="max-w-3xl text-xs leading-relaxed text-muted-foreground">{description}</p>
    {/if}
  </div>
  {#if children}
    <div class="flex shrink-0 flex-wrap items-center gap-1.5">{@render children()}</div>
  {/if}
</div>
