// How our pages, the shell and the settings page, talk to the managers. In the real app
// that's chrome.send and a global the main process calls; the engine tags each message
// with the page's WebContents. Under the mock it's HTTP, relative to the page, so the dev
// server can tell the shell at / from the settings page in a tab at /tab/<id>/. Nothing
// else in either page knows.

const real = typeof window.chrome?.send === 'function';

let sending = Promise.resolve();

// send delivers one command, in order.
export function send(msg) {
  const body = JSON.stringify(msg);
  if (real) {
    window.chrome.send('shell', [body]);
    return;
  }
  sending = sending
    .then(() => fetch('shell/send', { method: 'POST', body }))
    .catch((err) => console.error('send failed', err));
}

// listen calls onMessage for each message and onOpen once we can talk.
export function listen(onMessage, onOpen) {
  if (real) {
    window.__shellReceive = (json) => onMessage(JSON.parse(json));
    onOpen();
    return;
  }
  const events = new EventSource('shell/events');
  events.onmessage = (e) => onMessage(JSON.parse(e.data));
  events.onopen = onOpen;
}
