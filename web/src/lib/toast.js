import { writable } from 'svelte/store';

export const toasts = writable([]);
let seq = 0;

export function toast(message, type = 'success') {
  const id = ++seq;
  toasts.update((list) => [...list, { id, message, type }]);
  setTimeout(() => toasts.update((list) => list.filter((t) => t.id !== id)), 4200);
}
