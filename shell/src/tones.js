// Every colour of our pages, from the one frame colour Go owns, light or dark, so any
// frame colour looks coherent in both. Light: the bar and surfaces are the frame mixed far
// toward white, icons a little toward black. Dark: the bar and surfaces are a near-black
// with a trace of the frame, icons and the accent the frame lifted toward white. The
// frame itself stays the colour the user picked, as the window paints it.
export function tones(frame, dark) {
  const rgb = [1, 3, 5].map((i) => parseInt(frame.slice(i, i + 2), 16));
  const hex = (c) => '#' + c.map((v) => Math.round(v).toString(16).padStart(2, '0')).join('');
  const toward = (target, t) => hex(rgb.map((v, i) => v + ((Array.isArray(target) ? target[i] : target) - v) * t));
  const faint = (a) => `rgba(${rgb.join(', ')}, ${a})`;
  if (dark) {
    return {
      '--frame': frame,
      '--accent': toward(255, 0.2),
      '--bar': toward([27, 28, 30], 0.88),
      '--surface': toward([40, 42, 45], 0.9),
      '--icon': toward(255, 0.55),
      '--accent-soft': faint(0.34),
      '--accent-tint': faint(0.2),
      '--text': '#e8eaed',
      '--muted': '#9aa0a6',
      '--line': 'rgba(255, 255, 255, 0.12)',
      '--hover': 'rgba(255, 255, 255, 0.08)',
      '--press': 'rgba(255, 255, 255, 0.14)',
      '--danger': '#f28b82',
    };
  }
  return {
    '--frame': frame,
    '--accent': frame,
    '--bar': toward(255, 0.87),
    '--surface': '#ffffff',
    '--icon': toward(0, 0.16),
    '--accent-soft': faint(0.18),
    '--accent-tint': faint(0.1),
    '--text': '#1f1f1f',
    '--muted': '#5c6270',
    '--line': 'rgba(15, 20, 25, 0.1)',
    '--hover': 'rgba(255, 255, 255, 0.6)',
    '--press': 'rgba(0, 0, 0, 0.08)',
    '--danger': '#ea3323',
  };
}
