<script setup>
import { computed } from 'vue';
// A picture of the browser as the style step's choices make it: tabs on the side, on top,
// or Glide, or a phone with its address bar on top or at the foot. The desktop's three are
// drawn as those shells draw themselves (shell/src/apps/shell/standard and glide): the
// address bar and its buttons, New Tab, tabs with their titles, the workspace switcher or
// Glide's Space and its dots, and a page. It is drawn, not a picture, in the page's own
// tones, so it takes the colour and the theme as the page does; sized by its own width, so
// a choice's small one is the same picture as the large one. The window's buttons are the
// platform's: the Mac's lights at the left, or on Linux and Windows a small copy of
// ui/WindowButtons.vue at the right end of the top row. New Tab shows the keymap's key for
// it, as the platform writes it, newTab, and none when it has none.
const props = defineProps({ layout: String, top: Boolean, platform: { type: String, default: 'mac' }, newTab: String });
const mac = computed(() => props.platform === 'mac');
const tabs = ['Morning news', 'Project notes', 'Lentil soup', 'Inbox'];
const rows = [
  ['Team update', 'Here is what moved this week…'],
  ['Your order shipped', 'It arrives on Thursday.'],
  ['Weekend plans', 'Are we still on for Saturday?'],
  ['Invoice 2041', 'Due at the end of the month.'],
];
</script>

<template>
  <div v-if="layout === 'phone'" class="phone" aria-hidden="true">
    <div class="screen">
      <div v-if="top" class="address"><i></i></div>
      <div class="page plain"><u></u><u></u><u class="short"></u></div>
      <div v-if="!top" class="address"><i></i></div>
    </div>
  </div>
  <div v-else class="window" :class="layout" aria-hidden="true">
    <div class="w">
      <template v-if="layout === 'top'">
        <div class="strip">
          <span v-if="mac" class="lights"><i></i><i></i><i></i></span>
          <span class="ws"><b class="d1"></b><b class="d2"></b>All workspaces</span>
          <span v-for="(t, i) in tabs" :key="t" class="htab" :class="{ on: i === 3 }">{{ t }}</span>
          <span class="plus">+</span>
          <span v-if="!mac" class="window-buttons"><svg viewBox="0 0 16 16"><path d="M3.5 8h9"/></svg><svg viewBox="0 0 16 16"><rect x="3.75" y="3.75" width="8.5" height="8.5" rx="1"/></svg><svg viewBox="0 0 16 16"><path d="M4 4l8 8M12 4l-8 8"/></svg></span>
        </div>
        <div class="bar">
          <span class="icon">‹</span><span class="icon">›</span><span class="icon">↻</span>
          <span class="field"><span class="slot"></span>mail.example.com</span>
        </div>
      </template>
      <div v-else-if="layout === 'side'" class="bar">
        <span v-if="mac" class="lights"><i></i><i></i><i></i></span>
        <span class="icon">▯</span><span class="icon">‹</span><span class="icon">›</span><span class="icon">↻</span>
        <span class="field"><span class="slot"></span>mail.example.com</span>
        <span v-if="!mac" class="window-buttons"><svg viewBox="0 0 16 16"><path d="M3.5 8h9"/></svg><svg viewBox="0 0 16 16"><rect x="3.75" y="3.75" width="8.5" height="8.5" rx="1"/></svg><svg viewBox="0 0 16 16"><path d="M4 4l8 8M12 4l-8 8"/></svg></span>
      </div>
      <div class="body">
        <div v-if="layout === 'side'" class="rail">
          <span class="vtab new">+ New Tab<em v-if="newTab">{{ newTab }}</em></span>
          <span v-for="(t, i) in tabs" :key="t" class="vtab" :class="{ on: i === 3 }">{{ t }}</span>
          <span class="ws foot"><b class="d1"></b><b class="d2"></b>All workspaces</span>
        </div>
        <div v-if="layout === 'glide'" class="rail glide-rail">
          <span v-if="mac" class="lights"><i></i><i></i><i></i></span>
          <span class="pill">mail.example.com</span>
          <span class="space"><b class="d2"></b>Work</span>
          <span class="vtab new">+ New Tab</span>
          <span class="vtab on"><b class="fav">M</b>Inbox</span>
          <span class="dots"><b class="d1"></b><b class="d2 on"></b></span>
        </div>
        <div class="page">
          <template v-if="layout === 'glide'"><span v-if="!mac" class="window-buttons"><svg viewBox="0 0 16 16"><path d="M3.5 8h9"/></svg><svg viewBox="0 0 16 16"><rect x="3.75" y="3.75" width="8.5" height="8.5" rx="1"/></svg><svg viewBox="0 0 16 16"><path d="M4 4l8 8M12 4l-8 8"/></svg></span></template>
          <small>INBOX</small>
          <strong>Good morning</strong>
          <span v-for="r in rows" :key="r[0]" class="row"><i></i><span><b>{{ r[0] }}</b>{{ r[1] }}</span></span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* The desktop's picture, drawn on a 640 by 400 grid: --u is one unit of it at the
   picture's width. */
.window {
  container-type: inline-size;
  width: 100%;
  aspect-ratio: 16 / 10;
  border: 4px solid var(--frame);
  border-radius: 12px;
  background: var(--bar);
  overflow: hidden;
  transition: border-color var(--motion) var(--ease), background var(--motion) var(--ease);
}

.w {
  --u: calc(100cqw / 640);
  display: flex;
  flex-direction: column;
  height: 100%;
  font-size: calc(var(--u) * 6.5);
  line-height: 1.3;
  color: var(--text);
  white-space: nowrap;
}

.lights {
  display: flex;
  gap: calc(var(--u) * 4);
  flex: none;
  margin-right: calc(var(--u) * 8);
}

.lights i {
  width: calc(var(--u) * 6);
  height: calc(var(--u) * 6);
  border-radius: 50%;
  background: #ec6a5e;
}

.lights i:nth-child(2) {
  background: #f4bf4f;
}

.lights i:nth-child(3) {
  background: #61c554;
}

/* The window's own buttons, as ui/WindowButtons.vue draws them, at the picture's size. */
.window-buttons {
  display: flex;
  gap: calc(var(--u) * 4);
  flex: none;
  margin-left: auto;
  color: var(--icon);
}

.window-buttons svg {
  width: calc(var(--u) * 10);
  height: calc(var(--u) * 10);
  fill: none;
  stroke: currentColor;
  stroke-width: 1.5;
}

/* Glide's frame has no place for them yet: the page area's top right. */
.page .window-buttons {
  position: absolute;
  top: calc(var(--u) * 6);
  right: calc(var(--u) * 6);
}

.strip,
.bar {
  display: flex;
  align-items: center;
  gap: calc(var(--u) * 6);
  height: calc(var(--u) * 24);
  padding: 0 calc(var(--u) * 8);
  flex: none;
}

.icon {
  width: calc(var(--u) * 10);
  text-align: center;
  color: var(--muted);
  font-size: calc(var(--u) * 9);
}

.field {
  flex: 1;
  display: flex;
  align-items: center;
  gap: calc(var(--u) * 6);
  height: calc(var(--u) * 16);
  border-radius: calc(var(--u) * 4);
  background: var(--surface);
  border: 1px solid var(--line);
  overflow: hidden;
}

.slot {
  width: calc(var(--u) * 16);
  height: 100%;
  background: var(--accent-tint);
  border-right: 1px solid var(--line);
}

.ws {
  display: flex;
  align-items: center;
  gap: calc(var(--u) * 3);
  color: var(--text);
}

.d1,
.d2 {
  width: calc(var(--u) * 5);
  height: calc(var(--u) * 5);
  border-radius: 50%;
  background: #2e9d5b;
  flex: none;
}

.d2 {
  background: #3470d0;
}

.ws .d2 {
  margin-left: calc(var(--u) * -2);
  margin-right: calc(var(--u) * 2);
}

.htab {
  width: calc(var(--u) * 72);
  padding: calc(var(--u) * 3) calc(var(--u) * 6);
  border-radius: calc(var(--u) * 4);
  overflow: hidden;
  text-overflow: ellipsis;
}

.htab.on,
.vtab.on {
  background: var(--accent-soft);
}

.plus {
  color: var(--muted);
}

.body {
  flex: 1;
  display: flex;
  min-height: 0;
}

.rail {
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 2);
  width: calc(var(--u) * 112);
  padding: calc(var(--u) * 6);
  flex: none;
}

.vtab {
  display: flex;
  align-items: center;
  gap: calc(var(--u) * 4);
  padding: calc(var(--u) * 3) calc(var(--u) * 6);
  border-radius: calc(var(--u) * 4);
  overflow: hidden;
}

.vtab.new em {
  margin-left: auto;
  font-style: normal;
  color: var(--muted);
}

.foot {
  margin-top: auto;
  padding: calc(var(--u) * 3) calc(var(--u) * 4);
}

.glide-rail {
  padding-top: calc(var(--u) * 8);
}

.glide-rail .lights {
  margin-bottom: calc(var(--u) * 6);
}

.pill {
  padding: calc(var(--u) * 5) calc(var(--u) * 6);
  border-radius: calc(var(--u) * 5);
  background: color-mix(in srgb, var(--surface) 45%, transparent);
  margin-bottom: calc(var(--u) * 6);
}

.space {
  display: flex;
  align-items: center;
  gap: calc(var(--u) * 4);
  font-weight: 600;
  padding: calc(var(--u) * 3) calc(var(--u) * 4);
}

.glide-rail .vtab.on {
  background: var(--surface);
}

.fav {
  display: grid;
  place-items: center;
  width: calc(var(--u) * 8);
  height: calc(var(--u) * 8);
  border-radius: calc(var(--u) * 2);
  background: #3470d0;
  color: #fff;
  font-size: calc(var(--u) * 5);
}

.dots {
  display: flex;
  justify-content: center;
  gap: calc(var(--u) * 8);
  margin-top: auto;
}

.dots b.on {
  outline: calc(var(--u) * 3) solid var(--surface);
}

.page {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: calc(var(--u) * 18) calc(var(--u) * 48);
  background: var(--surface);
  overflow: hidden;
  position: relative;
}

.glide .page {
  margin: calc(var(--u) * 4) calc(var(--u) * 4) calc(var(--u) * 4) 0;
  border-radius: calc(var(--u) * 8);
  box-shadow: 0 0 0 1px var(--line);
}

.page small {
  font-size: calc(var(--u) * 5);
  color: var(--muted);
  letter-spacing: 0.04em;
}

.page strong {
  font-size: calc(var(--u) * 13);
  margin: calc(var(--u) * 2) 0 calc(var(--u) * 8);
}

.page .row {
  display: flex;
  gap: calc(var(--u) * 6);
  padding: calc(var(--u) * 5) 0;
  border-top: 1px solid var(--line);
  color: var(--muted);
}

.page .row i {
  width: calc(var(--u) * 14);
  height: calc(var(--u) * 14);
  border-radius: 50%;
  background: var(--accent-tint);
  flex: none;
}

.page .row b {
  display: block;
  color: var(--text);
  font-weight: 600;
}

/* The phone's picture keeps lines for a page. */
.page.plain {
  gap: 8px;
  padding: 16px;
}

.page.plain u {
  display: block;
  height: 8px;
  border-radius: 4px;
  background: var(--accent-tint);
}

.page.plain u:first-child {
  width: 40%;
  height: 12px;
  background: var(--accent-soft);
}

.page.plain u.short {
  width: 60%;
}

.phone {
  width: 168px;
  margin: 0 auto;
  padding: 8px;
  border-radius: 24px;
  background: var(--frame);
  transition: background var(--motion) var(--ease);
}

.screen {
  display: flex;
  flex-direction: column;
  aspect-ratio: 9 / 18;
  border-radius: 16px;
  background: var(--bar);
  overflow: hidden;
}

.screen .page {
  border-radius: 0;
  box-shadow: none;
}

.address {
  flex: none;
  padding: 8px;
}

.address i {
  display: block;
  height: 20px;
  border-radius: 10px;
  background: var(--surface);
  border: 1px solid var(--line);
}
</style>
