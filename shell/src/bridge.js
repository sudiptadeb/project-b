// How a page of ours talks to the managers. In the real app that's chrome.send and a
// global the main process calls. Under the mock it's HTTP. Nothing else knows. The shell
// and the settings page each have a channel: 'shell' or 'settings'.

import { timeline } from '@/timeline.js';

const real = typeof window.chrome?.send === 'function';

export function bridge(channel) {
  let sending = Promise.resolve();
  const b = {
    // send delivers one command, in order.
    send(msg) {
      const body = JSON.stringify(msg);
      if (real) {
        window.chrome.send(channel, [body]);
        return;
      }
      sending = sending
        .then(() => fetch(`/${channel}/send`, { method: 'POST', body }))
        .catch((err) => console.error('send failed', err));
    },
    // listen calls onMessage for each message and onOpen once we can talk.
    listen(onMessage, onOpen) {
      if (real) {
        window.__shellReceive = (json) => onMessage(JSON.parse(json));
        onOpen();
        return;
      }
      const events = new EventSource(`/${channel}/events`);
      events.onmessage = (e) => onMessage(JSON.parse(e.data));
      events.onopen = onOpen;
    },
  };
  // The page's entries on the app's timeline go the same way.
  b.timeline = timeline(channel, b.send);
  return b;
}

const shell = bridge('shell');
export const listen = shell.listen;
export const { mark, end, painted, firstPaint } = shell.timeline;

// The commands that start an interaction the timeline measures, and its span. The span
// begins here, at the user's click, and Go has the shell end it once the result is drawn.
const spans = { newTab: 'tab.new', closeTab: 'tab.close', setTabLayout: 'tabs.layout' };

export function send(msg) {
  const span = msg.t === 'run' && msg.text === 'app.settings' ? 'settings.open' : spans[msg.t];
  if (span) shell.timeline.begin(span);
  shell.send(msg);
}
