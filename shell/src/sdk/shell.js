// The SDK every shell may build on: its copy of Go's state, the channel to Go, and the
// helpers every shell needs to draw tabs. It has no framework in it: the state is a
// plain object, and onChange says what changed, so a view in any framework, or in none,
// follows it (docs/shells.md). The managers own the truth: the state is only ever
// replaced by what they send, and a shell starts from a snapshot.

import { shown } from '@/keys.js';
import { paintTheme } from '@/theme.js';
import { channel, send } from './channel.js';
import { frame, measureNow, reportLayout } from './regions.js';

export { send };
export { setFrame } from './regions.js';

const { mark, end, painted, firstPaint, inputs } = channel.timeline;

mark('shell.script');

const emptyTab = { id: 0, url: '', title: '', loading: false, blocked: false, crashed: false, error: '' };

// state is everything Go sends, as it last sent it. Each field is replaced whole when it
// changes, never changed in place: a tab's new state comes as a new tabs list.
export const state = {
  // This window's tabs in order, and the active one's id.
  tabs: [],
  active: 0,
  // This window's tab groups, each {id, name, color, collapsed, tabs}, in the order made;
  // a tab's group is its id (docs/groups.md).
  groups: [],
  // Where the standard shell puts the tabs: 'vertical' (a sidebar) or 'horizontal' (a
  // strip on top).
  tabLayout: 'vertical',
  // Where the Mobile shell puts its address bar: 'bottom' or 'top'.
  addressBarPosition: 'bottom',
  // 'system', 'light' or 'dark', as Go's theme pref says.
  theme: 'system',
  // Where the native window controls are. Zero size when there are none.
  controls: { x: 0, y: 0, w: 0, h: 0 },
  // The radius the OS rounds the window's corners with, in CSS pixels. 0 in full screen.
  cornerRadius: 0,
  // The frame colour the window paints natively, and a band of it around its edge when
  // the shell asks for one (setFrame); a shell paints its own strips with the same colour.
  windowColor: '',
  // Every colour a shell paints with, made by Go from the frame colour, light and dark:
  // {light, dark}, each the design system's tokens by name ('--accent' and so on). It comes
  // with windowColor, always together; null before the snapshot, when nothing is painted.
  tones: null,
  // What the updater is doing: phase '' (nothing), 'downloading', 'ready' or 'failed',
  // with the release's version and, when it failed, why; and how a check the user asked
  // for went.
  update: { phase: '' },
  // The version running, '' for a development build.
  running: '',
  // The platform, as the keymap names it, and the key each action shows, by action id.
  platform: 'mac',
  keys: {},
  // The profile's workspaces, each with its id, name and colour, and the tones of its
  // colour as state.tones has the frame's, in the user's order: one at least once the
  // snapshot is in. Main, the profile's own storage, has the id ''.
  workspaces: [],
  // The report of the last run's crash, '' when it didn't crash or the notice was closed.
  crashLog: '',
  // macOS's report of that crash, or the folder it keeps its reports in.
  crashReport: '',
  // The extensions' buttons, null while the Extensions flag is off: buttons, each {id, name,
  // title, icon, badge, badgeColor, badgeTextColor, enabled, pinned}, in the order the
  // menu lists them; popup, the id of the extension whose popup is open, or ''; and prompt,
  // an install or a removal waiting for the user's answer, {id, name, version, icon,
  // permissions, remove}, or null.
  extensions: null,
  // The shell's own data, as it last saved it for this window, or null: what it needs to
  // lay itself out again after a reload. It comes only in the snapshot.
  data: null,
  // What the shell saved for the whole profile with saveProfileData, or null: kept on disk,
  // the same in every window, and there again after a restart.
  profileData: null,
};

// The messages that are only state: each sets the fields it carries.
const updates = new Set(['tabs', 'theme', 'windowColor', 'controls', 'keys', 'workspaces', 'update', 'crashLog', 'tabLayout', 'addressBarPosition', 'extensions', 'profileData']);

const listeners = [];

// onChange calls fn after each change to state, with the fields that changed and their
// new values: on each message from Go that carries state, and on saveData.
export function onChange(fn) {
  listeners.push(fn);
}

function change(fields) {
  Object.assign(state, fields);
  for (const fn of listeners) fn(fields);
}

// The root's colours follow the frame colour and the theme.
onChange((fields) => {
  if ('tones' in fields || 'theme' in fields) paintTheme(state.tones, state.theme);
});

// While an extension's popup is open, a press anywhere in the shell closes it. The one
// exception is the popup's own button, which the shell marks data-extension="<id>": that
// press goes to Go as any press on it does, and Go closes the popup it finds open. The
// engine closes it for a press outside the shell.
function closePopup(e) {
  const own = e.target.closest('[data-extension]');
  if (!own || own.dataset.extension !== state.extensions.popup) send({ t: 'extensionClosePopup' });
}
onChange((fields) => {
  if (!('extensions' in fields)) return;
  if (state.extensions?.popup) addEventListener('pointerdown', closePopup, true);
  else removeEventListener('pointerdown', closePopup, true);
});

// saveData keeps value, any JSON, as this window's copy of the shell's own data, in place
// of what it was. Go keeps it and never reads it.
export function saveData(value) {
  change({ data: value });
  send({ t: 'data', data: value });
}

// saveProfileData keeps value, any JSON, as what this shell saves for the whole profile,
// in place of what it was. Go writes it to the profile and sends it to the shell in every
// window, this one too, as profileData; it never reads it.
export function saveProfileData(value) {
  change({ profileData: value });
  send({ t: 'profileData', data: value });
}

// How the shell's view waits until it has drawn what just changed: what connect was given.
let afterDraw;

// focusPage gives the keyboard to the active tab's page. The layout goes first, so the
// window already shows the tab when it gets the keyboard: once the view has drawn the
// change that asked for it, as a page marker learns the tab it shows only then.
export function focusPage() {
  afterDraw(() => {
    measureNow();
    send({ t: 'focusTab', id: state.active });
  });
}

// The helpers below each read the state they are given: the SDK's, or a framework's
// reactive copy of it, which then knows what was read.

// activeTab is the tab the address bar, back, forward and reload act on.
export function activeTab(s) {
  return s.tabs.find((t) => t.id === s.active) || emptyTab;
}

// workspaceOf is the workspace a tab is in, for a shell to show it by: undefined while the
// profile has one workspace only, as every tab is then in it, and for one the list doesn't
// hold.
export function workspaceOf(s, tab) {
  if (s.workspaces.length < 2) return undefined;
  return s.workspaces.find((w) => w.id === tab.workspace);
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

// keyLabel is the key an action shows, as the menus show it, or '' when it has none.
export function keyLabel(s, action) {
  const keys = s.keys[action];
  return keys ? shown(keys, s.platform) : '';
}

// How much room each end of the top row leaves: the native window controls' space on
// whichever side they are, counted from inside the band the shell asked for, and 8px, the
// same gap the controls get, on an end without them.
export function controlSpace(s) {
  const c = s.controls;
  if (c.w <= 0) return { left: '8px', right: '8px' };
  const onLeft = c.x + c.w / 2 < window.innerWidth / 2;
  return onLeft
    ? { left: c.x + c.w + 8 - frame + 'px', right: '8px' }
    : { left: '8px', right: window.innerWidth - c.x + 8 - frame + 'px' };
}

// connect says ready, and from then on keeps state as Go sends it. handlers are the
// shell's own, by message kind, each called with the message after state has it and
// onChange has heard of it: suggestions, focusAddressBar, or any other. Once the snapshot
// is drawn it reports the layout and says shown. afterDraw(fn) calls fn once the view
// has drawn what changed: Vue's nextTick, Svelte's tick. Left out, the view is taken to
// draw as it hears of a change, as a shell with no framework does in its onChange. Call
// it once, as the shell's view is made.
export function connect(handlers = {}, drawn = (fn) => fn()) {
  afterDraw = drawn;
  mark('shell.mounted');
  firstPaint();
  inputs();
  channel.listen(
    (m) => {
      switch (m.t) {
        case 'snapshot': {
          const { t, ...all } = m;
          change(all);
          mark('shell.snapshot');
          break;
        }
        // Go asks for a span to end once what it just sent is drawn.
        case 'paint':
          painted(() => end(m.span));
          break;
        case 'tab':
          change({ tabs: state.tabs.map((t) => (t.id === m.tab.id ? m.tab : t)) });
          break;
        default:
          if (updates.has(m.t)) {
            const { t, ...fields } = m;
            change(fields);
          }
      }
      handlers[m.t]?.(m);
      // Once the page shows it, and Go has its regions to route clicks, Go marks the app
      // ready.
      if (m.t === 'snapshot') {
        afterDraw(() => {
          reportLayout();
          send({ t: 'shown' });
        });
      }
    },
    () => send({ t: 'ready' }),
  );
}
