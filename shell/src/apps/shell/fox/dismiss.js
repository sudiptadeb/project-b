import { onBeforeUnmount, watch } from 'vue';

// closesOutside closes a floating menu, open while open.value is true, on Escape, on a
// press anywhere in the shell outside root, and when the page takes the keyboard from the
// shell: a press on the page never reaches the shell, so that is how one there closes it.
export function closesOutside(open, root) {
  const onKey = (e) => {
    if (e.key === 'Escape') open.value = false;
  };
  const onPress = (e) => {
    if (!root.value.contains(e.target)) open.value = false;
  };
  const onBlur = () => {
    open.value = false;
  };
  const listen = (on) => {
    const method = on ? 'addEventListener' : 'removeEventListener';
    window[method]('keydown', onKey);
    window[method]('pointerdown', onPress, true);
    window[method]('blur', onBlur);
  };
  watch(open, listen);
  onBeforeUnmount(() => listen(false));
}
