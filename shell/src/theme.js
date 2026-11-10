// Light or dark for one of our pages: Go's theme pref says light, dark, or as the system
// is, and the system's comes from the page's own prefers-color-scheme. The tones go on the
// root as CSS variables, and color-scheme makes the browser's own controls and scrollbars
// match.
import { ref, watchEffect } from 'vue';
import { tones } from './tones.js';

const media = window.matchMedia('(prefers-color-scheme: dark)');
const systemDark = ref(media.matches);
media.addEventListener('change', () => (systemDark.value = media.matches));

// useTheme keeps the root's colours following state.windowColor and state.theme.
export function useTheme(state) {
  watchEffect(() => {
    if (!state.windowColor) return;
    const dark = state.theme === 'dark' || (state.theme !== 'light' && systemDark.value);
    const root = document.documentElement;
    for (const [name, value] of Object.entries(tones(state.windowColor, dark))) {
      root.style.setProperty(name, value);
    }
    root.style.colorScheme = dark ? 'dark' : 'light';
  });
}
