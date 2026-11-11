<script setup>
import { onMounted, onUnmounted, ref } from 'vue';
import { send } from '@/apps/settings/state.js';
import ExtensionIcon from './ExtensionIcon.vue';

// The engine asking whether to install an extension, as a modal dialog over the settings
// page. The answer goes to Go, which takes the prompt away.
defineProps({ prompt: Object });

const dialog = ref(null);

function answer(on) {
  send({ t: 'extensionAnswerPrompt', on });
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
  <dialog ref="dialog" class="pbx-prompt" :aria-label="`Add ${prompt.name}`">
    <div class="pbx-prompt-head">
      <ExtensionIcon :src="prompt.icon" :name="prompt.name" :size="32" />
      <div>
        <h2 class="pbx-prompt-title">Add "{{ prompt.name }}"?</h2>
        <div class="pbx-prompt-version">Version {{ prompt.version }}</div>
      </div>
    </div>
    <template v-if="prompt.permissions && prompt.permissions.length">
      <p class="pbx-prompt-text">It can:</p>
      <ul class="pbx-prompt-list">
        <li v-for="p in prompt.permissions" :key="p">{{ p }}</li>
      </ul>
    </template>
    <p v-else class="pbx-prompt-text">It asks for no permissions.</p>
    <div class="pbx-prompt-buttons">
      <button type="button" class="text-button" @click="answer(false)">Cancel</button>
      <button type="button" class="text-button pbx-primary" @click="answer(true)">Add extension</button>
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
  box-shadow: 0 8px 28px rgba(16, 24, 40, 0.14), 0 1px 3px rgba(16, 24, 40, 0.08);
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

.pbx-prompt-list {
  margin: 6px 0 0;
  padding-left: 20px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
}

.pbx-prompt-buttons {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 20px;
}

.pbx-primary,
.pbx-primary:hover:not(:disabled) {
  border-color: var(--accent);
  background: var(--accent);
  color: var(--surface);
}
</style>
