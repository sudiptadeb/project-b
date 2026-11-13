<script setup>
import { watch, watchEffect } from 'vue';
import PaneTree from './PaneTree.vue';
import PromptLine from './PromptLine.vue';
import TopStrip from './TopStrip.vue';
import { connect, state } from '@/kit/shell.js';
import { useTheme } from '@/theme.js';
import { follow, load, shown } from './panes.js';
import { prompt } from './prompt.js';

// A terminal's window: a strip of groups on top, the shown group's panes in the body, and
// a prompt line at the foot that is the active pane's address bar.
useTheme(state);
watchEffect(() => {
  document.documentElement.style.setProperty('--frame-width', state.frameWidth + 'px');
});

connect({
  snapshot: () => load(state.data),
  tabs: follow,
  suggestions: (m) => prompt.suggestions(m.id, m.items || []),
  focusAddressBar: () => prompt.focus(),
});

// A pane that becomes active with nothing in it wants an address; one with a page wants
// the keyboard in its page.
watch(
  () => state.active,
  () => {
    if (state.active !== 0) prompt.activated();
  },
);
</script>

<!-- The frame's bands are painted as the window paints them, and nothing is painted under
     the panes, so their pages show through. -->
<template>
  <div class="app">
    <div v-drag class="band across"></div>
    <div class="middle">
      <div class="band side"></div>
      <div class="inner">
        <TopStrip />
        <div class="body">
          <PaneTree v-if="shown" :node="shown" />
        </div>
        <PromptLine />
      </div>
      <div class="band side"></div>
    </div>
    <div class="band across"></div>
  </div>
</template>

<style>
.app {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  font-family: ui-monospace, 'SF Mono', Menlo, monospace;
  font-size: 12px;
  line-height: 16px;
}

.band {
  flex: none;
  background: var(--frame);
}

.band.across {
  height: var(--frame-width);
}

.band.side {
  width: var(--frame-width);
}

.middle {
  display: flex;
  flex: 1;
  min-height: 0;
}

.inner {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.body {
  display: flex;
  flex: 1;
  min-height: 0;
}
</style>
