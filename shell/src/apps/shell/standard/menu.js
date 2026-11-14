import { reactive } from 'vue';

// The one menu the standard shell has open at a time: a group's or a tab's, at the point
// it was asked for. FloatMenu draws it; the lists only open it.
export const menu = reactive({ kind: '', id: null, x: 0, y: 0, keyboard: false });

// openMenu opens kind's menu for id at the event's point, or, from the keyboard, under the
// element it came from.
export function openMenu(kind, id, e) {
  e.preventDefault();
  e.stopPropagation();
  const fromKeys = !e.clientX && !e.clientY;
  const r = e.currentTarget.getBoundingClientRect();
  Object.assign(menu, {
    kind,
    id,
    x: fromKeys ? r.left : e.clientX,
    y: fromKeys ? r.bottom + 4 : e.clientY,
    keyboard: fromKeys,
  });
}

export function closeMenu() {
  menu.kind = '';
  menu.id = null;
}
