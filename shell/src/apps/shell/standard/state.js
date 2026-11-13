// What only the standard shell keeps: whether the sidebar is open, and where the tabs go.
// Everything else is Go's, in the kit's state.

import { computed, reactive } from 'vue';
import { state } from '@/kit/shell.js';
import { phone } from '@/phone.js';

export { phone };

export const view = reactive({
  // Whether the sidebar is open. On a phone the sidebar is the drawer, closed until asked
  // for.
  sidebar: !phone,
});

// tabsOnTop says the tabs are a strip above the toolbar rather than a sidebar.
export const tabsOnTop = computed(() => !phone && state.tabLayout === 'horizontal');

// The address bar's answers to what Go sends it, set once it is mounted.
export const addressBar = {
  suggestions() {},
  focus() {},
};
