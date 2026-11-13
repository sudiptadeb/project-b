<script setup>
import { computed, ref, watch } from 'vue';
import { send } from '@/sdk/channel.js';
import ExtensionIcon from './ExtensionIcon.vue';
import InstallPrompt from './InstallPrompt.vue';
import PinIcon from './PinIcon.vue';
import { extensions } from './state.js';

// The toolbar's extensions, as Chrome has them: a button for each pinned one, then the
// puzzle piece, whose menu lists them all. A press only tells Go where the button is. The
// engine opens the popup under it, in a view of its own, so the shell never draws it.
const pinned = computed(() => extensions.value.buttons.filter((b) => b.pinned));
const toolbar = ref(null);
const puzzle = ref(null);
const menu = ref(false);

function press(id, el) {
  const r = el.getBoundingClientRect();
  send({ t: 'extensionClick', extension: id, rect: { x: r.x, y: r.y, w: r.width, h: r.height } });
}

// From the menu, the popup opens under the puzzle piece.
function openFromMenu(id) {
  menu.value = false;
  press(id, puzzle.value);
}

function manage() {
  menu.value = false;
  send({ t: 'extensionsManage' });
}

// The menu closes on Escape, on a press anywhere else in the shell, and when the page
// takes the focus from the shell.
function onKey(e) {
  if (e.key === 'Escape') menu.value = false;
}
function onPress(e) {
  if (!toolbar.value.contains(e.target)) menu.value = false;
}
function onBlur() {
  menu.value = false;
}

// While a popup is open, a press anywhere in the shell closes it. The one exception is
// the popup's own button: that press goes to Go as any press on it does, and Go closes
// the popup it finds open. The engine closes it for a press outside the shell.
function onPressWithPopup(e) {
  const own = e.target.closest('[data-extension]');
  if (!own || own.dataset.extension !== extensions.value.popup) send({ t: 'extensionClosePopup' });
}
// Nothing is known of extensions until the snapshot arrives.
watch(
  () => extensions.value?.popup,
  (open) => {
    if (open) window.addEventListener('pointerdown', onPressWithPopup, true);
    else window.removeEventListener('pointerdown', onPressWithPopup, true);
  },
);

watch(menu, (on) => {
  if (on) {
    window.addEventListener('keydown', onKey);
    window.addEventListener('pointerdown', onPress, true);
    window.addEventListener('blur', onBlur);
  } else {
    window.removeEventListener('keydown', onKey);
    window.removeEventListener('pointerdown', onPress, true);
    window.removeEventListener('blur', onBlur);
  }
});
</script>

<template>
  <div v-if="extensions" ref="toolbar" class="pbx-toolbar">
    <button
      v-for="b in pinned"
      :key="b.id"
      class="icon-button pbx-action"
      :class="{ 'pbx-open': extensions.popup === b.id }"
      :data-extension="b.id"
      type="button"
      :disabled="!b.enabled"
      :aria-label="b.title || b.name"
      :title="b.title || b.name"
      @click="press(b.id, $event.currentTarget)"
    >
      <ExtensionIcon :src="b.icon" :name="b.name" />
      <span v-if="b.badge" class="pbx-badge" :style="{ background: b.badgeColor || null, color: b.badgeTextColor || null }">{{ b.badge.slice(0, 4) }}</span>
    </button>
    <div class="pbx-puzzle">
      <!-- A puzzle piece, Chrome's sign for extensions, at the toolbar's 1.5px stroke. -->
      <button
        ref="puzzle"
        class="icon-button"
        :class="{ 'pbx-open': menu }"
        type="button"
        aria-label="Extensions"
        title="Extensions"
        :aria-expanded="menu"
        @click="menu = !menu"
      >
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linejoin="round"><path d="M20.5 11H19V7a2 2 0 0 0-2-2h-4V3.5a2.5 2.5 0 0 0-5 0V5H4a2 2 0 0 0-2 2v3.8h1.5a2.7 2.7 0 0 1 0 5.4H2V20a2 2 0 0 0 2 2h3.8v-1.5a2.7 2.7 0 0 1 5.4 0V22H17a2 2 0 0 0 2-2v-4h1.5a2.5 2.5 0 0 0 0-5Z"/></svg>
      </button>
      <div v-if="menu" v-shell class="pbx-menu" role="group" aria-label="Extensions menu">
        <div v-for="b in extensions.buttons" :key="b.id" class="pbx-row">
          <button type="button" class="pbx-item" :disabled="!b.enabled" @click="openFromMenu(b.id)">
            <ExtensionIcon :src="b.icon" :name="b.name" />
            <span class="pbx-name">{{ b.name }}</span>
          </button>
          <button
            type="button"
            class="icon-button pbx-pin"
            :class="{ 'pbx-pinned': b.pinned }"
            :aria-label="(b.pinned ? 'Unpin ' : 'Pin ') + b.name"
            :title="b.pinned ? 'Unpin from the toolbar' : 'Pin to the toolbar'"
            @click="send({ t: 'extensionPin', extension: b.id, on: !b.pinned })"
          >
            <PinIcon :on="b.pinned" />
          </button>
        </div>
        <p v-if="!extensions.buttons.length" class="pbx-empty">No extensions installed</p>
        <div class="pbx-line"></div>
        <button type="button" class="pbx-item pbx-manage" @click="manage">Manage extensions</button>
      </div>
    </div>
    <!-- An install a store's page began asks here, over the page. The cover makes the
         whole window the shell's while it does, so no press reaches the page. -->
    <div v-if="extensions.prompt" v-shell class="pbx-cover">
      <InstallPrompt :prompt="extensions.prompt" :send="send" />
    </div>
  </div>
</template>

<style scoped>
.pbx-cover {
  position: fixed;
  inset: 0;
}

/* The same 4px apart as the other toolbar buttons, and 12px from the address bar. */
.pbx-toolbar {
  flex: none;
  display: flex;
  align-items: center;
  gap: 4px;
  margin-left: 12px;
}

.pbx-action {
  position: relative;
}

/* A button whose popup is open, or the puzzle piece with its menu open, stays pressed. */
.pbx-open,
.pbx-open:hover:not(:disabled) {
  background: var(--accent-soft);
}

/* Chrome's badge: a small rounded label over the icon's bottom-right corner. */
.pbx-badge {
  position: absolute;
  right: 1px;
  bottom: 2px;
  min-width: 8px;
  height: 12px;
  padding: 0 2px;
  border-radius: 3px;
  background: var(--accent);
  color: var(--on-accent);
  font-size: 9px;
  font-weight: 600;
  line-height: 12px;
  text-align: center;
  white-space: nowrap;
  pointer-events: none;
}

.pbx-puzzle {
  position: relative;
}

/* A floating surface under the puzzle piece, over the page's top-right, with the shadow
   the address bar's suggestions have. */
.pbx-menu {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  z-index: 10;
  width: 280px;
  padding: 4px;
  box-sizing: border-box;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
  box-shadow: var(--shadow-float);
}

.pbx-row {
  display: flex;
  align-items: center;
  gap: 2px;
}

.pbx-item {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  height: 32px;
  padding: 0 8px;
  border: 0;
  border-radius: 4px;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}

.pbx-item:hover:not(:disabled) {
  background: var(--accent-tint);
}

.pbx-item:disabled {
  opacity: 0.4;
  cursor: default;
}

.pbx-item:focus-visible {
  outline: none;
  box-shadow: inset 0 0 0 2px var(--accent-soft);
}

.pbx-manage {
  width: 100%;
}

.pbx-name {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.pbx-pinned {
  color: var(--accent);
}

.pbx-empty {
  margin: 0;
  padding: 8px;
  color: var(--muted);
  font-size: 13px;
}

.pbx-line {
  height: 1px;
  margin: 4px 0;
  background: var(--line);
}
</style>
