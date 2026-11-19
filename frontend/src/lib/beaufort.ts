/**
 * Échelle de Beaufort : borne haute (nœuds, exclue) de chaque force et couleur
 * de fond sur la carte. Couleurs usuelles de l'échelle, du calme (blanc) à
 * l'ouragan (rouge).
 */
export const BEAUFORT = [
  { force: 0, below: 1, label: "Calme", color: "#ffffff" },
  { force: 1, below: 4, label: "Très légère brise", color: "#aef1f9" },
  { force: 2, below: 7, label: "Légère brise", color: "#96f7dc" },
  { force: 3, below: 11, label: "Petite brise", color: "#96f7b4" },
  { force: 4, below: 17, label: "Jolie brise", color: "#6ff46f" },
  { force: 5, below: 22, label: "Bonne brise", color: "#73ed12" },
  { force: 6, below: 28, label: "Vent frais", color: "#a4ed12" },
  { force: 7, below: 34, label: "Grand frais", color: "#daed12" },
  { force: 8, below: 41, label: "Coup de vent", color: "#edc212" },
  { force: 9, below: 48, label: "Fort coup de vent", color: "#ed8f12" },
  { force: 10, below: 56, label: "Tempête", color: "#ed6312" },
  { force: 11, below: 64, label: "Violente tempête", color: "#ed2912" },
  { force: 12, below: Number.POSITIVE_INFINITY, label: "Ouragan", color: "#d5102d" },
] as const;

/** Force Beaufort d'une vitesse en nœuds. */
export function beaufortForce(knots: number): number {
  return BEAUFORT.findIndex((level) => knots < level.below);
}

/** Champ de vent renvoyé par GET /api/weather/wind. */
export type WindField = {
  time: string;
  south: number;
  north: number;
  west: number;
  east: number;
  cols: number;
  rows: number;
  /** Nœuds, ligne par ligne du nord au sud ; négatif = inconnu. */
  speeds: number[];
};

const RGB = BEAUFORT.map((level) => [1, 3, 5].map((index) => parseInt(level.color.slice(index, index + 2), 16)));

/**
 * Pixels RGBA d'une image width × height couvrant le champ : la vitesse est
 * interpolée entre les nœuds de la grille, puis colorée selon sa force, pour
 * des frontières nettes entre deux forces. Les points inconnus sont ignorés ;
 * un pixel qui ne dépend que d'eux reste transparent.
 */
export function windFieldPixels(field: WindField, width: number, height: number, alpha = 255): Uint8ClampedArray {
  const pixels = new Uint8ClampedArray(width * height * 4);
  const at = (col: number, row: number) => field.speeds[row * field.cols + col];
  for (let y = 0; y < height; y += 1) {
    const gy = height > 1 ? (y / (height - 1)) * (field.rows - 1) : 0;
    const row = Math.min(field.rows - 2, Math.floor(gy));
    const fy = gy - row;
    for (let x = 0; x < width; x += 1) {
      const gx = width > 1 ? (x / (width - 1)) * (field.cols - 1) : 0;
      const col = Math.min(field.cols - 2, Math.floor(gx));
      const fx = gx - col;
      const corners = [at(col, row), at(col + 1, row), at(col, row + 1), at(col + 1, row + 1)];
      const weights = [(1 - fx) * (1 - fy), fx * (1 - fy), (1 - fx) * fy, fx * fy];
      // Les points inconnus sont écartés et les autres repondérés.
      let total = 0;
      let sum = 0;
      corners.forEach((value, index) => {
        if (value >= 0 && weights[index] > 0) {
          total += weights[index];
          sum += value * weights[index];
        }
      });
      if (total < 1e-9) continue;
      const [r, g, b] = RGB[beaufortForce(sum / total)];
      const offset = (y * width + x) * 4;
      pixels[offset] = r;
      pixels[offset + 1] = g;
      pixels[offset + 2] = b;
      pixels[offset + 3] = alpha;
    }
  }
  return pixels;
}
