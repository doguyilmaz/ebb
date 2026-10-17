/**
 * Keyed transitions between two layouts. Shapes in both interpolate; shapes
 * only in the new layout come in from ghostIn(shape) while fading in, shapes
 * only in the old one leave toward ghostOut(shape) while fading out. Zoom,
 * metric switches and refreshes are all this one mechanism with different ghosts.
 */
export interface Keyed {
  key: string;
}

export interface Transition<S> {
  /** [from, to, alpha from, alpha to] */
  moves: [S, S, number, number][];
}

export interface Frame<S> {
  shape: S;
  alpha: number;
}

export function plan<S extends Keyed>(prev: S[], next: S[], ghostIn: (s: S) => S, ghostOut: (s: S) => S): Transition<S> {
  const old = new Map(prev.map((s) => [s.key, s]));
  const moves: [S, S, number, number][] = [];
  for (const s of next) {
    const p = old.get(s.key);
    if (p) {
      moves.push([p, s, 1, 1]);
      old.delete(s.key);
    } else moves.push([ghostIn(s), s, 0, 1]);
  }
  for (const p of old.values()) moves.push([p, ghostOut(p), 1, 0]);
  return { moves };
}

export function frame<S>(tr: Transition<S>, t: number, lerp: (a: S, b: S, t: number) => S): Frame<S>[] {
  const out: Frame<S>[] = [];
  for (const [a, b, fa, fb] of tr.moves) {
    const alpha = fa + (fb - fa) * t;
    if (t >= 1 && fb === 0) continue;
    out.push({ shape: t >= 1 ? b : lerp(a, b, t), alpha });
  }
  return out;
}
