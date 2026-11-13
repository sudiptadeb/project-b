// The settings page's copy of what the managers own. It is only ever replaced by what
// they send, and the page starts from a snapshot.

import { reactive } from 'vue';
import { bridge } from '@/bridge.js';
import { extensionsMessage } from '@extensions';

const channel = bridge('settings');
export const send = channel.send;
export const { mark, end, firstPaint, inputs } = channel.timeline;

export const state = reactive({
  ready: false,
  windowColor: '',
  frameWidth: 0,
  controls: { x: 0, y: 0, w: 0, h: 0 },
  tabLayout: 'vertical',
  // 'system', 'light' or 'dark', as Go's theme pref says.
  theme: 'system',
  autoUpdate: true,
  update: { phase: '' },
  running: '',
  // The section showing: the one Go asks for when it opens the page at one, or the one the
  // user picked. '' is the first.
  section: '',
  keyboard: { platform: 'mac', templates: [], template: '', following: true, groups: [], missing: [], gone: [], problems: [] },
  // Every flag with its saved value, whether its build has it, and whether it waits for a
  // relaunch.
  flags: [],
  // This profile in the list: its name, the template it was made from or last reset to,
  // the templates, the id of the shell it loads, and the shells to choose from, each with
  // its id and name. null in a copy on a folder named outright, which isn't listed.
  profile: null,
  // The profile's workspaces, each with its id, name and colour, in the order they were
  // made.
  workspaces: [],
});

export function connect() {
  channel.listen(
    (m) => {
      extensionsMessage(m);
      switch (m.t) {
        case 'snapshot':
          Object.assign(state, {
            windowColor: m.windowColor,
            frameWidth: m.frameWidth,
            controls: m.controls,
            tabLayout: m.tabLayout,
            theme: m.theme,
            autoUpdate: m.autoUpdate,
            update: m.update,
            running: m.running,
            keyboard: m.keyboard,
            flags: m.flags,
            section: m.section,
            profile: m.profile || null,
            workspaces: m.workspaces,
            ready: true,
          });
          mark('settings.snapshot');
          break;
        case 'theme':
          state.theme = m.theme;
          break;
        case 'tabLayout':
          state.tabLayout = m.tabLayout;
          break;
        case 'autoUpdate':
          state.autoUpdate = m.autoUpdate;
          break;
        case 'update':
          state.update = m.update;
          break;
        case 'windowColor':
          state.windowColor = m.windowColor;
          break;
        case 'controls':
          state.controls = m.controls;
          break;
        case 'section':
          state.section = m.section;
          break;
        case 'keyboard':
          state.keyboard = m.keyboard;
          break;
        case 'flags':
          state.flags = m.flags;
          break;
        case 'profile':
          state.profile = m.profile;
          break;
        case 'workspaces':
          state.workspaces = m.workspaces;
          break;
      }
    },
    () => send({ t: 'ready' }),
  );
}
