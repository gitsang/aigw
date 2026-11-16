import ChartLineIcon from '@lucide/svelte/icons/chart-line';
import LayersIcon from '@lucide/svelte/icons/layers';
import BoxesIcon from '@lucide/svelte/icons/boxes';
import RouteIcon from '@lucide/svelte/icons/route';
import KeyRoundIcon from '@lucide/svelte/icons/key-round';
import SettingsIcon from '@lucide/svelte/icons/settings';

/**
 * Console navigation entries, shared by the sidebar and breadcrumb.
 * `labelKey` is resolved against the active locale by the consumer.
 */
export const navItems = [
  { href: '/logs', labelKey: 'nav.logs', icon: ChartLineIcon },
  { href: '/models', labelKey: 'nav.models', icon: LayersIcon },
  { href: '/providers', labelKey: 'nav.providers', icon: BoxesIcon },
  { href: '/virtual-models', labelKey: 'nav.virtualModels', icon: RouteIcon },
  { href: '/keys', labelKey: 'nav.keys', icon: KeyRoundIcon },
  { href: '/settings', labelKey: 'nav.settings', icon: SettingsIcon }
];

/** Resolve the nav entry that matches the current pathname. */
export function navItemFor(pathname) {
  return navItems.find((item) => pathname === item.href || pathname.startsWith(`${item.href}/`));
}
