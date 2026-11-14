// What a shell sends about find in page. What Go sends is state.find (shell.js), and
// focusFind when the find key is pressed; how the bar looks is the shell's own. Each names
// the tab whose bar it is, state.find.tab, and goes nowhere once that bar has closed.

import { send } from './channel.js';

// findText searches tab's page for text afresh, as it is typed.
export function findText(tab, text) {
  send({ t: 'findText', tab, text });
}

// findNext shows the next match in tab's page, or the one before.
export function findNext(tab, forward = true) {
  send({ t: 'findNext', tab, forward });
}

// closeFind closes tab's bar, leaving the match shown selected.
export function closeFind(tab) {
  send({ t: 'findClose', tab });
}
