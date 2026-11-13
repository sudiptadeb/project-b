// The one channel every shell speaks on, 'shell'. The extensions' toolbar sends on it too,
// so this is all it brings with it into the settings page's build, which shares @extensions:
// nothing here sends until asked.

import { bridge } from '@/bridge.js';

export const channel = bridge('shell');

// The commands that start an interaction the timeline measures, and its span. The span
// begins here, at the user's click, and Go has the shell end it once the result is drawn.
const spans = { newTab: 'tab.new', closeTab: 'tab.close', activateTab: 'tab.activate', setTabLayout: 'tabs.layout' };

// send delivers one command to Go, in order.
export function send(msg) {
  const span = msg.t === 'run' && msg.text === 'app.settings' ? 'settings.open' : spans[msg.t];
  if (span) channel.timeline.begin(span);
  channel.send(msg);
}
