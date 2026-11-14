<script setup>
import { ref } from 'vue';
import Pane from './Pane.vue';
import { save } from './panes.js';

// A node of a group's tree: a pane, or a split's parts with a divider between each two.
// Dragging a divider moves the share between the parts either side of it.
const props = defineProps({ node: { type: Object, required: true } });

// No part is dragged smaller than this, in pixels, so no page vanishes.
const smallest = 80;
const box = ref(null);
let drag = null;

function start(e, i) {
  e.preventDefault();
  e.currentTarget.setPointerCapture(e.pointerId);
  const r = box.value.getBoundingClientRect();
  const row = props.node.dir === 'row';
  // The room the parts share, without the dividers.
  const room = (row ? r.width : r.height) - (props.node.parts.length - 1);
  drag = { i, row, room, from: row ? e.clientX : e.clientY, a: props.node.sizes[i], b: props.node.sizes[i + 1] };
}

function move(e) {
  if (!drag) return;
  const { i, row, room, from, a, b } = drag;
  const least = smallest / room;
  const delta = Math.min(Math.max(((row ? e.clientX : e.clientY) - from) / room, least - a), b - least);
  props.node.sizes[i] = a + delta;
  props.node.sizes[i + 1] = b - delta;
}

function end() {
  if (!drag) return;
  drag = null;
  save();
}
</script>

<template>
  <Pane v-if="!node.parts" :id="node.tab" />
  <div v-else ref="box" class="split" :class="node.dir">
    <template v-for="(part, i) in node.parts" :key="part.tab ?? `split-${i}`">
      <div
        v-if="i > 0"
        class="divider"
      >
        <div
          v-shell
          class="grab"
          role="separator"
          :aria-orientation="node.dir === 'row' ? 'vertical' : 'horizontal'"
          aria-label="Resize"
          @pointerdown="start($event, i - 1)"
          @pointermove="move"
          @pointerup="end"
          @pointercancel="end"
        ></div>
      </div>
      <div class="part" :style="{ flex: `${node.sizes[i]} 1 0` }">
        <PaneTree :node="part" />
      </div>
    </template>
  </div>
</template>

<style scoped>
.split {
  display: flex;
  width: 100%;
  height: 100%;
}

.split.column {
  flex-direction: column;
}

.part {
  display: flex;
  min-width: 0;
  min-height: 0;
}

/* A hairline between two parts, with room either side of it to take hold of. */
.divider {
  position: relative;
  flex: none;
  background: var(--line);
}

.row > .divider {
  width: 1px;
}

.column > .divider {
  height: 1px;
}

.grab {
  position: absolute;
  z-index: 1;
}

.row > .divider > .grab {
  inset: 0 -4px;
  cursor: col-resize;
}

.column > .divider > .grab {
  inset: -4px 0;
  cursor: row-resize;
}
</style>
