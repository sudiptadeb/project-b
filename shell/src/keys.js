// Keys as the settings page records and shows them. A combination is written the way
// the keymap file writes it: modifiers in order (Ctrl, Option or Alt, Shift, Cmd or Win)
// and the key's name, joined with +.

// The key's name, from KeyboardEvent.code: the physical key, as on a US keyboard.
const named = {
  Backspace: 'Backspace', Tab: 'Tab', Enter: 'Enter', Escape: 'Escape', Space: 'Space',
  PageUp: 'PageUp', PageDown: 'PageDown', End: 'End', Home: 'Home',
  ArrowLeft: 'Left', ArrowUp: 'Up', ArrowRight: 'Right', ArrowDown: 'Down',
  Insert: 'Insert', Delete: 'Delete',
  Semicolon: ';', Equal: '=', Comma: ',', Minus: '-', Period: '.', Slash: '/', Backquote: '`',
  BracketLeft: '[', Backslash: '\\', BracketRight: ']', Quote: "'",
};

function keyName(code) {
  if (named[code]) return named[code];
  let m = /^Key([A-Z])$/.exec(code) || /^Digit([0-9])$/.exec(code) || /^(F[0-9]{1,2})$/.exec(code);
  return m ? m[1] : '';
}

// comboOf is the combination a key press is, or '' for a modifier alone or a key no name
// covers.
export function comboOf(e, platform) {
  const key = keyName(e.code);
  if (!key) return '';
  const mac = platform === 'mac';
  const parts = [];
  if (e.ctrlKey) parts.push('Ctrl');
  if (e.altKey) parts.push(mac ? 'Option' : 'Alt');
  if (e.shiftKey) parts.push('Shift');
  if (e.metaKey) parts.push(mac ? 'Cmd' : 'Win');
  parts.push(key);
  return parts.join('+');
}

// On a Mac, keys show as the menus show them: ⌃⌥⇧⌘ and the key.
const symbols = { Ctrl: '⌃', Option: '⌥', Shift: '⇧', Cmd: '⌘' };
const keySymbols = {
  Left: '←', Right: '→', Up: '↑', Down: '↓', Enter: '↩', Escape: '⎋', Tab: '⇥',
  Backspace: '⌫', Delete: '⌦', PageUp: '⇞', PageDown: '⇟', Home: '↖', End: '↘', Space: 'Space',
};

export function shown(keys, platform) {
  if (platform !== 'mac') return keys;
  const parts = keys.split('+');
  const key = parts.pop();
  return parts.map((p) => symbols[p] || p).join('') + (keySymbols[key] || key);
}
