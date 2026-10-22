import { axialToWorld, hexDistance, hexNeighbor, hexKey, type HexCoord } from "@/utils/hexGrid";
import type { HexSnapshot } from "@/types/HexGameTypes";

type Point = { x: number; y: number };
const point = (hex: HexCoord): Point => { const world = axialToWorld(hex, 1); return { x: world.x, y: world.z }; };
const cross = (a: Point, b: Point, c: Point) => (b.x - a.x) * (c.y - a.y) - (b.y - a.y) * (c.x - a.x);
const onSegment = (a: Point, b: Point, p: Point) => Math.abs(cross(a, b, p)) <= 1e-8 && p.x >= Math.min(a.x, b.x) - 1e-8 && p.x <= Math.max(a.x, b.x) + 1e-8 && p.y >= Math.min(a.y, b.y) - 1e-8 && p.y <= Math.max(a.y, b.y) + 1e-8;
function segmentsTouch(a: Point, b: Point, c: Point, d: Point) {
  const abc = cross(a, b, c), abd = cross(a, b, d), cda = cross(c, d, a), cdb = cross(c, d, b);
  if (Math.abs(abc) <= 1e-8 && onSegment(a, b, c)) return true;
  if (Math.abs(abd) <= 1e-8 && onSegment(a, b, d)) return true;
  if (Math.abs(cda) <= 1e-8 && onSegment(c, d, a)) return true;
  if (Math.abs(cdb) <= 1e-8 && onSegment(c, d, b)) return true;
  return (abc > 1e-8 && abd < -1e-8 || abc < -1e-8 && abd > 1e-8) && (cda > 1e-8 && cdb < -1e-8 || cda < -1e-8 && cdb > 1e-8);
}

// Use only revealed geometry. The server checks the authoritative map again after moving.
function clearSight(snapshot: HexSnapshot, from: HexCoord, to: HexCoord) {
  const known = new Map(snapshot.cells.filter(cell => cell.visibility !== "unexplored").map(cell => [hexKey(cell.position), cell]));
  if (known.get(hexKey(from))?.terrain !== "floor" || known.get(hexKey(to))?.terrain !== "floor") return false;
  const a = point(from), b = point(to);
  for (const edge of snapshot.edges) {
    const p = point(edge.position), q = point(hexNeighbor(edge.position, edge.direction));
    const middle = { x: (p.x + q.x) / 2, y: (p.y + q.y) / 2 };
    const dx = q.x - p.x, dy = q.y - p.y, length = Math.hypot(dx, dy);
    if (length && segmentsTouch(a, b, { x: middle.x - dy / length / 2, y: middle.y + dx / length / 2 }, { x: middle.x + dy / length / 2, y: middle.y - dx / length / 2 })) return false;
  }
  for (const cell of snapshot.cells) {
    if (cell.terrain === "floor" && cell.visibility !== "unexplored" || hexKey(cell.position) === hexKey(from) || hexKey(cell.position) === hexKey(to)) continue;
    const center = point(cell.position);
    const corners = Array.from({ length: 6 }, (_, index) => {
      const angle = (-90 + index * 60) * Math.PI / 180;
      return { x: center.x + Math.cos(angle), y: center.y + Math.sin(angle) };
    });
    if (corners.some((corner, index) => segmentsTouch(a, b, corner, corners[(index + 1) % 6]))) return false;
  }
  return true;
}

export function attackApproachCells(snapshot: HexSnapshot, target: HexCoord): HexCoord[] {
  const hero = snapshot.player;
  if (!hero) return [];
  const range = hero.isRanged ? hero.attackRange : 1;
  const occupied = new Set(snapshot.actors.map(actor => hexKey(actor.position)));
  return snapshot.cells.filter(cell => cell.terrain === "floor" && cell.visibility !== "unexplored" && !occupied.has(hexKey(cell.position)) &&
    hexKey(cell.position) !== hexKey(hero.position) && hexDistance(cell.position, target) <= range && hexDistance(cell.position, target) > 0 && clearSight(snapshot, cell.position, target))
    .map(cell => cell.position).sort((a, b) => hexDistance(hero.position, a) - hexDistance(hero.position, b));
}
