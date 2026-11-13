// Escape closes the innermost thing open: a menu, then a dialog, then whatever opened
// them. Each layer pushes its close here while it is open, and one listener, ahead of
// every other, runs the last one pushed and keeps Escape from reaching anything else.
// A page that closes itself on Escape (settings) does so only when no layer took it.

const layers = [];

function onKey(e) {
  if (e.key !== 'Escape' || !layers.length) return;
  e.preventDefault();
  e.stopImmediatePropagation();
  layers[layers.length - 1]();
}

if (typeof window !== 'undefined') window.addEventListener('keydown', onKey, true);

// pushEscape adds a layer and returns the function that takes it off again.
export function pushEscape(close) {
  layers.push(close);
  return () => {
    const i = layers.lastIndexOf(close);
    if (i >= 0) layers.splice(i, 1);
  };
}
