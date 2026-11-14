// What a shell sends about the right-click menu. What Go sends is state.contextMenu
// (shell.js): what the press was on and the actions that can run there. How the menu looks,
// which actions it shows and in what order is the shell's own (docs/daily-driver.md).

import { send } from './channel.js';
import { pageRect } from './regions.js';

// chooseContextItem runs action id of the open menu; Go closes the menu.
export function chooseContextItem(menu, id) {
  send({ t: 'contextMenuChoose', id: menu.id, item: id });
}

// closeContextMenu says the menu went without running anything.
export function closeContextMenu(menu) {
  send({ t: 'contextMenuClose', id: menu.id });
}

// contextMenuPoint is where in the window the press was: the menu's point in its page,
// from where the shell shows that page. Go keeps a menu only while its tab is shown.
export function contextMenuPoint(menu) {
  const r = pageRect(menu.tab);
  return { x: r.left + menu.x, y: r.top + menu.y };
}
