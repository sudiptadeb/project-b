// How a page of ours talks to the managers: chrome.send, and a global the engine calls
// with each message. Nothing else knows. The shell and the settings page each have a
// channel: 'shell' or 'settings'.

import { timeline } from '@/timeline.js';

export function bridge(channel) {
  const b = {
    // send delivers one command, in order.
    send(msg) {
      window.chrome.send(channel, [JSON.stringify(msg)]);
    },
    // listen calls onMessage for each message and onOpen once we can talk.
    listen(onMessage, onOpen) {
      window.__shellReceive = (json) => onMessage(JSON.parse(json));
      onOpen();
    },
  };
  // The page's entries on the app's timeline go the same way.
  b.timeline = timeline(channel, b.send);
  // inputs ends the span the window begins for each click, "input.click", as the click
  // reaches the page. The page's main script calls it once.
  b.timeline.inputs = () => {
    addEventListener('mouseup', () => b.timeline.end('input.click'), true);
  };
  return b;
}

const shell = bridge('shell');
export const listen = shell.listen;
export const { mark, end, painted, firstPaint, inputs } = shell.timeline;

// The commands that start an interaction the timeline measures, and its span. The span
// begins here, at the user's click, and Go has the shell end it once the result is drawn.
const spans = { newTab: 'tab.new', closeTab: 'tab.close', activateTab: 'tab.activate', setTabLayout: 'tabs.layout' };

export function send(msg) {
  const span = msg.t === 'run' && msg.text === 'app.settings' ? 'settings.open' : spans[msg.t];
  if (span) shell.timeline.begin(span);
  shell.send(msg);
}
