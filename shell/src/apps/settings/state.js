// The settings page's copy of what the managers own. It is only ever replaced by what
// they send, and the page starts from a snapshot.

import { reactive } from 'vue';
import { bridge } from '@/bridge.js';

const channel = bridge('settings');
export const send = channel.send;

export const state = reactive({
  ready: false,
  windowColor: '',
  frameWidth: 0,
  controls: { x: 0, y: 0, w: 0, h: 0 },
  tabLayout: 'vertical',
  autoUpdate: true,
  update: { phase: '' },
  running: '',
  keyboard: { platform: 'mac', templates: [], template: '', following: true, groups: [], missing: [], gone: [], problems: [] },
});

export function connect() {
  channel.listen(
    (m) => {
      switch (m.t) {
        case 'snapshot':
          Object.assign(state, {
            windowColor: m.windowColor,
            frameWidth: m.frameWidth,
            controls: m.controls,
            tabLayout: m.tabLayout,
            autoUpdate: m.autoUpdate,
            update: m.update,
            running: m.running,
            keyboard: m.keyboard,
            ready: true,
          });
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
        case 'keyboard':
          state.keyboard = m.keyboard;
          break;
      }
    },
    () => send({ t: 'ready' }),
  );
}
