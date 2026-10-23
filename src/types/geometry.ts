export type HexPosition = { q: number; r: number };
export type HexGeometry = { kind: "hex"; coordinates: "axial"; orientation: "pointy"; version: 1 };

function record(value: unknown): Record<string, unknown> | null {
  return value !== null && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown> : null;
}

export function parseGeometry(value: unknown, schemaVersion: unknown): HexGeometry {
  const geometry = record(value);
  if (schemaVersion === 2 && geometry?.kind === "hex" && geometry.coordinates === "axial" && geometry.orientation === "pointy" && geometry.version === 1) {
    return geometry as HexGeometry;
  }
  throw new Error(`Unsupported match geometry: schemaVersion=${String(schemaVersion)}, geometry=${JSON.stringify(value)}`);
}

export function parsePosition(value: unknown, geometry: HexGeometry): HexPosition {
  const position = record(value);
  if (geometry.kind !== "hex" || !position || Object.keys(position).length !== 2 ||
      !Number.isSafeInteger(position.q) || !Number.isSafeInteger(position.r)) {
    throw new Error("Invalid HEX position");
  }
  return position as HexPosition;
}
