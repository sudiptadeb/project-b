// The address field: the active tab's address. Focused, it selects all of it; typing asks
// Go for suggestions, listed in a plain panel under the field. Up and Down choose one,
// Enter or a click goes, and Escape puts the address back and gives the page the keyboard.

import { activeTab, focusPage, send, state } from '@/sdk/shell.js';
import { shell } from '@/sdk/regions.js';
import { h, set } from './dom.js';

// What the user typed, the suggestions for it, the one chosen (-1 for none), and the id of
// the last question asked, so a late answer to an older one is dropped.
let typed = '';
let items = [];
let chosen = -1;
let asked = 0;
// Whether the field has the keyboard, and whether the press that is giving it the keyboard
// should select all on its release, which would otherwise put the caret down.
let editing = false;
let selectOnRelease = false;

const input = h('input', {
  'aria-label': 'Address',
  role: 'combobox',
  'aria-expanded': 'false',
  'aria-controls': 'suggestions',
  'aria-autocomplete': 'list',
  autocomplete: 'off',
  autocapitalize: 'off',
  spellcheck: 'false',
  oninput: () => {
    typed = input.value;
    if (typed.trim() === '') close();
    else send({ t: 'suggest', id: ++asked, text: typed });
  },
  onkeydown: (e) => {
    if (e.key === 'Enter') go(input.value);
    else if (e.key === 'Escape') {
      e.preventDefault();
      input.blur();
      focusPage();
    } else if ((e.key === 'ArrowDown' || e.key === 'ArrowUp') && items.length) {
      e.preventDefault();
      choose(chosen + (e.key === 'ArrowDown' ? 1 : -1));
      input.value = chosen < 0 ? typed : items[chosen].url;
    }
  },
  onmousedown: () => {
    selectOnRelease = !editing;
  },
  onmouseup: (e) => {
    if (selectOnRelease) {
      e.preventDefault();
      input.select();
      selectOnRelease = false;
    }
  },
  onfocus: () => {
    editing = true;
    typed = input.value;
    input.select();
  },
  onblur: () => {
    editing = false;
    close();
    input.value = activeTab(state).url;
  },
});

// A press on the panel keeps the keyboard in the field.
const list = h('ul', { id: 'suggestions', class: 'suggestions', role: 'listbox', onmousedown: (e) => e.preventDefault() });

export const field = h('div', { class: 'address' }, input);

// choose marks suggestion i, or none for any i outside the list.
function choose(i) {
  chosen = i >= 0 && i < items.length ? i : -1;
  [...list.children].forEach((li, n) => set(li, 'aria-selected', String(n === chosen)));
  set(input, 'aria-activedescendant', chosen < 0 ? null : `suggestion-${chosen}`);
}

// The panel shows while there are suggestions, and is the shell's while it does: it lies
// over the page.
function drawList() {
  list.replaceChildren(
    ...items.map((item, i) =>
      h(
        'li',
        { id: `suggestion-${i}`, role: 'option', 'aria-selected': 'false', onmouseenter: () => choose(i), onclick: () => go(item.url) },
        h('span', { class: 'title' }, item.title || item.url),
        item.title ? h('span', { class: 'url' }, item.url) : '',
      ),
    ),
  );
  chosen = -1;
  set(input, 'aria-activedescendant', null);
  const open = items.length > 0;
  if (open === list.isConnected) return;
  set(input, 'aria-expanded', String(open));
  if (open) {
    field.append(list);
    shell.mark(list);
  } else {
    shell.unmark(list);
    list.remove();
  }
}

function close() {
  items = [];
  asked++;
  drawList();
}

function go(text) {
  const value = text.trim();
  if (value === '') return;
  send({ t: 'navigate', text: value });
  input.blur();
  focusPage();
}

export const address = {
  // Go's answer to a question: drawn if it is the last one asked.
  suggestions(m) {
    if (m.id !== asked) return;
    items = m.items;
    drawList();
  },
  // The keyboard in the field, all of it selected.
  focus() {
    input.focus();
    input.select();
  },
  blur() {
    input.blur();
  },
  // The active tab's address, unless the user is editing it.
  show() {
    if (!editing) input.value = activeTab(state).url;
  },
  // Another tab became active: its address, whatever was being typed.
  reset() {
    close();
    input.value = activeTab(state).url;
  },
};
