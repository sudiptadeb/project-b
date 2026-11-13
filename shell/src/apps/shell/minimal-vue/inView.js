import { onBeforeUnmount, onMounted, watch } from 'vue';
import { state } from '@/sdk/vue.js';

// keepActiveInView scrolls list, a ref to a list of tabs that scrolls sideways when
// across is true and down when not, so the active tab shows whole: when another becomes
// active, when tabs come or go, and when the list is resized.
export function keepActiveInView(list, across) {
  function reveal() {
    const box = list.value;
    const tab = box.querySelector('[aria-selected="true"]');
    // Before the snapshot there is no tab to bring into view.
    if (!tab) return;
    const [start, size, scroll, view] = across
      ? [tab.offsetLeft, tab.offsetWidth, box.scrollLeft, box.clientWidth]
      : [tab.offsetTop, tab.offsetHeight, box.scrollTop, box.clientHeight];
    let to = scroll;
    if (start < scroll) to = start;
    else if (start + size > scroll + view) to = start + size - view;
    if (across) box.scrollLeft = to;
    else box.scrollTop = to;
  }

  const observer = new ResizeObserver(reveal);
  onMounted(() => observer.observe(list.value));
  onBeforeUnmount(() => observer.disconnect());
  watch(() => [state.active, state.tabs.length], reveal, { flush: 'post' });
}
