import { keyLabel, state } from '@/sdk/svelte.svelte.js';

// A control's tooltip: its name, and its key when it has one.
export function hint(name, action) {
  const key = keyLabel(state, action);
  return key ? `${name} (${key})` : name;
}
