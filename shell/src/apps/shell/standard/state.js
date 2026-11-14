// What only the standard shell keeps: whether the sidebar is open, and where the tabs go.
// Everything else is Go's, in the SDK's state.

import { computed, reactive } from 'vue';
import { state } from '@/sdk/vue.js';
export const view = reactive({
  // Whether the sidebar is open.
  sidebar: true,
});

// tabsOnTop says the tabs are a strip above the toolbar rather than a sidebar.
export const tabsOnTop = computed(() => state.tabLayout === 'horizontal');

// The address bar's answers to what Go sends it, set once it is mounted.
export const addressBar = {
  suggestions() {},
  focus() {},
};
