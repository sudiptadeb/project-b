// The dev window: on the mock engine, the browser tab is the OS window, and this draws
// what the real window holds in views of its own beside the shell. Go's mock engine says
// what that is on /mock/window/events, the state on connecting and then each change, as
// {"t":"window","page":{x,y,w,h},"settings":bool,"popup":{...}|null}, and draws it on the
// html element, outside the shell's app:
//
//   page      the mock page, in an iframe under the shell where the window manager's
//             layout puts the page, as the real window places the tab's view under the
//             shell's unpainted hole. As there, a press goes to the shell only inside a
//             region it marked (v-shell, v-drag), and to the page everywhere else.
//
// and above the shell:
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
  const under = document.createElement('div');
  under.className = 'mock-window-under';
  under.style.cssText = 'position: fixed; inset: 0; z-index: -1; pointer-events: none;';
  const style = document.createElement('style');
  style.textContent = 'html, body, #app, #app * { pointer-events: none; } #app [data-region], #app [data-region] * { pointer-events: auto; }';
  document.documentElement.append(under, layer, style);

  const page = document.createElement('iframe');
  page.className = 'mock-page';
  page.title = 'Mock page';
  page.src = '/mock/page';
  page.style.cssText = 'position: absolute; border: 0; pointer-events: auto; display: none;';
  under.append(page);

  function placePage(r) {
    const shown = r && r.w > 0 && r.h > 0;
    page.style.display = shown ? 'block' : 'none';
    if (shown) Object.assign(page.style, { left: `${r.x}px`, top: `${r.y}px`, width: `${r.w}px`, height: `${r.h}px` });
  }

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
    placePage(msg.page);
    showSettings(msg.settings);
    showPopup(msg.popup);
  };
}
