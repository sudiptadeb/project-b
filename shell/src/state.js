// The shell's copy of the state. The managers own the truth: this is only ever replaced
// by what they send, and a new shell starts from a snapshot.

import { reactive } from 'vue';
import { listen, send } from './bridge.js';

export const state = reactive({
  engine: '',
  tab: { id: 0, url: '', title: '', loading: false, blocked: false, crashed: false, error: '' },
  // Where the native window controls are. Zero size when there are none.
  controls: { x: 0, y: 0, w: 0, h: 0 },
});

let onSuggestions = () => {};
let onFocusAddressBar = () => {};

export function setHandlers(handlers) {
  onSuggestions = handlers.suggestions;
  onFocusAddressBar = handlers.focusAddressBar;
}

export function connect() {
  listen(
    (m) => {
      switch (m.t) {
        case 'snapshot':
          state.engine = m.engine;
          state.tab = m.tab;
          state.controls = m.controls;
          break;
        case 'tab':
          state.tab = m.tab;
          break;
        case 'controls':
          state.controls = m.controls;
          break;
        case 'suggestions':
          onSuggestions(m.id, m.items || []);
          break;
        case 'focusAddressBar':
          onFocusAddressBar();
          break;
      }
    },
    () => send({ t: 'ready' }),
  );
}
