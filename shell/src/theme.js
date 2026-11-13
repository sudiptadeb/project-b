// Light or dark for one of our pages: Go's theme pref says light, dark, or as the system
// is, and the system's comes from the page's own prefers-color-scheme. The tones go on the
// root as CSS variables, and color-scheme makes the browser's own controls and scrollbars
// match. The shell's SDK paints them as Go's state changes; the settings page and the
// picker with a watchEffect on theirs.
import { tones } from './tones.js';

const media = window.matchMedia('(prefers-color-scheme: dark)');
// What was last painted, so the page follows the system when it changes.
let painted = { windowColor: '', theme: 'system' };

// paintTheme puts the tones of windowColor on the root, light or dark as theme says:
// 'light', 'dark', or 'system', as the system is. Nothing is painted before Go sends a
// colour.
export function paintTheme(windowColor, theme) {
  painted = { windowColor, theme };
  if (!windowColor) return;
  const dark = theme === 'dark' || (theme !== 'light' && media.matches);
  const root = document.documentElement;
  for (const [name, value] of Object.entries(tones(windowColor, dark))) {
    root.style.setProperty(name, value);
  }
  root.style.colorScheme = dark ? 'dark' : 'light';
}

media.addEventListener('change', () => paintTheme(painted.windowColor, painted.theme));
