import type { WindField } from "./beaufort.ts";

/**
 * Barbules de vent (convention OMM) : la hampe part du point et pointe d'où
 * vient le vent ; au bout, un fanion vaut 50 kn, un trait 10 kn, un demi-trait
 * 5 kn. Sous 2,5 kn, un cercle marque le calme. Dans l'hémisphère sud, les
 * traits sont de l'autre côté de la hampe.
 */

/** Force arrondie aux 5 kn, décomposée en fanions, traits et demi-trait. */
export type BarbFeathers = { knots: number; pennants: number; full: number; half: number };

export function barbFeathers(speedKnots: number): BarbFeathers {
  const knots = Math.max(0, Math.round(speedKnots / 5) * 5);
  const pennants = Math.floor(knots / 50);
  const full = Math.floor((knots - pennants * 50) / 10);
  const half = knots - pennants * 50 - full * 10 >= 5 ? 1 : 0;
  return { knots, pennants, full, half };
}

/** Dessin d'une barbule dont la hampe pointe vers le haut (vent du nord). */
export type BarbShape = {
  calm: boolean;
  /** Segments [x1, y1, x2, y2] : la hampe, puis les traits. */
  lines: [number, number, number, number][];
  /** Fanions [x1, y1, x2, y2, x3, y3]. */
  pennants: [number, number, number, number, number, number][];
};

/**
 * Barbule tracée depuis le point (cx, cy), hampe de `length` px vers le haut.
 * On la tourne ensuite de la direction d'où vient le vent.
 */
export function barbShape(speedKnots: number, cx: number, cy: number, length: number, south: boolean): BarbShape {
  const feathers = barbFeathers(speedKnots);
  if (feathers.knots === 0) return { calm: true, lines: [], pennants: [] };
  const tipY = cy - length;
  const side = south ? -1 : 1;
  const feather = length * 0.42;
  const step = length * 0.15;
  // Les traits penchent vers le bout de la hampe.
  const lean = feather * 0.35;
  const lines: BarbShape["lines"] = [[cx, cy, cx, tipY]];
  const pennants: BarbShape["pennants"] = [];
  let pos = 0;
  for (let index = 0; index < feathers.pennants; index += 1) {
    pennants.push([cx, tipY + pos, cx + side * feather, tipY + pos, cx, tipY + pos + step * 1.4]);
    pos += step * 1.9;
  }
  for (let index = 0; index < feathers.full; index += 1) {
    lines.push([cx, tipY + pos, cx + side * feather, tipY + pos - lean]);
    pos += step;
  }
  if (feathers.half) {
    // Un demi-trait seul ne se pose pas au bout de la hampe.
    if (pos === 0) pos = step;
    lines.push([cx, tipY + pos, cx + side * feather * 0.5, tipY + pos - lean * 0.5]);
  }
  return { calm: false, lines, pennants };
}

/** Une barbule à placer sur la carte. */
export type BarbPoint = { longitude: number; latitude: number; knots: number; direction: number };

function latitudeFromMercatorY(y: number): number {
  return ((2 * Math.atan(Math.exp(y)) - Math.PI / 2) * 180) / Math.PI;
}

function mercatorY(latitude: number): number {
  return Math.log(Math.tan(Math.PI / 4 + (latitude * Math.PI) / 360));
}

/**
 * Barbules en quinconce sur la grille du champ de vent : une ligne sur
 * `rowStep`, une colonne sur 2 × `rowStep`, décalée d'une ligne à l'autre.
 * Les points sans vent ou sans direction sont ignorés.
 */
export function barbPoints(field: WindField, rowStep: number): BarbPoint[] {
  const directions = field.directions;
  if (!directions || directions.length !== field.speeds.length) return [];
  const step = Math.max(1, Math.round(rowStep));
  const top = mercatorY(field.north);
  const bottom = mercatorY(field.south);
  const points: BarbPoint[] = [];
  let line = 0;
  for (let row = Math.floor(step / 2); row < field.rows; row += step, line += 1) {
    const latitude = latitudeFromMercatorY(top + ((bottom - top) * row) / (field.rows - 1));
    for (let col = Math.floor(step / 2) + (line % 2) * step; col < field.cols; col += 2 * step) {
      const index = row * field.cols + col;
      const knots = field.speeds[index];
      const direction = directions[index];
      if (!(knots >= 0) || !(direction >= 0)) continue;
      const longitude = field.west + ((field.east - field.west) * col) / (field.cols - 1);
      points.push({ longitude, latitude, knots, direction });
    }
  }
  return points;
}
