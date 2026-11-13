// The shell's copy of the state. The managers own the truth: this is only ever replaced
// by what they send, and a new shell starts from a snapshot.

import { computed, nextTick, reactive } from 'vue';
import { end, listen, mark, painted, send } from '@/bridge.js';
import { extensionsMessage } from '@extensions';

import { phone } from '@/phone.js';
import { measureNow } from './regions.js';

export { phone };

const emptyTab = { id: 0, url: '', title: '', loading: false, blocked: false, crashed: false, error: '' };

export const state = reactive({
  tabs: [],
  active: 0,
  // Where the tabs go: 'vertical' (a sidebar) or 'horizontal' (a strip on top).
  tabLayout: 'vertical',
  // 'system', 'light' or 'dark', as Go's theme pref says.
  theme: 'system',
  // Where the native window controls are. Zero size when there are none.
  controls: { x: 0, y: 0, w: 0, h: 0 },
  // The radius the OS rounds the window's corners with, in CSS pixels. 0 in full screen.
  cornerRadius: 0,
  // The frame the window paints natively; the shell paints its own bands with the same
  // colour and width.
  windowColor: '',
  frameWidth: 0,
  // Whether the sidebar is open. The shell's own, so it stays here. On a phone the
  // sidebar is the drawer, closed until asked for.
  sidebar: !phone,
  // What the updater is doing: phase '' (nothing), 'downloading', 'ready' or 'failed',
  // with the release's version and, when it failed, why; and how a check the user asked
  // for went.
  update: { phase: '' },
  // The version running, '' for a development build.
  running: '',
  // The platform, as the keymap names it, and the key each action shows, by action id.
  platform: 'mac',
  keys: {},
  // The report of the last run's crash, '' when it didn't crash or the notice was closed.
  crashLog: '',
  // macOS's report of that crash, or the folder it keeps its reports in.
  crashReport: '',
});

// tabsOnTop says the tabs are a strip above the toolbar rather than a sidebar.
export const tabsOnTop = computed(() => !phone && state.tabLayout === 'horizontal');

// activeTab is the tab whose page is showing.
export const activeTab = computed(() => state.tabs.find((t) => t.id === state.active) || emptyTab);

// focusPage gives the keyboard to the active tab's page. The layout goes first, so the
// window already shows the tab when it gets the keyboard.
export function focusPage() {
  measureNow();
  send({ t: 'focusTab', id: state.active });
}

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
      extensionsMessage(m);
      switch (m.t) {
        case 'snapshot':
          state.tabs = m.tabs;
          state.active = m.active;
          state.windowColor = m.windowColor;
          state.frameWidth = m.frameWidth;
          state.controls = m.controls;
          state.cornerRadius = m.cornerRadius;
          state.tabLayout = m.tabLayout;
          state.theme = m.theme;
          state.update = m.update;
          state.running = m.running;
          state.crashLog = m.crashLog;
          state.crashReport = m.crashReport;
          state.platform = m.platform;
          state.keys = m.keys;
          mark('shell.snapshot');
          // Once the page shows it, Go marks the app ready.
          nextTick(() => send({ t: 'shown' }));
          break;
        // Go asks for a span to end once what it just sent is drawn.
        case 'paint':
          painted(() => end(m.span));
          break;
        case 'keys':
          state.keys = m.keys;
          break;
        case 'crashLog':
          state.crashLog = m.crashLog;
          state.crashReport = m.crashReport;
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
        case 'theme':
          state.theme = m.theme;
          break;
        case 'tabLayout':
          state.tabLayout = m.tabLayout;
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
