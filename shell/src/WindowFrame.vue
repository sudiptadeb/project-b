<script setup>
import { watchEffect } from 'vue';
import { state } from './state.js';

// The window's own corner radius, as the engine reports it, becomes one CSS variable on
// the root. Everything that meets the window's curve derives its radius from it below,
// so the frame keeps one thickness around the corner.
watchEffect(() => {
  document.documentElement.style.setProperty('--window-radius', state.cornerRadius + 'px');
});
</script>

<template>
  <div class="frame" aria-hidden="true">
    <div v-drag class="edge top"></div>
    <div v-drag class="edge right"></div>
    <div v-drag class="edge bottom"></div>
    <div v-drag class="edge left"></div>
  </div>
</template>

<style>
/* The inner edge of the frame runs parallel to the window's edge, so its radius is the
   window's less the frame's width. 0 when the window has square corners. */
:root {
  --inner-radius: max(0px, calc(var(--window-radius, 0px) - var(--frame-width)));
}
</style>

<style scoped>
/* One frame around the whole window, the same in every layout. The shadow paints the frame
   colour outside a rounded box and nothing inside it, so the page still shows through. */
.frame {
  position: absolute;
  inset: 0;
  z-index: 2;
  overflow: hidden;
  pointer-events: none;
}

.frame::before {
  content: '';
  position: absolute;
  inset: var(--frame-width);
  border-radius: var(--inner-radius);
  box-shadow: 0 0 0 100vmax var(--frame);
}

.edge {
  position: absolute;
}

.top,
.bottom {
  left: 0;
  right: 0;
  height: var(--frame-width);
}

.left,
.right {
  top: 0;
  bottom: 0;
  width: var(--frame-width);
}

.top {
  top: 0;
}

.right {
  right: 0;
}

.bottom {
  bottom: 0;
}

.left {
  left: 0;
}
</style>
