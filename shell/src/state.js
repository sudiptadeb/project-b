// The shell's copy of the state. The managers own the truth: this is only ever replaced
// by what they send, and a new shell starts from a snapshot.

import { reactive } from 'vue';
import { listen, send } from './bridge.js';

export const state = reactive({
  engine: '',
  tab: { id: 0, url: '', title: '', loading: false, blocked: false, crashed: false, error: '' },
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
          break;
        case 'tab':
          state.tab = m.tab;
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
