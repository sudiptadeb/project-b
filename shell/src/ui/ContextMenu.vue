<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue';
import { pushEscape } from './escape.js';

// The design system's floating menu, opened at a point: a right-click's. items is a list of
// { label, run, icon, key, dot, disabled, danger, items } and { separator: true }, items
// being a submenu's. It keeps inside the window, flipping left or up. The arrow keys move
// through it, → and ← open and close a submenu, Enter runs an item, a letter jumps to the
// next item starting with it, and Escape, Tab or a press outside closes it. Opened by the
// mouse, no item is highlighted until the pointer or a key moves. The cover under it takes
// every press outside, over the page too: it is the root, for the caller to mark as the
// shell's.
const props = defineProps({ items: Array, x: Number, y: Number, label: String, keyboard: Boolean });
const emit = defineEmits(['close']);

const menu = ref(null);
const sub = ref(null);
const at = ref({ left: props.x, top: props.y });
// The submenu open: its parent's index, and where it shows.
const open = ref(-1);
const subAt = ref({ left: 0, top: 0 });
let hover = 0;
let pop = null;

const edge = 4;
const hasIcons = computed(() => props.items.some((i) => i.icon || i.dot));
const subItems = computed(() => props.items[open.value]?.items || []);
const subIcons = computed(() => subItems.value.some((i) => i.icon || i.dot));

function entries(el) {
  return [...(el?.querySelectorAll(':scope > [role="menuitem"]:not([aria-disabled="true"])') || [])];
}

// place puts el's top-left at x, y, or flipped when it would cross the window's edge:
// left of flipX, above y + h.
function place(el, x, y, flipX, h = 0) {
  const r = el.getBoundingClientRect();
  let left = x;
  let top = y;
  if (left + r.width > innerWidth - edge) left = Math.max(edge, (flipX ?? innerWidth) - r.width);
  if (top + r.height > innerHeight - edge) top = Math.max(edge, Math.min(y + h, innerHeight - edge) - r.height);
  return { left, top };
}

onMounted(async () => {
  pop = pushEscape(() => (open.value >= 0 ? closeSub(true) : emit('close')));
  addEventListener('blur', onBlur);
  await nextTick();
  at.value = place(menu.value, props.x, props.y, props.x);
  if (props.keyboard) entries(menu.value)[0]?.focus();
  else menu.value?.focus();
});

onUnmounted(() => {
  pop?.();
  removeEventListener('blur', onBlur);
  clearTimeout(hover);
});

function onBlur() {
  emit('close');
}

async function openSub(i, focus) {
  clearTimeout(hover);
  if (open.value !== i) {
    open.value = i;
    await nextTick();
    const row = entries(menu.value).find((el) => +el.dataset.i === i) || menu.value;
    const r = row.getBoundingClientRect();
    const m = menu.value.getBoundingClientRect();
    subAt.value = place(sub.value, m.right - 2, r.top - 5, m.left + 2, r.height + 10);
  }
  if (focus) entries(sub.value)[0]?.focus();
}

function keepSub() {
  clearTimeout(hover);
}

function closeSub(refocus) {
  const i = open.value;
  open.value = -1;
  if (refocus) entries(menu.value).find((el) => +el.dataset.i === i)?.focus();
}

function enter(item, i) {
  clearTimeout(hover);
  if (item.items) hover = setTimeout(() => openSub(i, false), 150);
  else if (open.value >= 0) hover = setTimeout(() => closeSub(false), 150);
}

function choose(item, i) {
  if (item.disabled) return;
  if (item.items) return openSub(i, true);
  item.run();
  emit('close');
}

// The pointer moving over a row focuses it, as a highlight: the keyboard's ring shows only
// once a key moves.
const pointer = ref(!props.keyboard);
function point(e) {
  pointer.value = true;
  if (document.activeElement !== e.currentTarget) e.currentTarget.focus();
}

function onKey(e, inSub) {
  pointer.value = false;
  const el = inSub ? sub.value : menu.value;
  const list = entries(el);
  const i = list.indexOf(document.activeElement);
  // From no item, down goes to the first and up to the last.
  const step = (by) => list[i < 0 ? (by > 0 ? 0 : list.length - 1) : (i + by + list.length) % list.length]?.focus();
  if (e.key === 'ArrowDown') step(1);
  else if (e.key === 'ArrowUp') step(-1);
  else if (e.key === 'Home') list[0]?.focus();
  else if (e.key === 'End') list[list.length - 1]?.focus();
  else if (e.key === 'ArrowRight' && !inSub && i >= 0 && props.items[+list[i].dataset.i]?.items) openSub(+list[i].dataset.i, true);
  else if (e.key === 'ArrowLeft' && inSub) closeSub(true);
  else if (e.key === 'Tab') emit('close');
  else if (e.key.length === 1 && /\S/.test(e.key)) {
    const k = e.key.toLowerCase();
    const n = list.length;
    for (let j = 1; j <= n; j++) {
      const el = list[(i + j + n) % n];
      if (el.textContent.trim().toLowerCase().startsWith(k)) {
        el.focus();
        break;
      }
    }
  } else return;
  e.preventDefault();
  e.stopPropagation();
}
</script>

<template>
  <div class="ui-context-cover" @pointerdown.self="emit('close')" @contextmenu.self.prevent="emit('close')" @wheel.self="emit('close')">
  <div
    ref="menu"
    class="ui-menu ui-context-menu"
    :class="{ pointer }"
    role="menu"
    tabindex="-1"
    :aria-label="label"
    :style="{ left: at.left + 'px', top: at.top + 'px' }"
    @keydown="onKey($event, false)"
    @contextmenu.prevent
  >
    <template v-for="(item, i) in items" :key="i">
      <div v-if="item.separator" class="ui-menu-separator" role="separator"></div>
      <div
        v-else
        role="menuitem"
        tabindex="-1"
        class="ui-menu-item"
        :class="{ danger: item.danger, open: open === i }"
        :data-i="i"
        :aria-disabled="item.disabled ? 'true' : undefined"
        :aria-haspopup="item.items ? 'menu' : undefined"
        :aria-expanded="item.items ? open === i : undefined"
        @pointerenter="enter(item, i)"
        @pointermove="!item.disabled && point($event)"
        @click="choose(item, i)"
        @keydown.enter.space.prevent="choose(item, i)"
      >
        <span v-if="hasIcons" class="ui-menu-icon" aria-hidden="true">
          <i v-if="item.dot" class="ui-menu-dot" :style="{ background: item.dot }"></i>
          <svg v-else-if="item.icon" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" v-html="item.icon"></svg>
        </span>
        <span class="ui-menu-label">{{ item.label }}</span>
        <span v-if="item.key" class="ui-menu-key">{{ item.key }}</span>
        <svg v-if="item.items" class="ui-menu-chevron" width="6" height="10" viewBox="0 0 6 10" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true"><path d="M1 1l4 4-4 4" /></svg>
      </div>
    </template>
  </div>
  <div
    v-if="open >= 0"
    ref="sub"
    class="ui-menu ui-context-menu"
    :class="{ pointer }"
    role="menu"
    :aria-label="items[open].label"
    :style="{ left: subAt.left + 'px', top: subAt.top + 'px' }"
    @keydown="onKey($event, true)"
    @pointerenter="keepSub"
    @contextmenu.prevent
  >
    <div
      v-for="(item, i) in subItems"
      :key="i"
      role="menuitem"
      tabindex="-1"
      class="ui-menu-item"
      :data-i="i"
      :aria-disabled="item.disabled ? 'true' : undefined"
      @pointermove="!item.disabled && point($event)"
      @click="choose(item, i)"
      @keydown.enter.space.prevent="choose(item, i)"
    >
      <span v-if="subIcons" class="ui-menu-icon" :class="{ dot: subItems.every((i) => i.dot) }" aria-hidden="true">
        <i v-if="item.dot" class="ui-menu-dot" :style="{ background: item.dot }"></i>
        <svg v-else-if="item.icon" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" v-html="item.icon"></svg>
      </span>
      <span class="ui-menu-label">{{ item.label }}</span>
    </div>
  </div>
  </div>
</template>

<style>
/* Over the whole window, page included, so a press anywhere outside the menu closes it. */
.ui-context-cover {
  position: fixed;
  inset: 0;
  z-index: 100;
}

.ui-menu.ui-context-menu {
  position: fixed;
  right: auto;
  z-index: 101;
  min-width: 220px;
  max-width: 320px;
  outline: none;
}

/* Rows are divs, not RowMenu's buttons, so they need the buttons' box. */
.ui-context-menu .ui-menu-item {
  box-sizing: border-box;
  gap: 10px;
  user-select: none;
}

.ui-context-menu .ui-menu-item[aria-disabled='true'] {
  opacity: 0.5;
  cursor: default;
}

.ui-context-menu .ui-menu-item[aria-disabled='true']:hover {
  background: none;
}

.ui-context-menu .ui-menu-item:focus {
  outline: none;
  background: var(--accent-tint);
}

.ui-context-menu.pointer .ui-menu-item:focus-visible {
  box-shadow: none;
}

.ui-context-menu .ui-menu-item.open {
  background: var(--accent-tint);
}

.ui-menu-icon {
  display: flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  color: var(--icon, var(--muted));
}

.ui-menu-icon.dot {
  width: 10px;
}

.ui-menu-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.ui-menu-label {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.ui-menu-key {
  flex: none;
  color: var(--muted);
  font-size: 12px;
}

.ui-menu-chevron {
  flex: none;
  color: var(--icon, var(--muted));
}
</style>
