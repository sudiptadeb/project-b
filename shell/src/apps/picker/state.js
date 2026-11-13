// The picker page's copy of what Go sends: the profiles in the list and the templates a
// new one can be made from. It is only ever replaced by what Go sends.

import { nextTick, reactive } from 'vue';
import { bridge } from '@/bridge.js';

const channel = bridge('picker');
export const send = channel.send;
export const { mark, inputs } = channel.timeline;

export const state = reactive({
  ready: false,
  // The app's own frame colour: the picker belongs to no profile.
  windowColor: '',
  theme: 'system',
  profiles: [],
  templates: [],
});

export function connect() {
  channel.listen(
    (m) => {
      if (m.t === 'state') {
        Object.assign(state, { windowColor: m.color, profiles: m.profiles, templates: m.templates, ready: true });
        mark('picker.state');
        // Once the page shows it, Go marks the app ready.
        nextTick(() => send({ t: 'shown' }));
      }
    },
    () => send({ t: 'ready' }),
  );
}

// The window's layout, as the shell reports its own: the top band drags the window, and
// everything under it is the page's.
const band = 40;
function report() {
  const w = window.innerWidth;
  const h = window.innerHeight;
  send({ t: 'layout', shell: [{ x: 0, y: band, w, h: h - band }], drag: [{ x: 0, y: 0, w, h: band }], tabs: [], viewW: w, viewH: h });
}
window.addEventListener('resize', report);
report();
