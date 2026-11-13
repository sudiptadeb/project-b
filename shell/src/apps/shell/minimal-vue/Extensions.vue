<script setup>
import { computed, onBeforeUnmount, ref } from 'vue';
import { manageExtensions, pinExtension, pressExtension } from '@/sdk/extensions.js';
import Prompt from './Prompt.vue';

// The extensions' buttons, plainly, from extensions, the SDK's state.extensions: a button
// for each pinned one, then Extensions, whose menu lists them all to open, pin or unpin,
// and Manage extensions; and the install prompt over the window. The classes and labels
// are what tools/drive_extensions.py finds them by (docs/shells.md).
const props = defineProps({ extensions: Object });
const pinned = computed(() => props.extensions.buttons.filter((b) => b.pinned));
const menu = ref(false);
// The Extensions button: a popup opened from the menu opens under it.
const opener = ref(null);

function open(id) {
  menu.value = false;
  pressExtension(id, opener.value);
}

function manage() {
  menu.value = false;
  manageExtensions();
}

function escape(e) {
  if (e.key === 'Escape') menu.value = false;
}
addEventListener('keydown', escape);
onBeforeUnmount(() => removeEventListener('keydown', escape));
</script>

<template>
  <template v-if="extensions">
    <button
      v-for="b in pinned"
      :key="b.id"
      class="btn pbx-action"
      :class="{ 'pbx-open': extensions.popup === b.id }"
      :data-extension="b.id"
      :disabled="!b.enabled"
      :aria-label="b.title || b.name"
      @click="pressExtension(b.id, $event.currentTarget)"
    >
      <img v-if="b.icon" :src="b.icon" alt="" width="16" height="16" />
      <template v-else>{{ b.name.slice(0, 1) }}</template>
      <span v-if="b.badge" class="pbx-badge">{{ b.badge.slice(0, 4) }}</span>
    </button>
    <span class="anchor">
      <button
        ref="opener"
        class="btn"
        :class="{ 'pbx-open': menu }"
        aria-label="Extensions"
        :aria-expanded="menu"
        @click="menu = !menu"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round"><path d="M20.5 11H19V7a2 2 0 0 0-2-2h-4V3.5a2.5 2.5 0 0 0-5 0V5H4a2 2 0 0 0-2 2v3.8h1.5a2.7 2.7 0 0 1 0 5.4H2V20a2 2 0 0 0 2 2h3.8v-1.5a2.7 2.7 0 0 1 5.4 0V22H17a2 2 0 0 0 2-2v-4h1.5a2.5 2.5 0 0 0 0-5Z" /></svg>
      </button>
      <div v-if="menu" v-shell class="pbx-menu" role="group" aria-label="Extensions menu">
        <div v-for="b in extensions.buttons" :key="b.id" class="row">
          <button class="name" :disabled="!b.enabled" @click="open(b.id)">{{ b.name }}</button>
          <button :aria-label="`${b.pinned ? 'Unpin' : 'Pin'} ${b.name}`" @click="pinExtension(b.id, !b.pinned)">{{ b.pinned ? 'Unpin' : 'Pin' }}</button>
        </div>
        <button class="pbx-manage" @click="manage">Manage extensions</button>
      </div>
    </span>
    <Prompt v-if="extensions.prompt" :prompt="extensions.prompt" />
  </template>
</template>

<style scoped>
.pbx-action {
  position: relative;
}

.pbx-open,
.pbx-open:hover:not(:disabled) {
  background: var(--accent-soft);
}

.pbx-badge {
  position: absolute;
  right: 1px;
  bottom: 2px;
  min-width: 8px;
  height: 12px;
  padding: 0 2px;
  background: var(--accent);
  color: var(--on-accent);
  font-size: 9px;
  font-weight: 600;
  line-height: 12px;
  pointer-events: none;
}

/* The menu hangs from the toolbar's foot, over the page, at its right. */
.anchor {
  position: relative;
  align-self: stretch;
  display: flex;
  align-items: center;
}

.pbx-menu {
  position: absolute;
  top: 100%;
  right: 0;
  width: 240px;
  background: var(--surface);
  border: 1px solid var(--line);
}

.pbx-menu button {
  height: 32px;
  padding: 0 10px;
  border: 0;
  background: none;
  text-align: left;
  white-space: nowrap;
}

.pbx-menu button:hover:not(:disabled) {
  background: var(--accent-tint);
}

.pbx-menu button:disabled {
  color: var(--muted);
}

.row {
  display: flex;
}

.name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.row button:not(.name) {
  flex: none;
  color: var(--muted);
}

.pbx-menu .pbx-manage {
  width: 100%;
  border-top: 1px solid var(--line);
}
</style>
