<script>
  import PlusIcon from '@lucide/svelte/icons/plus';
  import XIcon from '@lucide/svelte/icons/x';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Label } from '$lib/components/ui/label/index.js';

  let {
    label = '额外 Header',
    hint = '',
    description = '',
    rows = $bindable([]),
    onadd = () => {}
  } = $props();

  function addRow() {
    rows = [...rows, { name: '', value: '' }];
    onadd();
  }

  function removeRow(index) {
    rows = rows.filter((_, i) => i !== index);
  }
</script>

<div class="grid gap-1.5">
  <div class="flex items-center justify-between gap-2">
    <Label>{label}</Label>
    <Button variant="ghost" size="xs" type="button" onclick={addRow}>
      <PlusIcon />
      添加
    </Button>
  </div>
  {#if description}
    <p class="text-[10.5px] leading-relaxed text-muted-foreground">{description}</p>
  {/if}

  {#if rows.length}
    <div class="overflow-hidden rounded-md border">
      <div
        class="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_28px] gap-2 border-b bg-muted/50 px-2 py-1 text-[10px] font-medium uppercase tracking-wide text-muted-foreground"
      >
        <span>Name</span>
        <span>Value</span>
        <span></span>
      </div>
      {#each rows as row, index (index)}
        <div
          class="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_28px] items-center gap-2 border-b px-2 py-1 last:border-b-0"
        >
          <Input
            class="h-7 font-mono text-[11px]"
            bind:value={row.name}
            placeholder="X-Custom-Header"
            aria-label={`${label} 名称 ${index + 1}`}
          />
          <Input
            class="h-7 font-mono text-[11px]"
            bind:value={row.value}
            placeholder="value"
            aria-label={`${label} 值 ${index + 1}`}
          />
          <Button
            variant="ghost"
            size="icon-xs"
            type="button"
            class="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
            onclick={() => removeRow(index)}
            aria-label="删除该 Header"
          >
            <XIcon />
          </Button>
        </div>
      {/each}
    </div>
  {:else}
    <div class="rounded-md border border-dashed px-2 py-3 text-center text-[11px] text-muted-foreground">
      暂无 Header
    </div>
  {/if}
  {#if hint}
    <p class="text-[10.5px] text-muted-foreground">{hint}</p>
  {/if}
</div>
