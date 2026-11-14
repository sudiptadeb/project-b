<script setup>
import { watch } from 'vue';
import PaneTree from './PaneTree.vue';
import PromptLine from './PromptLine.vue';
import TopStrip from './TopStrip.vue';
import { connect, setFrame, state } from '@/sdk/vue.js';
import { focusToward, follow, heirAsked, load, resizeToward, shown, split, zoom, zoomed } from './panes.js';
import { prompt } from './prompt.js';

// A terminal's window: a strip of groups on top, the shown group's panes in the body, and
// a prompt line at the foot that is the active pane's address bar, inside the window's
// 4px band.
setFrame(4);

// Its own actions, as the registry lists them, which Go sends while this shell is on: the
// splits, as the panes' buttons do; moving to the pane beside, by Ctrl+Cmd and an arrow;
// moving a divider, with Shift too; and zooming the active pane.
const actions = {
  'terminal.splitRight': () => split(state.active, 'row'),
  'terminal.splitDown': () => split(state.active, 'column'),
  'terminal.focusLeft': () => focusToward('left'),
  'terminal.focusRight': () => focusToward('right'),
  'terminal.focusUp': () => focusToward('up'),
  'terminal.focusDown': () => focusToward('down'),
  'terminal.resizeLeft': () => resizeToward('left'),
  'terminal.resizeRight': () => resizeToward('right'),
  'terminal.resizeUp': () => resizeToward('up'),
  'terminal.resizeDown': () => resizeToward('down'),
  'terminal.zoom': () => zoom(),
};

connect({
  snapshot: () => load(state.data),
  tabs: follow,
  suggestions: (m) => prompt.suggestions(m.id, m.items || []),
  focusAddressBar: () => prompt.focus(),
  action: (m) => actions[m.id](),
});

// A pane that becomes active with nothing in it wants an address; one with a page wants
// the keyboard in its page. Not while Go is being asked for the heir of a pane that closed.
watch(
  () => state.active,
  () => {
    if (state.active !== 0 && !heirAsked.value) prompt.activated();
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
          <PaneTree v-if="shown" :node="zoomed ? { tab: zoomed } : shown" />
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
