// The dev window: on the mock engine, what the real window draws over the shell in views
// of its own. Go's mock engine says what is open on /mock/window/events, the state on
// connecting and then each change, as {"t":"window","settings":bool,"popup":{...}|null}.
// This draws it in a layer on the html element, above the shell's app and outside it:
//
//   settings  the settings page, in an iframe over the whole window, as the real
//             window's overlay view is. It talks to Go on its own channel, as it does there.
//   popup     an extension's popup under its button. The mock runs no extension pages,
//             so it is a card that names the page; a click outside it closes it, as a
//             click outside the real popup does.
//
// In the real app the engine draws these and this does nothing.

const real = typeof window.chrome?.send === 'function';

export function mockWindow() {
  if (real) return;
  const layer = document.createElement('div');
  layer.className = 'mock-window';
  layer.style.cssText = 'position: fixed; inset: 0; z-index: 2147483647; pointer-events: none;';
  document.documentElement.append(layer);

  let settings = null;
  let popup = null;

  function showSettings(open) {
    if (open && !settings) {
      settings = document.createElement('iframe');
      settings.className = 'mock-settings';
      settings.title = 'Settings';
      settings.src = '/settings/';
      settings.style.cssText = 'position: absolute; inset: 0; width: 100%; height: 100%; border: 0; pointer-events: auto; background: Canvas;';
      // The real overlay takes the keyboard as it opens.
      settings.addEventListener('load', () => settings?.contentWindow?.focus());
      layer.append(settings);
    } else if (!open && settings) {
      settings.remove();
      settings = null;
      window.focus();
    }
  }

  function showPopup(p) {
    popup?.remove();
    popup = null;
    if (!p) return;
    popup = document.createElement('div');
    popup.className = 'mock-popup-backdrop';
    popup.style.cssText = 'position: absolute; inset: 0; pointer-events: auto;';
    const card = document.createElement('div');
    card.className = 'mock-popup';
    card.dataset.extension = p.extension;
    const a = p.anchor;
    card.style.cssText = `position: absolute; top: ${a.y + a.h + 6}px; left: ${Math.max(8, a.x + a.w - 320)}px; width: 320px;`
      + ' padding: 12px 14px; border-radius: 12px; background: Canvas; color: CanvasText;'
      + ' box-shadow: 0 0 0 0.5px rgb(0 0 0 / 25%), 0 8px 24px rgb(0 0 0 / 20%); font: 13px system-ui;';
    card.innerHTML = '<b>Extension popup</b><div class="url" style="margin-top: 6px; opacity: 0.7; word-break: break-all;"></div>';
    card.querySelector('.url').textContent = p.url;
    card.addEventListener('mousedown', (e) => e.stopPropagation());
    popup.addEventListener('mousedown', () => {
      const body = new URLSearchParams({ kind: 'closePopup', extension: p.extension });
      fetch('/mock/extensions/fire', { method: 'POST', body });
    });
    popup.append(card);
    layer.append(popup);
  }

  const events = new EventSource('/mock/window/events');
  events.onmessage = (e) => {
    const msg = JSON.parse(e.data);
    if (msg.t !== 'window') return;
    showSettings(msg.settings);
    showPopup(msg.popup);
  };
}
