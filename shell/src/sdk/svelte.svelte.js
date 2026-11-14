// The SDK for a shell written in Svelte 5, over the framework-free SDK (shell.js,
// regions.js). state is a $state copy of the SDK's that follows it, so markup and $derived
// that read it are drawn again when Go changes it; give it to the SDK's helpers,
// activeTab(state) and the rest, and what they read is followed too. The region markers
// are actions: use:shell, use:drag, use:page={tab.id}.

import { tick } from 'svelte';
import * as sdk from './shell.js';
import * as regions from './regions.js';

export { activeTab, controlSpace, focusPage, keyLabel, saveData, saveProfileData, send, setFrame, setPref, tabLabel, workspaceOf } from './shell.js';

export const state = $state({ ...sdk.state });
sdk.onChange((fields) => Object.assign(state, fields));

// connect, with Svelte's tick as the wait for the view to draw.
export const connect = (handlers) => sdk.connect(handlers, (fn) => tick().then(fn));

export function shell(el) {
  regions.shell.mark(el);
  return { destroy: () => regions.shell.unmark(el) };
}

export function drag(el) {
  regions.drag.mark(el);
  return { destroy: () => regions.drag.unmark(el) };
}

export function page(el, tab) {
  regions.page.mark(el, tab);
  return { update: (next) => regions.page.update(el, next), destroy: () => regions.page.unmark(el) };
}
