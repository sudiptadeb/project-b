// The toolbar: Back, Forward, Reload (Stop while the page loads), the address filling the
// rest, the extensions, Relaunch to update while a release is ready, and Settings.

import { activeTab, focusPage, send, state } from '@/sdk/shell.js';
import { field, address } from './address.js';
import { h, icon, set, svg } from './dom.js';

// A press on Back, Forward, Reload or Stop leaves the keyboard with the page.
function act(msg) {
  send(msg);
  focusPage();
}

const button = (label, onclick, ...children) => h('button', { class: 'btn', type: 'button', 'aria-label': label, title: label, onclick }, ...children);

const back = button('Back', () => act({ t: 'back' }), icon(14, 16, ['M10 3 5 8l5 5']));
const forward = button('Forward', () => act({ t: 'forward' }), icon(14, 16, ['m6 3 5 5-5 5']));
const reloadIcon = icon(14, 16, ['M13 8a5 5 0 1 1-1.5-3.5M13 2.5V5h-2.5']);
const stopIcon = icon(14, 16, ['M4 4l8 8M12 4l-8 8']);
const reload = button('Reload', () => act(activeTab(state).loading ? { t: 'run', text: 'nav.stop' } : { t: 'reload' }), reloadIcon);

const relaunch = h('button', { class: 'primary', type: 'button', onclick: () => send({ t: 'relaunch' }) }, 'Relaunch to update');

// The gear, on a 24-unit square at a 2-unit stroke.
const gear = svg(
  'svg',
  { width: 14, height: 14, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': 2, 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'aria-hidden': 'true' },
  svg('circle', { cx: 12, cy: 12, r: 3 }),
  svg('path', { d: 'M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z' }),
);

export const settings = button('Settings', () => send({ t: 'run', text: 'app.settings' }), gear);

export const toolbar = h('div', { class: 'toolbar' }, back, forward, reload, field, settings);

// drawNav follows the active tab: what Back and Forward can do, Reload or Stop, and its
// address.
export function drawNav() {
  const tab = activeTab(state);
  set(back, 'disabled', !tab.canGoBack);
  set(forward, 'disabled', !tab.canGoForward);
  const label = tab.loading ? 'Stop' : 'Reload';
  if (reload.title !== label) {
    set(reload, 'aria-label', label);
    set(reload, 'title', label);
    reload.firstChild.replaceWith(tab.loading ? stopIcon : reloadIcon);
  }
  set(reload, 'disabled', !tab.loading && !tab.url);
  address.show();
}

// drawUpdate shows Relaunch to update while a release is ready.
export function drawUpdate() {
  if (state.update.phase === 'ready') {
    set(relaunch, 'title', `Project B ${state.update.version} is ready`);
    settings.before(relaunch);
  } else {
    relaunch.remove();
  }
}
