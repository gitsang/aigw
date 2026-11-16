<script>
  import PlusIcon from '@lucide/svelte/icons/plus';
  import XIcon from '@lucide/svelte/icons/x';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Label } from '$lib/components/ui/label/index.js';
  import { t } from '$lib/i18n/index.svelte.js';

  let {
    label = '',
    hint = '',
    description = '',
    rows = $bindable([]),
    onadd = () => {}
  } = $props();

  const fieldLabel = $derived(label || t('app.headerRows.label'));

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
    <Label>{fieldLabel}</Label>
    <Button variant="ghost" size="xs" type="button" onclick={addRow}>
      <PlusIcon />
      {t('app.headerRows.add')}
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
        <span>{t('app.headerRows.nameColumn')}</span>
        <span>{t('app.headerRows.valueColumn')}</span>
        <span></span>
      </div>
      {#each rows as row, index (index)}
        <div
          class="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_28px] items-center gap-2 border-b px-2 py-1 last:border-b-0"
        >
          <Input
            class="h-7 font-mono text-[11px]"
            bind:value={row.name}
            placeholder={t('app.headerRows.namePlaceholder')}
            aria-label={t('app.headerRows.nameAria', { label: fieldLabel, index: index + 1 })}
          />
          <Input
            class="h-7 font-mono text-[11px]"
            bind:value={row.value}
            placeholder={t('app.headerRows.valuePlaceholder')}
            aria-label={t('app.headerRows.valueAria', { label: fieldLabel, index: index + 1 })}
          />
          <Button
            variant="ghost"
            size="icon-xs"
            type="button"
            class="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
            onclick={() => removeRow(index)}
            aria-label={t('app.headerRows.removeAria')}
          >
            <XIcon />
          </Button>
        </div>
      {/each}
    </div>
  {:else}
    <div class="rounded-md border border-dashed px-2 py-3 text-center text-[11px] text-muted-foreground">
      {t('app.headerRows.empty')}
    </div>
  {/if}
  {#if hint}
    <p class="text-[10.5px] text-muted-foreground">{hint}</p>
  {/if}
</div>
