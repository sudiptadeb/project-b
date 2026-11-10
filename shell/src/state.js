// The shell's copy of the state. The managers own the truth: this is only ever replaced
// by what they send, and a new shell starts from a snapshot.

import { computed, reactive } from 'vue';
import { listen, send } from './bridge.js';

const emptyTab = { id: 0, url: '', title: '', loading: false, blocked: false, crashed: false, error: '' };

export const state = reactive({
  engine: '',
  tabs: [],
  active: 0,
  // Where the tabs go: 'vertical' (a sidebar) or 'horizontal' (a strip on top).
  tabLayout: 'vertical',
  // Where the native window controls are. Zero size when there are none.
  controls: { x: 0, y: 0, w: 0, h: 0 },
  // The radius the OS rounds the window's corners with, in CSS pixels. 0 in full screen.
  cornerRadius: 0,
  // The frame the window paints natively; the shell paints its own bands with the same
  // colour and width.
  windowColor: '',
  frameWidth: 0,
  // Whether the sidebar is open, and whether the settings panel is. The shell's own, so
  // they stay here.
  sidebar: true,
  settings: false,
  // Whether the app checks for updates on its own.
  autoUpdate: true,
  // What the updater is doing: phase '' (nothing), 'downloading', 'ready' or 'failed',
  // with the release's version and, when it failed, why; and how a check the user asked
  // for went.
  update: { phase: '' },
  // The version running, '' for a development build.
  running: '',
});

// activeTab is the tab whose page is showing.
export const activeTab = computed(() => state.tabs.find((t) => t.id === state.active) || emptyTab);

// tabLabel is what a tab's row or strip entry says: the title, else the host while the
// page is on its way, else New Tab.
export function tabLabel(tab) {
  if (tab.title) return tab.title;
  if (tab.url) {
    try {
      return new URL(tab.url).host || tab.url;
    } catch {
      return tab.url;
    }
  }
  return 'New Tab';
}

// How much room each end of the top row leaves: the native window controls' space on
// whichever side they are, counted from inside the 4px window frame, and 8px, the same
// gap the controls get, on an end without them.
export const controlSpace = computed(() => {
  const frame = 4;
  const c = state.controls;
  if (c.w <= 0) return { left: '8px', right: '8px' };
  const onLeft = c.x + c.w / 2 < window.innerWidth / 2;
  return onLeft
    ? { left: c.x + c.w + 8 - frame + 'px', right: '8px' }
    : { left: '8px', right: window.innerWidth - c.x + 8 - frame + 'px' };
});

let onSuggestions = () => {};
let onFocusAddressBar = () => {};

export function setHandlers(handlers) {
  onSuggestions = handlers.suggestions;
  onFocusAddressBar = handlers.focusAddressBar;
}

export function connect() {
  listen(
    (m) => {
      switch (m.t) {
        case 'snapshot':
          state.engine = m.engine;
          state.tabs = m.tabs;
          state.active = m.active;
          state.windowColor = m.windowColor;
          state.frameWidth = m.frameWidth;
          state.controls = m.controls;
          state.cornerRadius = m.cornerRadius;
          state.tabLayout = m.tabLayout;
          state.autoUpdate = m.autoUpdate;
          state.update = m.update;
          state.running = m.running;
          break;
        case 'update':
          state.update = m.update;
          break;
        case 'tabs':
          state.tabs = m.tabs;
          state.active = m.active;
          break;
        case 'tab': {
          const i = state.tabs.findIndex((t) => t.id === m.tab.id);
          if (i >= 0) state.tabs[i] = m.tab;
          break;
        }
        case 'tabLayout':
          state.tabLayout = m.tabLayout;
          break;
        case 'autoUpdate':
          state.autoUpdate = m.autoUpdate;
          break;
        case 'windowColor':
          state.windowColor = m.windowColor;
          break;
        case 'controls':
          state.controls = m.controls;
          state.cornerRadius = m.cornerRadius;
          break;
        case 'suggestions':
          onSuggestions(m.id, m.items || []);
          break;
        case 'focusAddressBar':
          onFocusAddressBar();
          break;
      }
    },
    () => send({ t: 'ready' }),
  );
}
