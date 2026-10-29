<script>
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { cn } from '$lib/utils.js';

  let { tone = 'neutral', dot = false, class: className, children } = $props();

  const tones = {
    neutral: 'border-transparent bg-muted text-muted-foreground',
    primary: 'border-primary/20 bg-primary/10 text-primary',
    success: 'border-emerald-500/25 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
    warning: 'border-amber-500/25 bg-amber-500/10 text-amber-700 dark:text-amber-300',
    danger: 'border-destructive/25 bg-destructive/10 text-destructive',
    info: 'border-sky-500/25 bg-sky-500/10 text-sky-700 dark:text-sky-300',
    violet: 'border-violet-500/25 bg-violet-500/10 text-violet-700 dark:text-violet-300'
  };

  const dots = {
    neutral: 'bg-muted-foreground/60',
    primary: 'bg-primary',
    success: 'bg-emerald-500',
    warning: 'bg-amber-500',
    danger: 'bg-destructive',
    info: 'bg-sky-500',
    violet: 'bg-violet-500'
  };
</script>

<Badge variant="outline" class={cn(tones[tone] ?? tones.neutral, className)}>
  {#if dot}<span class={cn('size-1.5 rounded-full', dots[tone] ?? dots.neutral)}></span>{/if}
  {@render children?.()}
</Badge>
