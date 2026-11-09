// How the shell talks to the managers. In the real app that's chrome.send and a global
// the main process calls. Under the mock it's HTTP. Nothing else in the shell knows.

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
    .then(() => fetch('/shell/send', { method: 'POST', body }))
    .catch((err) => console.error('send failed', err));
}

// listen calls onMessage for each message and onOpen once we can talk.
export function listen(onMessage, onOpen) {
  if (real) {
    window.__shellReceive = (json) => onMessage(JSON.parse(json));
    onOpen();
    return;
  }
  const events = new EventSource('/shell/events');
  events.onmessage = (e) => onMessage(JSON.parse(e.data));
  events.onopen = onOpen;
}
