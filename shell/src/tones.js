// Every colour of our pages, from the one frame colour Go owns, light or dark, so any
// frame colour looks coherent in both. Light: the bar and surfaces are the frame mixed far
// toward white, icons a little toward black. Dark: the bar and surfaces are a near-black
// with a trace of the frame, icons and the accent the frame lifted toward white. The
// frame itself stays the colour the user picked, as the window paints it.
export function tones(frame, dark) {
  const rgb = [1, 3, 5].map((i) => parseInt(frame.slice(i, i + 2), 16));
  const hex = (c) => '#' + c.map((v) => v.toString(16).padStart(2, '0')).join('');
  const toward = (target, t) => hex(mix(rgb, target, t));
  const faint = (a) => `rgba(${rgb.join(', ')}, ${a})`;
  const { accent, onAccent } = labelled(dark ? mix(rgb, 255, 0.2) : rgb);
  if (dark) {
    return {
      '--frame': frame,
      '--accent': hex(accent),
      '--on-accent': hex(onAccent),
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
    '--accent': hex(accent),
    '--on-accent': hex(onAccent),
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

// A colour moved a fraction t of the way to target, a colour or one channel value for all.
function mix(from, target, t) {
  return from.map((v, i) => Math.round(v + ((Array.isArray(target) ? target[i] : target) - v) * t));
}

const white = [255, 255, 255];
const ink = [31, 31, 31];

// What goes on the accent, a primary button's words or a badge's count, is white or the
// ink, whichever reads better on it: the accent's lightness follows the frame, so no one
// label colour reads on all of them. It must reach WCAG's 4.5 for text. A few colours sit
// where neither does, a mid blue or a red lifted for dark: those move away from the
// better label, in small steps, until it reads. The rest keep the accent they had.
function labelled(base) {
  const onAccent = contrast(base, white) >= contrast(base, ink) ? white : ink;
  let accent = base;
  for (let t = 0.02; contrast(accent, onAccent) < 4.5; t += 0.02) accent = mix(base, onAccent === white ? 0 : 255, t);
  return { accent, onAccent };
}

// WCAG 2's contrast ratio of two colours, from their relative luminance.
function contrast(a, b) {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

function luminance(c) {
  const [r, g, b] = c.map((v) => {
    const s = v / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}
