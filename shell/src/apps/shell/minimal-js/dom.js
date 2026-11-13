// The shell's few utilities for drawing with the DOM itself, with no framework: an element
// maker, an SVG maker, and a list kept as one element per item, by key.

const svgNS = 'http://www.w3.org/2000/svg';

// set gives el the attribute name: true sets it bare, false or null takes it away, and any
// other value is written as it is. It writes only a change, so what watches the element,
// such as the SDK watching a drag bar, hears only of changes.
export function set(el, name, value) {
  const text = value === false || value == null ? null : value === true ? '' : String(value);
  if (el.getAttribute(name) === text) return;
  if (text === null) el.removeAttribute(name);
  else el.setAttribute(name, text);
}

function make(el, props, children) {
  for (const [name, value] of Object.entries(props)) {
    if (name.startsWith('on')) el.addEventListener(name.slice(2), value);
    else set(el, name, value);
  }
  el.append(...children);
  return el;
}

// h makes an element: its tag, its props, then its children, nodes or text. A prop named
// on<event> listens for that event; any other is an attribute, as set writes it.
export function h(tag, props = {}, ...children) {
  return make(document.createElement(tag), props, children);
}

// svg makes an SVG element, as h makes an HTML one.
export function svg(tag, props = {}, ...children) {
  return make(document.createElementNS(svgNS, tag), props, children);
}

// icon is a line drawing in the text's colour, size px square, of paths on a box-unit
// square, with any other props of the svg.
export function icon(size, box, paths, props = {}) {
  return svg(
    'svg',
    { width: size, height: size, viewBox: `0 0 ${box} ${box}`, fill: 'none', stroke: 'currentColor', 'stroke-width': 1.5, 'aria-hidden': 'true', ...props },
    ...paths.map((d) => svg('path', { d })),
  );
}

// keyed keeps a list of items drawn as one element each, in their order. make(item) makes
// the view of an item whose key it hasn't drawn: {el, fill(item)}. draw(parent, items)
// makes what is new, fills a view again only when its item changed (or every view, with
// all), puts the elements in parent in order, and takes away those whose key went. A view
// keeps its element while its key stays, so the list can move to another parent whole.
export function keyed(key, make) {
  const views = new Map();
  return {
    el: (k) => views.get(k)?.el,
    draw(parent, items, all = false) {
      const seen = new Set();
      let at = parent.firstChild;
      for (const item of items) {
        const k = key(item);
        seen.add(k);
        let view = views.get(k);
        if (!view) {
          view = make(item);
          views.set(k, view);
        }
        if (all || view.item !== item) {
          view.fill(item);
          view.item = item;
        }
        if (view.el === at) at = at.nextSibling;
        else parent.insertBefore(view.el, at);
      }
      for (const [k, view] of views) {
        if (!seen.has(k)) {
          view.el.remove();
          views.delete(k);
        }
      }
    },
  };
}
