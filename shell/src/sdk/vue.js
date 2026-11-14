// The SDK for a shell written in Vue: the names a Vue shell uses, over the framework-free
// SDK (shell.js, regions.js). state is a reactive copy of the SDK's that follows it, so
// whatever reads it is drawn again when Go changes it, and the helpers read that copy.
// The region markers are directives: createApp(App).directive('shell', vShell)
// .directive('drag', vDrag).directive('page', vPage).

import { computed, nextTick, reactive } from 'vue';
import * as sdk from './shell.js';
import { drag, page, shell } from './regions.js';

export { focusPage, saveData, saveProfileData, send, setFrame, setPref, tabLabel } from './shell.js';

export const state = reactive({ ...sdk.state });
sdk.onChange((fields) => Object.assign(state, fields));

export const activeTab = computed(() => sdk.activeTab(state));
export const controlSpace = computed(() => sdk.controlSpace(state));
export const workspaceOf = (tab) => sdk.workspaceOf(state, tab);
export const keyLabel = (action) => sdk.keyLabel(state, action);

// connect, with Vue's nextTick as the wait for the view to draw.
export const connect = (handlers) => sdk.connect(handlers, nextTick);

// v-shell, v-drag and v-page="tab.id".
export const vShell = { mounted: shell.mark, unmounted: shell.unmark };
export const vDrag = { mounted: drag.mark, unmounted: drag.unmark };
export const vPage = {
  mounted: (el, { value }) => page.mark(el, value),
  updated: (el, { value }) => page.update(el, value),
  unmounted: page.unmark,
};
