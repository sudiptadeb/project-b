import { shown } from '@/keys.js';
import { state } from '@/kit/shell.js';

// keyOf is the key an action shows, as the menus show it, or '' when it has none.
export function keyOf(action) {
  const keys = state.keys[action];
  return keys ? shown(keys, state.platform) : '';
}

// withKey is a control's tooltip: its name, and the key of the action it does.
export function withKey(label, action) {
  const key = keyOf(action);
  return key ? `${label} (${key})` : label;
}
