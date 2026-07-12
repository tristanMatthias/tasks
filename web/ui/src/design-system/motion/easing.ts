/**
 * A CSS `cubic-bezier(x1, y1, x2, y2)` timing function as a Svelte easing
 * function (maps linear time in [0,1] to eased progress). Lets us reuse the same
 * curves the platform uses — e.g. the iOS sheet curve cubic-bezier(0.32,0.72,0,1).
 */
export function cubicBezier(x1: number, y1: number, x2: number, y2: number): (t: number) => number {
  // Cubic Bézier component with P0=0, P3=1 and the given control point.
  const curve = (a: number, b: number, u: number) => {
    const v = 1 - u;
    return 3 * v * v * u * a + 3 * v * u * u * b + u * u * u;
  };
  const slope = (a: number, b: number, u: number) => {
    const v = 1 - u;
    return 3 * v * v * a + 6 * v * u * (b - a) + 3 * u * u * (1 - b);
  };

  return (t: number): number => {
    if (t <= 0) return 0;
    if (t >= 1) return 1;
    // Solve curveX(u) = t for the Bézier parameter u, then read curveY(u).
    let u = t;
    for (let i = 0; i < 8; i++) {
      const x = curve(x1, x2, u) - t;
      const dx = slope(x1, x2, u);
      if (Math.abs(x) < 1e-5 || dx === 0) break;
      u -= x / dx;
    }
    return curve(y1, y2, Math.min(1, Math.max(0, u)));
  };
}
