import type { Cell, PlayerState } from "@/types/GameTypes";
import { getCellIndex } from "./fogOfWar";
import { normalizeAvatarPath } from "./normalizeAvatarPath";

// GameService writes rune values, not terrain names. Keep these codes aligned
// with game/mapgen.go and the existing Map background/interaction rules.
export const TILE = { obstacle: 32, walkable: 48, border: 49, monster: 77,
  start: 80, resource: 82, barrel: 66, portal: 112 } as const;

export type MapPlayer = Pick<PlayerState, "user_id" | "position"> &
  Partial<Pick<PlayerState, "image" | "name" | "group_id">>;
export type RealMapSource = {
  map_width: number;
  map_height: number;
  map: Cell[];
  start_positions?: readonly (readonly [number, number])[];
  players?: readonly MapPlayer[];
};
export type RenderObject = {
  kind: "portal" | "base" | "barrel" | "resource" | "monster" | "structure" | "player";
  x: number;
  y: number;
  image?: string;
  userId?: number;
};
export type RealMapView = {
  width: number;
  height: number;
  grid: Cell[];
  objects: RenderObject[];
  walkable: ReadonlySet<number>;
};

export function isWalkableTile(code: number): boolean {
  return code !== TILE.obstacle && code !== TILE.border;
}

/** Build a view of the server snapshot using the same visibility sets as Map. */
export function adaptRealMap(source: RealMapSource,
  visible: ReadonlySet<number>, explored: ReadonlySet<number>): RealMapView {
  const width = source.map_width, height = source.map_height;
  if (!Number.isInteger(width) || !Number.isInteger(height) || width <= 0 || height <= 0 ||
      width * height > 10000 || source.map.length !== width * height) {
    throw new Error("Invalid GameService map dimensions");
  }
  const grid: Cell[] = new Array(width * height);
  const walkable = new Set<number>();
  const objects: RenderObject[] = [];
  const add = (kind: RenderObject["kind"], cell: Cell, image?: string) =>
    objects.push({ kind, x: cell.x, y: cell.y, image });

  for (const cell of source.map) {
    if (!Number.isInteger(cell.x) || !Number.isInteger(cell.y) ||
        cell.x < 0 || cell.y < 0 || cell.x >= width || cell.y >= height ||
        !Number.isInteger(cell.tileCode)) throw new Error("Invalid GameService cell");
    const index = getCellIndex(cell.x, cell.y, width);
    if (grid[index]) throw new Error("Duplicate GameService cell");
    const known = visible.has(index) || explored.has(index);
    // Unknown terrain is opaque; explored cells retain only their static shape.
    const safe: Cell = known ? { ...cell,
      tileCode: !visible.has(index) &&
        (cell.tileCode === TILE.monster || cell.tileCode === TILE.resource || cell.tileCode === TILE.barrel)
        ? TILE.walkable : cell.tileCode,
      resource: visible.has(index) ? cell.resource : null,
      barbel: visible.has(index) ? cell.barbel : null,
      monster: visible.has(index) ? cell.monster : null,
      structure_type: visible.has(index) ? cell.structure_type : undefined,
      structure_image: visible.has(index) ? cell.structure_image : undefined,
      isPortal: visible.has(index) && cell.isPortal,
      isPlayer: visible.has(index) && cell.isPlayer,
    } : { cell_id: cell.cell_id, x: cell.x, y: cell.y, tileCode: TILE.obstacle,
      resource: null, barbel: null, monster: null, isPortal: false, isPlayer: false };
    grid[index] = safe;
    if (known && isWalkableTile(cell.tileCode)) walkable.add(index);
    if (!visible.has(index)) continue;
    if (cell.isPortal || cell.tileCode === TILE.portal) add("portal", cell, "/portal.png");
    if (cell.barbel) add("barrel", cell, cell.barbel.image);
    if (cell.resource) add("resource", cell, cell.resource.image);
    if (cell.monster) add("monster", cell, cell.monster.image);
    if (cell.structure_type) add(cell.structure_type === "base" ? "base" : "structure", cell,
      cell.structure_image || (cell.structure_type === "base" ? "/ui-icons/base.png" : undefined));
    else if (cell.tileCode === TILE.start) add("base", cell, "/ui-icons/base.png");
  }
  for (let index = 0; index < grid.length; index++) {
    if (!grid[index]) throw new Error("Incomplete GameService map");
  }
  for (const player of source.players ?? []) {
    const { x, y } = player.position;
    if (x < 0 || y < 0 || x >= width || y >= height ||
        !visible.has(getCellIndex(x, y, width))) continue;
    objects.push({ kind: "player", x, y, image: normalizeAvatarPath(player.image), userId: player.user_id });
  }
  return { width, height, grid, objects, walkable };
}
