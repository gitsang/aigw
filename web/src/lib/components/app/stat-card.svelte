<script>
  import * as Card from '$lib/components/ui/card/index.js';
  import { cn } from '$lib/utils.js';

  let {
    label = '',
    value = '',
    hint = '',
    icon: Icon = undefined,
    trend = '',
    class: className
  } = $props();
</script>

<Card.Root class={cn('gap-0', className)}>
  <Card.Content class="flex items-start justify-between gap-2">
    <div class="min-w-0 space-y-1">
      <div class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
        {label}
      </div>
      <div class="truncate text-xl font-semibold tracking-tight tabular-nums text-foreground">
        {value}
      </div>
      {#if hint}
        <div class="truncate text-[11px] text-muted-foreground">{hint}</div>
      {/if}
    </div>
    {#if Icon}
      <div
        class="grid size-7 shrink-0 place-items-center rounded-md bg-muted/70 text-muted-foreground"
      >
        <Icon class="size-3.5" />
      </div>
    {/if}
  </Card.Content>
</Card.Root>
