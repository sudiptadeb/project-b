<script setup>
import { computed, nextTick, onUnmounted, ref, watch } from 'vue';
import { send } from '@/apps/settings/state.js';
import Dialog from '@/ui/Dialog.vue';
import RowMenu from '@/ui/RowMenu.vue';
import { pushEscape } from '@/ui/escape.js';
import ExtensionIcon from './ExtensionIcon.vue';
import InstallPrompt from './InstallPrompt.vue';
import { extensions } from './state.js';

// The extension manager: what is installed, the ways to add one from this machine, and the
// way to the store. Go owns all of it; the page shows it and sends the user's changes back.
const view = computed(() => extensions.value);
const installed = computed(() => view.value.installed || []);
const pinned = (id) => (view.value.pinned || []).includes(id);

// The one extension whose details are open. Escape closes them while nothing above takes it.
const open = ref('');
let popDetails = null;
watch(open, (id) => {
  popDetails?.();
  popDetails = id ? pushEscape(() => (open.value = '')) : null;
});
onUnmounted(() => popDetails?.());

// Each row's menu, by extension, so a right-click on the row opens it.
const menus = {};

function items(e) {
  return [
    { label: open.value === e.id ? 'Hide details' : 'Details', run: () => (open.value = open.value === e.id ? '' : e.id) },
    pinned(e.id)
      ? { label: 'Unpin from toolbar', run: () => send({ t: 'extensionPin', extension: e.id, on: false }) }
      : { label: 'Pin to toolbar', run: () => send({ t: 'extensionPin', extension: e.id, on: true }) },
    { separator: true },
    { label: 'Remove…', danger: true, run: () => (removing.value = e) },
  ];
}

// The extension being asked about before it is removed, with its data.
const removing = ref(null);
const list = ref(null);

// Removing takes the row away once Go says so; focus then goes to the row that took its
// place, or to Load unpacked… when none is left.
function remove() {
  const e = removing.value;
  const at = installed.value.findIndex((x) => x.id === e.id);
  removing.value = null;
  send({ t: 'extensionRemove', extension: e.id });
  const stop = watch(installed, async (now) => {
    if (now.some((x) => x.id === e.id)) return;
    stop();
    if (open.value === e.id) open.value = '';
    await nextTick();
    const buttons = list.value?.querySelectorAll('.ui-row-menu > .icon-button') || [];
    (buttons[Math.min(at, buttons.length - 1)] || document.getElementById('pbx-load-unpacked'))?.focus();
  });
}
</script>

<template>
  <div>
    <section aria-labelledby="pbx-installed">
      <div class="pbx-heading">
        <h2 id="pbx-installed">Installed</h2>
        <button id="pbx-load-unpacked" type="button" class="text-button" @click="send({ t: 'extensionLoadUnpacked' })">Load unpacked</button>
        <button type="button" class="text-button" @click="send({ t: 'extensionInstallPackage' })">Install package…</button>
      </div>
      <p v-if="view.error" class="pbx-message" role="alert">{{ view.error }}</p>
      <div ref="list" class="card">
        <div v-if="!installed.length" class="row">
          <span class="label pbx-muted">No extensions installed</span>
        </div>
        <template v-for="e in installed" :key="e.id">
          <div
            class="row pbx-extension"
            :class="{ 'pbx-off': !e.enabled }"
            :aria-label="e.name"
            role="group"
            @contextmenu.prevent="menus[e.id]?.open()"
          >
            <ExtensionIcon :src="e.icon" :name="e.name" :size="24" />
            <span class="label">
              <span class="pbx-name" :title="e.name">{{ e.name }}</span>
              <span class="secondary pbx-line" :title="e.description">{{ e.version }}<template v-if="e.description"> · {{ e.description }}</template></span>
            </span>
            <input
              type="checkbox"
              role="switch"
              class="switch"
              :aria-label="`${e.name} on`"
              :checked="e.enabled"
              :aria-checked="String(e.enabled)"
              @change="send({ t: 'extensionSetEnabled', extension: e.id, on: $event.target.checked })"
            />
            <RowMenu :ref="(m) => (menus[e.id] = m)" :label="`Actions for ${e.name}`" :items="items(e)" />
          </div>
          <dl v-if="open === e.id" class="pbx-details" :aria-label="`Details of ${e.name}`">
            <dt>ID</dt>
            <dd>{{ e.id }}</dd>
            <dt>Version</dt>
            <dd>{{ e.version }}</dd>
            <dt>Source</dt>
            <dd>{{ e.unpacked ? `Unpacked, from ${e.path}` : 'Package' }}</dd>
            <dt>Permissions</dt>
            <dd>
              <ul v-if="e.permissions && e.permissions.length">
                <li v-for="p in e.permissions" :key="p">{{ p }}</li>
              </ul>
              <template v-else>None</template>
            </dd>
            <dt>Site access</dt>
            <dd>
              <ul v-if="e.hosts && e.hosts.length">
                <li v-for="h in e.hosts" :key="h">{{ h }}</li>
              </ul>
              <template v-else>None</template>
            </dd>
            <dt>Toolbar button</dt>
            <dd>{{ e.hasAction ? 'Yes' : 'No' }}</dd>
          </dl>
        </template>
      </div>
    </section>

    <!-- The store is a website: an extension is installed from its own page. -->
    <section aria-labelledby="pbx-store">
      <h2 id="pbx-store">Store</h2>
      <div class="card">
        <div class="row">
          <span class="label">Chrome Web Store</span>
          <button type="button" class="text-button" @click="send({ t: 'extensionOpenStore' })">Open</button>
        </div>
      </div>
    </section>

    <Dialog v-if="removing" :title="`Remove ${removing.name}?`" @close="removing = null">
      <p>Its data is deleted too.</p>
      <template #buttons>
        <button type="button" class="text-button" @click="removing = null">Cancel</button>
        <button type="button" class="text-button danger" @click="remove">Remove</button>
      </template>
    </Dialog>
    <InstallPrompt v-if="view.prompt" :prompt="view.prompt" :send="send" />
  </div>
</template>

<style scoped>
/* Like Chrome's developer bar: the ways to add an extension sit beside its heading. */
.pbx-heading {
  display: flex;
  align-items: center;
  gap: 8px;
}

.pbx-heading h2 {
  flex: 1;
}

.pbx-message {
  margin: 8px 0 0;
  color: var(--danger);
  font-size: 13px;
  font-weight: 400;
}

.pbx-muted {
  color: var(--muted);
  font-weight: 400;
}

.pbx-extension {
  gap: 12px;
}

.pbx-name,
.pbx-line {
  display: block;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

/* A switched-off extension's icon and name fade, as its button does on the toolbar. */
.pbx-off :deep(.pbx-icon),
.pbx-off .pbx-name {
  opacity: 0.5;
}

/* The details open under their row, on the tint an open row has, lined up with the name. */
.pbx-details {
  display: grid;
  grid-template-columns: 120px 1fr;
  gap: 4px 16px;
  margin: 0;
  padding: 8px 16px 12px 52px;
  border-top: 1px solid var(--line);
  background: var(--accent-tint);
  font-size: 13px;
}

.pbx-details dt {
  color: var(--muted);
}

.pbx-details dd {
  min-width: 0;
  margin: 0;
  overflow-wrap: anywhere;
}

.pbx-details ul {
  margin: 0;
  padding: 0;
  list-style: none;
}

.pbx-details + .row {
  border-top: 1px solid var(--line);
}

/* The card's corners stay round when the last row's details are open. */
.pbx-details:last-child {
  border-radius: 0 0 8px 8px;
}
</style>
