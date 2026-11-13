// The extensions, drawn from the SDK's state.extensions: a button for each pinned one, then
// Extensions, whose menu lists them all to open, pin or unpin, and Manage extensions; and
// the install or removal prompt over the window. The classes and labels are what
// tools/drive_extensions.py finds them by (docs/shells.md). The engine draws an
// extension's popup itself, under the button that opened it.

import { answerExtensionPrompt, manageExtensions, pinExtension, pressExtension } from '@/sdk/extensions.js';
import { shell } from '@/sdk/regions.js';
import { state } from '@/sdk/shell.js';
import { h, icon, keyed, set } from './dom.js';
import './extensions.css';

// An extension's icon, or the first letter of its name when it has none.
const iconOf = (b, size) =>
  b.icon
    ? h('img', { class: 'pbx-icon', src: b.icon, alt: '', width: size, height: size })
    : h('span', { class: `pbx-icon pbx-letter size${size}`, 'aria-hidden': 'true' }, b.name.slice(0, 1).toUpperCase());

const puzzle = () =>
  icon(14, 24, ['M20.5 11H19V7a2 2 0 0 0-2-2h-4V3.5a2.5 2.5 0 0 0-5 0V5H4a2 2 0 0 0-2 2v3.8h1.5a2.7 2.7 0 0 1 0 5.4H2V20a2 2 0 0 0 2 2h3.8v-1.5a2.7 2.7 0 0 1 5.4 0V22H17a2 2 0 0 0 2-2v-4h1.5a2.5 2.5 0 0 0 0-5Z'], {
    'stroke-width': 2,
    'stroke-linejoin': 'round',
  });

// A push pin, filled while the extension is on the toolbar.
const pin = (on) =>
  icon(14, 24, ['M8 3.5h8', 'M10 3.5h4v5.5l3.5 4.5h-11L10 9z', 'M12 13.5v7'], { 'stroke-width': 2, 'stroke-linecap': 'round', 'stroke-linejoin': 'round', class: on ? 'on' : null });

// A pinned extension's button. A press tells Go where it is: Go opens its popup under it.
function pinnedView(b) {
  const el = h('button', { class: 'btn pbx-action', type: 'button', 'data-extension': b.id, onclick: () => pressExtension(b.id, el) });
  return {
    el,
    fill(b) {
      const name = b.title || b.name;
      set(el, 'aria-label', name);
      set(el, 'title', name);
      set(el, 'disabled', !b.enabled);
      el.classList.toggle('pbx-open', state.extensions.popup === b.id);
      el.replaceChildren(iconOf(b, 16));
      if (b.badge) {
        const badge = h('span', { class: 'pbx-badge' }, b.badge.slice(0, 4));
        // The extension's own colours, when it set them.
        badge.style.background = b.badgeColor;
        badge.style.color = b.badgeTextColor;
        el.append(badge);
      }
    },
  };
}

// The install or removal prompt, a modal dialog. Go takes it away once it has an answer.
// The dialog closing, as Escape closes it, answers no; taking it away doesn't close it.
function promptDialog(p) {
  const head = (title, ...more) => h('div', { class: 'head' }, iconOf(p, 32), h('div', {}, h('h2', {}, title), ...more));
  const body = p.remove
    ? [head(`Remove "${p.name}"?`), h('p', {}, 'Its data is deleted too.')]
    : [
        head(`Add "${p.name}"?`, h('div', { class: 'version' }, `Version ${p.version}`)),
        ...(p.permissions?.length
          ? [h('p', { class: 'label' }, 'It can'), h('ul', {}, ...p.permissions.map((x) => h('li', {}, x)))]
          : [h('p', {}, 'It asks for no permissions.')]),
      ];
  return h(
    'dialog',
    {
      class: 'pbx-prompt',
      'aria-label': `${p.remove ? 'Remove' : 'Add'} ${p.name}`,
      onclose: () => answerExtensionPrompt(false),
    },
    ...body,
    h(
      'div',
      { class: 'buttons' },
      h('button', { class: 'plain', type: 'button', onclick: () => answerExtensionPrompt(false) }, 'Cancel'),
      h('button', { class: 'plain primary', type: 'button', onclick: () => answerExtensionPrompt(true) }, p.remove ? 'Remove' : 'Add extension'),
    ),
  );
}

// extensionsPart makes the extensions' part of the toolbar, el, and draw, which follows
// state.extensions: null while their flag is off, and then nothing shows. Made only in a
// build that has extensions.
export function extensionsPart() {
  const pinned = keyed((b) => b.id, pinnedView);
  const pinnedBox = h('div', { class: 'pinned' });
  const opener = h('button', { class: 'btn', type: 'button', 'aria-label': 'Extensions', title: 'Extensions', 'aria-expanded': 'false', onclick: () => (menu.isConnected ? closeMenu() : openMenu()) }, puzzle());
  const menu = h('div', { class: 'pbx-menu', role: 'group', 'aria-label': 'Extensions menu' });
  const anchor = h('div', { class: 'anchor' }, opener);
  const el = h('div', { class: 'exts', hidden: true }, pinnedBox, anchor);

  // The menu's rows, from what Go sent last. A popup opened from it opens under Extensions.
  function fillMenu() {
    const { buttons } = state.extensions;
    menu.replaceChildren(
      ...buttons.map((b) =>
        h(
          'div',
          { class: 'row' },
          h('button', { class: 'item', type: 'button', disabled: !b.enabled, onclick: () => (closeMenu(), pressExtension(b.id, opener)) }, iconOf(b, 16), h('span', { class: 'name' }, b.name)),
          h('button', { class: 'pin', type: 'button', 'aria-label': `${b.pinned ? 'Unpin' : 'Pin'} ${b.name}`, title: b.pinned ? 'Unpin from the toolbar' : 'Pin to the toolbar', onclick: () => pinExtension(b.id, !b.pinned) }, pin(b.pinned)),
        ),
      ),
      ...(buttons.length ? [] : [h('p', { class: 'empty' }, 'No extensions installed')]),
      h('div', { class: 'line' }),
      h('button', { class: 'item pbx-manage', type: 'button', onclick: () => (closeMenu(), manageExtensions()) }, 'Manage extensions'),
    );
  }

  // The menu closes on Escape, on choosing one of its items, and on its own button.
  const onKey = (e) => e.key === 'Escape' && closeMenu();

  function openMenu() {
    fillMenu();
    anchor.append(menu);
    shell.mark(menu);
    set(opener, 'aria-expanded', 'true');
    opener.classList.add('pbx-open');
    addEventListener('keydown', onKey);
  }

  function closeMenu() {
    removeEventListener('keydown', onKey);
    shell.unmark(menu);
    menu.remove();
    set(opener, 'aria-expanded', 'false');
    opener.classList.remove('pbx-open');
  }

  // The prompt shown, as "<id> <remove>", and the cover it is on, which makes the whole
  // window the shell's while it asks, so no press reaches the page.
  let asking = '';
  let cover = null;

  function drawPrompt(p) {
    const now = p ? `${p.id} ${p.remove}` : '';
    if (now === asking) return;
    asking = now;
    if (cover) {
      shell.unmark(cover);
      cover.remove();
      cover = null;
    }
    if (!p) return;
    const dialog = promptDialog(p);
    cover = h('div', { class: 'cover' }, dialog);
    document.body.append(cover);
    shell.mark(cover);
    dialog.showModal();
  }

  function draw() {
    const ext = state.extensions;
    el.hidden = !ext;
    if (!ext) {
      if (menu.isConnected) closeMenu();
      drawPrompt(null);
      return;
    }
    pinned.draw(pinnedBox, ext.buttons.filter((b) => b.pinned));
    if (menu.isConnected) fillMenu();
    drawPrompt(ext.prompt);
  }

  return { el, draw };
}
