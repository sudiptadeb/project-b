// A switcher card's gestures, as an action: use:gesture={{ id, onclose, onmenu, ondrop }}.
// A swipe sideways past a third of its width closes it (onclose). Held still for a moment
// it lifts: let go where it is, it opens its menu (onmenu); moved, it follows the finger,
// and let go over another card, ondrop(target) gets that card's element. A tap is left
// to the card's own click.

const hold = 350;

export function gesture(el, params) {
  let p = params;
  let start = null;
  let timer = 0;
  let mode = '';
  // While lifted, the grid mustn't scroll under the finger.
  const still = (e) => mode === 'lift' && e.cancelable && e.preventDefault();
  el.addEventListener('touchmove', still, { passive: false });
  // A phone's own long press comes while the finger is still down: the lift has it.
  const press = (e) => {
    if (start) {
      e.preventDefault();
      e.stopPropagation();
    }
  };
  el.addEventListener('contextmenu', press, true);
  // After a swipe or a lift, the click that follows mustn't open the card.
  const eat = (c) => c.stopImmediatePropagation();
  const noClick = () => {
    el.addEventListener('click', eat, { capture: true, once: true });
    setTimeout(() => el.removeEventListener('click', eat, { capture: true }), 300);
  };

  function reset() {
    clearTimeout(timer);
    el.style.transform = '';
    el.style.transition = '';
    el.classList.remove('lifted', 'swiping');
    start = null;
    mode = '';
  }

  function down(e) {
    if (e.button > 0 || e.target.closest('button')) return;
    start = { x: e.clientX, y: e.clientY };
    mode = '';
    el.setPointerCapture(e.pointerId);
    timer = setTimeout(() => {
      mode = 'lift';
      el.classList.add('lifted');
    }, hold);
  }

  function move(e) {
    if (!start) return;
    const dx = e.clientX - start.x;
    const dy = e.clientY - start.y;
    if (mode === '' && Math.hypot(dx, dy) > 8) {
      clearTimeout(timer);
      if (Math.abs(dx) > Math.abs(dy)) {
        mode = 'swipe';
        el.classList.add('swiping');
      } else {
        // A scroll of the grid: the browser has it.
        start = null;
        return;
      }
    }
    if (mode === 'swipe') {
      el.style.transform = `translateX(${dx}px)`;
      el.style.opacity = String(1 - Math.min(Math.abs(dx) / el.offsetWidth, 1) * 0.7);
    } else if (mode === 'lift') {
      el.style.transform = `translate(${dx}px, ${dy}px) scale(1.04)`;
    }
  }

  function up(e) {
    if (!start) return reset();
    const dx = e.clientX - start.x;
    const dy = e.clientY - start.y;
    const was = mode;
    if (was) noClick();
    if (was === 'swipe' && Math.abs(dx) > el.offsetWidth / 3) {
      el.style.transition = 'transform 150ms ease-out';
      el.style.transform = `translateX(${Math.sign(dx) * el.offsetWidth * 1.5}px)`;
      start = null;
      clearTimeout(timer);
      setTimeout(() => {
        el.style.opacity = '';
        reset();
        p.onclose();
      }, 150);
      return;
    }
    el.style.opacity = '';
    if (was === 'lift') {
      if (Math.hypot(dx, dy) < 12) {
        reset();
        p.onmenu();
        return;
      }
      el.style.visibility = 'hidden';
      const under = document.elementFromPoint(e.clientX, e.clientY)?.closest('[data-tab], [data-group]');
      el.style.visibility = '';
      reset();
      if (under && under !== el && !el.contains(under)) p.ondrop(under);
      return;
    }
    reset();
  }

  el.addEventListener('pointerdown', down);
  el.addEventListener('pointermove', move);
  el.addEventListener('pointerup', up);
  el.addEventListener('pointercancel', () => {
    el.style.opacity = '';
    reset();
  });
  return {
    update: (next) => (p = next),
    destroy: () => {
      clearTimeout(timer);
      el.removeEventListener('touchmove', still);
      el.removeEventListener('contextmenu', press, true);
    },
  };
}
