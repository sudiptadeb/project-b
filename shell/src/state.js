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
  // Whether the sidebar is open. The shell's own, so it stays here.
  sidebar: true,
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
          state.controls = m.controls;
          state.tabLayout = m.tabLayout;
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
        case 'controls':
          state.controls = m.controls;
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
