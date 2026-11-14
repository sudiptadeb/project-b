import { reactive } from 'vue';

// The one menu the standard shell has open at a time: a group's or a tab's, at the point
// it was asked for. FloatMenu draws it; the lists only open it.
// above is where it ends when it has no room below and opens upward instead.
export const menu = reactive({ kind: '', id: null, x: 0, y: 0, above: 0, keyboard: false });

// openMenu opens kind's menu for id at the event's point, or, from the keyboard or when
// anchored to a button, under the element it came from (over it, without room below).
export function openMenu(kind, id, e, anchored = false) {
  e.preventDefault();
  e.stopPropagation();
  const fromKeys = !e.clientX && !e.clientY;
  const r = e.currentTarget.getBoundingClientRect();
  const under = fromKeys || anchored;
  Object.assign(menu, {
    kind,
    id,
    x: under ? r.left : e.clientX,
    y: under ? r.bottom + 4 : e.clientY,
    above: under ? r.top - 4 : e.clientY,
    keyboard: fromKeys,
  });
}

export function closeMenu() {
  menu.kind = '';
  menu.id = null;
}
