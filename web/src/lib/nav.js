import ChartLineIcon from '@lucide/svelte/icons/chart-line';
import BoxesIcon from '@lucide/svelte/icons/boxes';
import RouteIcon from '@lucide/svelte/icons/route';
import KeyRoundIcon from '@lucide/svelte/icons/key-round';

/** Console navigation entries, shared by the sidebar and breadcrumb. */
export const navItems = [
  { href: '/logs', label: '日志与统计', icon: ChartLineIcon },
  { href: '/providers', label: 'Model Provider', icon: BoxesIcon },
  { href: '/routing', label: 'Routing', icon: RouteIcon },
  { href: '/keys', label: 'API Keys', icon: KeyRoundIcon }
];

/** Resolve the nav entry that matches the current pathname. */
export function navItemFor(pathname) {
  return navItems.find((item) => pathname === item.href || pathname.startsWith(`${item.href}/`));
}
