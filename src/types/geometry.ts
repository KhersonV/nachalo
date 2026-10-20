export type SquarePosition = { x: number; y: number; q?: never; r?: never };
export type HexPosition = { q: number; r: number; x?: never; y?: never };
export type WorldPosition = { x: number; y: number; z: number };

export type SquareGeometry = { kind: "square"; coordinates: "cartesian"; version: 1 };
export type HexGeometry = { kind: "hex"; coordinates: "axial"; orientation: "pointy"; version: 1 };
export type MatchGeometry = SquareGeometry | HexGeometry;
export const SQUARE_V1: SquareGeometry = { kind: "square", coordinates: "cartesian", version: 1 };

export type SquareSnapshot = { schemaVersion: 1; geometry: SquareGeometry; map: Array<SquarePosition>; players: Array<{ position: SquarePosition }> };
export type HexSnapshot = import("./HexGameTypes").HexSnapshot;
export type VersionedSnapshot = SquareSnapshot | HexSnapshot;

function record(value: unknown): Record<string, unknown> | null {
  return value !== null && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown> : null;
}

export function parseGeometry(value: unknown, schemaVersion: unknown): MatchGeometry {
  if (value == null && (schemaVersion == null || schemaVersion === 1)) return SQUARE_V1;
  const g = record(value);
  if (schemaVersion === 1 && g?.kind === "square" && g.coordinates === "cartesian" && g.version === 1 && g.orientation === undefined) return SQUARE_V1;
  if (schemaVersion === 2 && g?.kind === "hex" && g.coordinates === "axial" && g.orientation === "pointy" && g.version === 1) return g as HexGeometry;
  throw new Error(`Unsupported match geometry: schemaVersion=${String(schemaVersion)}, geometry=${JSON.stringify(value)}`);
}

export function parsePosition(value: unknown, geometry: MatchGeometry, allowExtra = false): SquarePosition | HexPosition {
  const p = record(value);
  const keys = geometry.kind === "square" ? ["x", "y"] : ["q", "r"];
  const forbidden = geometry.kind === "square" ? ["q", "r"] : ["x", "y"];
  if (!p || (!allowExtra && Object.keys(p).length !== 2) || forbidden.some(k => k in p) || !keys.every(k => Number.isSafeInteger(p[k]))) {
    throw new Error(`Incomplete or mixed ${geometry.kind} position`);
  }
  return p as SquarePosition | HexPosition;
}

export function assertSquareSnapshot(value: unknown): void {
  const snapshot = record(value);
  if (!snapshot) throw new Error("Invalid match snapshot");
  const geometry = parseGeometry(snapshot.geometry, snapshot.schemaVersion);
  if (geometry.kind !== "square") throw new Error("HEX snapshot is not supported by the square scene");
  if (!Array.isArray(snapshot.map) || !Array.isArray(snapshot.players)) throw new Error("Incomplete match snapshot");
  snapshot.map.forEach(cell => parsePosition(cell, geometry, true));
  snapshot.players.forEach(player => parsePosition(record(player)?.position, geometry));
  const starts = snapshot.start_positions;
  if (starts !== undefined && (!Array.isArray(starts) || starts.some(p => !Array.isArray(p) || p.length !== 2 || !p.every(Number.isSafeInteger)))) throw new Error("Invalid square start positions");
  const portal = snapshot.portal_position;
  if (portal !== undefined && (!Array.isArray(portal) || portal.length !== 2 || !portal.every(Number.isSafeInteger))) throw new Error("Invalid square portal position");
}

export function assertSquarePatch(payload: unknown): void {
  const p = record(payload);
  if (!p) throw new Error("Invalid match patch");
  if (p.geometry !== undefined || p.schemaVersion !== undefined) {
    if (parseGeometry(p.geometry, p.schemaVersion).kind !== "square") throw new Error("HEX patch is not supported by the square scene");
  }
  for (const key of ["newPosition", "position"]) if (p[key] !== undefined) parsePosition(p[key], SQUARE_V1);
  if (p.updatedCell !== undefined) parsePosition(p.updatedCell, SQUARE_V1, true);
  for (const key of ["attackerSnapshot", "targetSnapshot"]) {
    const actor = record(p[key]);
    if (actor?.position !== undefined) parsePosition(actor.position, SQUARE_V1);
  }
  if (Array.isArray(p.effects)) for (const effect of p.effects) {
    const position = record(effect)?.positionAfter;
    if (position !== undefined) parsePosition(position, SQUARE_V1);
  }
}
