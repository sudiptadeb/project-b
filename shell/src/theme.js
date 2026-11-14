// Light or dark for one of our pages: Go's theme pref says light, dark, or as the system
// is, and the system's comes from the page's own prefers-color-scheme. Go makes the tones,
// both palettes of the frame colour (core/tones), and sends them with it; the chosen one
// goes on the root as CSS variables, and color-scheme makes the browser's own controls and
// scrollbars match. The shell's SDK paints them as Go's state changes; the settings page
// and the picker with a watchEffect on theirs.

const media = window.matchMedia('(prefers-color-scheme: dark)');
// What was last painted, so the page follows the system when it changes.
let painted = { tones: null, theme: 'system' };

// paintTheme puts tones' light or dark palette on the root, as theme says: 'light',
// 'dark', or 'system', as the system is. Nothing is painted before Go sends the tones.
export function paintTheme(tones, theme) {
  painted = { tones, theme };
  if (!tones) return;
  const dark = theme === 'dark' || (theme !== 'light' && media.matches);
  const root = document.documentElement;
  for (const [name, value] of Object.entries(dark ? tones.dark : tones.light)) {
    root.style.setProperty(name, value);
  }
  root.style.colorScheme = dark ? 'dark' : 'light';
}

media.addEventListener('change', () => paintTheme(painted.tones, painted.theme));
