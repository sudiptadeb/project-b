// The shell's tones, all from the one frame colour Go owns, so any frame colour looks
// coherent: the bar and surfaces are the frame mixed far toward white, icons a little
// toward black, and the accent's soft fills the frame itself, faint.
export function tones(frame) {
  const rgb = [1, 3, 5].map((i) => parseInt(frame.slice(i, i + 2), 16));
  const hex = (c) => '#' + c.map((v) => Math.round(v).toString(16).padStart(2, '0')).join('');
  const toward = (target, t) => hex(rgb.map((v) => v + (target - v) * t));
  const faint = (a) => `rgba(${rgb.join(', ')}, ${a})`;
  return {
    '--frame': frame,
    '--accent': frame,
    '--bar': toward(255, 0.87),
    '--icon': toward(0, 0.16),
    '--accent-soft': faint(0.18),
    '--accent-tint': faint(0.1),
  };
}
