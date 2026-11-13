// What a shell sends about extensions. What Go sends is state.extensions (shell.js); how
// the buttons, their menu and the install prompt look is the shell's own. The engine
// draws an extension's popup itself, in a view of its own, under the button that opened
// it, and a press anywhere else in the shell closes it (shell.js).

import { send } from './channel.js';

// pressExtension is a press on extension id's button, the element el: Go opens its popup
// under el, closes it when it is the one open, or tells the extension it was clicked.
export function pressExtension(id, el) {
  const r = el.getBoundingClientRect();
  send({ t: 'extensionClick', extension: id, rect: { x: r.x, y: r.y, w: r.width, h: r.height } });
}

// pinExtension puts extension id's button on the toolbar, or takes it off.
export function pinExtension(id, on) {
  send({ t: 'extensionPin', extension: id, on });
}

// manageExtensions opens settings at its Extensions section.
export function manageExtensions() {
  send({ t: 'extensionsManage' });
}

// answerExtensionPrompt answers the install or removal prompt: yes or no.
export function answerExtensionPrompt(on) {
  send({ t: 'extensionAnswerPrompt', on });
}
