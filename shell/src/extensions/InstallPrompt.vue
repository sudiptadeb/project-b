<script setup>
import { onMounted, onUnmounted, ref } from 'vue';
import ExtensionIcon from './ExtensionIcon.vue';

// The engine asking whether to install an extension, or with prompt.remove whether to
// remove one a store's page asked to remove, as a modal dialog. The shell shows
// it over the page of a store that asked, and the settings page over itself, where a
// package or a folder is picked. The answer goes to Go through the page's own send, and
// Go takes the prompt away.
const props = defineProps({ prompt: Object, send: Function });

const dialog = ref(null);

function answer(on) {
  props.send({ t: 'extensionAnswerPrompt', on });
}

// Escape answers Cancel, and only that: the settings page leaves an Escape it sees
// prevented alone, so this one takes it first, before it reaches the page.
function onKey(e) {
  if (e.key !== 'Escape') return;
  e.preventDefault();
  answer(false);
}

onMounted(() => {
  dialog.value.showModal();
  window.addEventListener('keydown', onKey, true);
});
onUnmounted(() => window.removeEventListener('keydown', onKey, true));
</script>

<template>
  <dialog v-if="prompt.remove" ref="dialog" class="pbx-prompt" :aria-label="`Remove ${prompt.name}`">
    <div class="pbx-prompt-head">
      <ExtensionIcon :src="prompt.icon" :name="prompt.name" :size="32" />
      <h2 class="pbx-prompt-title">Remove "{{ prompt.name }}"?</h2>
    </div>
    <p class="pbx-prompt-text">Its data is deleted too.</p>
    <div class="pbx-prompt-buttons">
      <button type="button" class="text-button" @click="answer(false)">Cancel</button>
      <button type="button" class="text-button primary" @click="answer(true)">Remove</button>
    </div>
  </dialog>
  <dialog v-else ref="dialog" class="pbx-prompt" :aria-label="`Add ${prompt.name}`">
    <div class="pbx-prompt-head">
      <ExtensionIcon :src="prompt.icon" :name="prompt.name" :size="32" />
      <div>
        <h2 class="pbx-prompt-title">Add "{{ prompt.name }}"?</h2>
        <div class="pbx-prompt-version">Version {{ prompt.version }}</div>
      </div>
    </div>
    <template v-if="prompt.permissions && prompt.permissions.length">
      <p class="pbx-prompt-label">It can</p>
      <ul class="pbx-prompt-list">
        <li v-for="p in prompt.permissions" :key="p">{{ p }}</li>
      </ul>
    </template>
    <p v-else class="pbx-prompt-text">It asks for no permissions.</p>
    <div class="pbx-prompt-buttons">
      <button type="button" class="text-button" @click="answer(false)">Cancel</button>
      <button type="button" class="text-button primary" @click="answer(true)">Add extension</button>
    </div>
  </dialog>
</template>

<style scoped>
.pbx-prompt {
  width: 360px;
  max-width: calc(100% - 32px);
  padding: 20px;
  box-sizing: border-box;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  color: var(--text);
  box-shadow: var(--shadow-float);
}

/* The page behind fades toward the bar's colour. */
.pbx-prompt::backdrop {
  background: color-mix(in srgb, var(--bar) 70%, transparent);
}

.pbx-prompt-head {
  display: flex;
  align-items: center;
  gap: 12px;
}

.pbx-prompt-title {
  margin: 0;
  color: var(--text);
  font-size: 15px;
  font-weight: 600;
}

.pbx-prompt-version {
  color: var(--muted);
  font-size: 12px;
}

.pbx-prompt-text {
  margin: 16px 0 0;
}

.pbx-prompt-label {
  margin: 16px 0 0;
  color: var(--muted);
  font-size: 13px;
  font-weight: 500;
}

/* The permissions as the manifest names them, in a hairline box that scrolls past six. */
.pbx-prompt-list {
  max-height: 168px;
  margin: 6px 0 0;
  padding: 4px 0;
  overflow: auto;
  border: 1px solid var(--line);
  border-radius: 6px;
  list-style: none;
  font-size: 13px;
}

.pbx-prompt-list li {
  padding: 4px 12px;
  overflow-wrap: anywhere;
}

.pbx-prompt-buttons {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 20px;
}
</style>
