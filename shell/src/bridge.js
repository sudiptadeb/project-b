// How a page of ours talks to the managers. In the real app that's chrome.send and a
// global the main process calls. Under the mock it's HTTP. Nothing else knows. The shell
// and the settings page each have a channel: 'shell' or 'settings'.

const real = typeof window.chrome?.send === 'function';

export function bridge(channel) {
  let sending = Promise.resolve();
  return {
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
}

const shell = bridge('shell');
export const send = shell.send;
export const listen = shell.listen;
