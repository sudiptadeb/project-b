// What every shell is built on: its copy of Go's state, the channel to Go, and the helpers
// every shell needs to draw tabs. The managers own the truth: the state is only ever
// replaced by what they send, and a shell starts from a snapshot (docs/shells.md).

import { computed, nextTick, reactive } from 'vue';
import { extensionsMessage } from '@extensions';
import { channel, send } from './channel.js';
import { measureNow, reportLayout } from './regions.js';

export { send };

const { mark, end, painted, firstPaint, inputs } = channel.timeline;

mark('shell.script');

const emptyTab = { id: 0, url: '', title: '', loading: false, blocked: false, crashed: false, error: '' };

// state is everything Go sends, as it last sent it.
export const state = reactive({
  // This window's tabs in order, and the active one's id.
  tabs: [],
  active: 0,
  // Where the standard shell puts the tabs: 'vertical' (a sidebar) or 'horizontal' (a
  // strip on top).
  tabLayout: 'vertical',
  // 'system', 'light' or 'dark', as Go's theme pref says.
  theme: 'system',
  // Where the native window controls are. Zero size when there are none.
  controls: { x: 0, y: 0, w: 0, h: 0 },
  // The radius the OS rounds the window's corners with, in CSS pixels. 0 in full screen.
  cornerRadius: 0,
  // The frame the window paints natively; a shell paints its own bands with the same
  // colour and width.
  windowColor: '',
  frameWidth: 0,
  // What the updater is doing: phase '' (nothing), 'downloading', 'ready' or 'failed',
  // with the release's version and, when it failed, why; and how a check the user asked
  // for went.
  update: { phase: '' },
  // The version running, '' for a development build.
  running: '',
  // The platform, as the keymap names it, and the key each action shows, by action id.
  platform: 'mac',
  keys: {},
  // The profile's workspaces, each with its id, name and colour; the default workspace
  // isn't among them.
  workspaces: [],
  // The report of the last run's crash, '' when it didn't crash or the notice was closed.
  crashLog: '',
  // macOS's report of that crash, or the folder it keeps its reports in.
  crashReport: '',
});

// The messages that are only state: each sets the fields it carries.
const updates = new Set(['tabs', 'theme', 'windowColor', 'controls', 'keys', 'workspaces', 'update', 'crashLog', 'tabLayout']);

// activeTab is the tab the address bar, back, forward and reload act on.
export const activeTab = computed(() => state.tabs.find((t) => t.id === state.active) || emptyTab);

// focusPage gives the keyboard to the active tab's page. The layout goes first, so the
// window already shows the tab when it gets the keyboard: once Vue has drawn the change
// that asked for it, as a v-page learns the tab it shows only then. A watcher on the
// active tab runs before that.
export function focusPage() {
  nextTick(() => {
    measureNow();
    send({ t: 'focusTab', id: state.active });
  });
}

// workspaceOf is the workspace a tab is in, or undefined for the default one.
export function workspaceOf(tab) {
  return state.workspaces.find((w) => w.id === tab.workspace);
}

// tabLabel is what a tab's entry says: the title, else the host while the page is on its
// way, else New Tab.
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

// connect says ready, and from then on keeps state as Go sends it. Once the snapshot is
// drawn it reports the layout and says shown. handlers are the shell's own, by message
// kind, each called with the message after state has it: suggestions, focusAddressBar,
// or any other. Call it once, as the shell's app is made.
export function connect(handlers = {}) {
  mark('shell.mounted');
  firstPaint();
  inputs();
  channel.listen(
    (m) => {
      extensionsMessage(m);
      switch (m.t) {
        case 'snapshot': {
          const { t, ...all } = m;
          Object.assign(state, all);
          mark('shell.snapshot');
          // Once the page shows it, and Go has its regions to route clicks, Go marks the
          // app ready.
          nextTick(() => {
            reportLayout();
            send({ t: 'shown' });
          });
          break;
        }
        // Go asks for a span to end once what it just sent is drawn.
        case 'paint':
          painted(() => end(m.span));
          break;
        case 'tab': {
          const i = state.tabs.findIndex((t) => t.id === m.tab.id);
          if (i >= 0) state.tabs[i] = m.tab;
          break;
        }
        default:
          if (updates.has(m.t)) {
            const { t, ...fields } = m;
            Object.assign(state, fields);
          }
      }
      handlers[m.t]?.(m);
    },
    () => send({ t: 'ready' }),
  );
}
