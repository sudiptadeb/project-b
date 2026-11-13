// How a page of ours talks to the managers: chrome.send, and a global the engine calls
// with each message. Nothing else knows. Each page has a channel: 'shell' for every
// shell (through @/sdk/shell.js), 'settings' or 'picker'.

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
