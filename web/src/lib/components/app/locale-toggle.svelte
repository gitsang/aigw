<script>
  import LanguagesIcon from '@lucide/svelte/icons/languages';
  import * as Select from '$lib/components/ui/select/index.js';
  import { LOCALES, i18n, setLocale, t } from '$lib/i18n/index.svelte.js';

  let { class: className = '' } = $props();

  // The trigger renders the active language's own name; deriving it from the
  // locale (rather than from the mounted items) keeps it correct before the
  // select is opened for the first time.
  const currentLabel = $derived(t(`common.language.${i18n.locale}`));
</script>

<Select.Root type="single" value={i18n.locale} onValueChange={(value) => setLocale(value)}>
  <Select.Trigger size="sm" class={className} aria-label={t('common.language.label')}>
    <LanguagesIcon />
    <Select.Value placeholder={currentLabel}>{currentLabel}</Select.Value>
  </Select.Trigger>
  <Select.Content>
    {#each LOCALES as item (item.id)}
      <Select.Item value={item.id} label={t(`common.language.${item.id}`)}>
        {t(`common.language.${item.id}`)}
      </Select.Item>
    {/each}
  </Select.Content>
</Select.Root>
