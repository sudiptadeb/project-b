// What a shell sends about a page's dialog. What Go sends is state.dialogs (shell.js); how
// one looks is the shell's own. Every shell draws them: the page waits until answered.

import { send } from './channel.js';

// answerDialog answers dialog id: ok or not, and a prompt's text, or a sign-in's username
// and password.
export function answerDialog(id, ok, text = '', password = '') {
  send({ t: 'answerDialog', id, ok, text, password });
}

// dialogFor is the dialog the page of tab waits on, or null: the oldest, as the page asks
// one at a time.
export function dialogFor(state, tab) {
  return state.dialogs.find((d) => d.tab === tab) || null;
}
