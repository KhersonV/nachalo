import { parseGeometry, parsePosition } from "./geometry";
import type { HexCoord } from "@/utils/hexGrid";

export type HexResource = { id: number; type: string; description: string; image: string; item_type?: "resource" | "artifact" };
export type HexCell = { cellId: number; position: HexCoord; terrain: "floor" | "rock"; visibility?: "unexplored" | "explored" | "visible"; room?: number; resource?: HexResource; barrel?: HexResource; isPortal?: boolean };
export type HexEdge = { position: HexCoord; direction: number; kind: "wall" };
export type HexActor = { id: string; kind: "player" | "monster"; userId?: number; teamId?: number; position: HexCoord; health?: number; maxHealth?: number; monsterRefId?: number; name?: string; type?: string; image?: string; attack?: number; defense?: number; speed?: number; maneuverability?: number; vision?: number };
export type HexStart = { userId: number; teamId: number; position: HexCoord };
export type HexQuestArtifact = { id: number; name: string; image?: string; description?: string; status: "not_found" | "found" | "carried" | "ground" | "extracted"; carrierUserId?: number; carrierName?: string; position?: HexCoord };
export type HexPlayer = {
  userId: number; teamId?: number; name: string; position: HexCoord; energy: number; maxEnergy: number;
  health: number; maxHealth: number; mobility: number; agility: number;
  characterType: string; isRanged: boolean; attackRange: number;
  inventory?: Record<string, { item_count?: number; item_name?: string }>;
  balance?: number;
};
export type HexResultItem = { name: string; image?: string; status?: string };
export type HexResultStats = { playerKills: number; monsterKills: number; damageTotal: number; damageToPlayers: number; damageToMonsters: number; damageTaken: number };
export type HexPlayerResult = { method: "artifact" | "paid" | "defeat" | "team_victory" | "opponent_extracted"; reason?: string; winnerUserId?: number; winnerTeamId?: number; goldEarned: number; goldSpent: number; xp: number; transferred: Record<string, { item_count?: number; item_name?: string; name?: string; image?: string }>; lost: Record<string, { item_count?: number; item_name?: string; name?: string; image?: string }>; equipment: string[]; lostEquipment: string[]; equipmentItems?: HexResultItem[]; lostEquipmentItems?: HexResultItem[]; wornEquipment?: HexResultItem[]; stats?: HexResultStats; activeUser?: number; turnNumber?: number };
export type HexRoute = { path: HexCoord[]; stepCost: number; totalCost: number; energy: number; affordable: boolean; position: HexCoord };
export type HexAttackPreview = { targetId: number; distance: number; cost: number; mode: "melee" | "ranged"; energy: number; targetHP: number };
export type HexAttackResult = HexAttackPreview & { damage: number; counterDamage: number; newTargetHP: number; newAttackerHP: number; monsterDied: boolean; playerDied?: boolean; effects: Array<{ kind: string; amount?: number }>; drops: Array<{ name: string }>; stateRevision: number; mapRevision: number };
export type HexSnapshot = {
  schemaVersion: 2;
  geometry: { kind: "hex"; coordinates: "axial"; orientation: "pointy"; version: 1 };
  instanceId: string; mode?: string; turnOrder?: number[]; mapRevision: number; stateRevision: number; seed: number; radius?: number;
  starts?: HexStart[]; portal?: HexCoord;
  cells: HexCell[]; edges: HexEdge[]; actors: HexActor[];
  player?: HexPlayer; activeUser?: number; activePlayerName?: string; turnNumber?: number; turnDeadline?: string; turnSeconds?: number; finished?: boolean; questArtifactId?: number; questArtifact?: HexQuestArtifact; result?: HexPlayerResult;
};

export function isHexSnapshot(value: unknown): value is HexSnapshot {
  if (!value || typeof value !== "object") return false;
  const data = value as Partial<HexSnapshot>;
  try {
    if (parseGeometry(data.geometry, data.schemaVersion).kind !== "hex") return false;
    if (typeof data.instanceId !== "string" || !Number.isSafeInteger(data.stateRevision) || !Number.isSafeInteger(data.mapRevision)) return false;
    if (!Array.isArray(data.cells) || !Array.isArray(data.edges) || !Array.isArray(data.actors)) return false;
    if (!data.cells.every(c => c && Number.isSafeInteger(c.cellId) && (c.terrain === "floor" || c.terrain === "rock") && !!parsePosition(c.position, data.geometry!))) return false;
    if (data.portal && !parsePosition(data.portal, data.geometry!)) return false;
    if (data.questArtifact && (!Number.isSafeInteger(data.questArtifact.id) || typeof data.questArtifact.name !== "string" || !["not_found", "found", "carried", "ground", "extracted"].includes(data.questArtifact.status) || (data.questArtifact.position && !parsePosition(data.questArtifact.position, data.geometry!)))) return false;
    if (data.starts && (!Array.isArray(data.starts) || !data.starts.every(s => Number.isSafeInteger(s.userId) && Number.isSafeInteger(s.teamId) && !!parsePosition(s.position, data.geometry!)))) return false;
    if (!data.edges.every(e => e && e.kind === "wall" && Number.isInteger(e.direction) && e.direction >= 0 && e.direction < 3 && !!parsePosition(e.position, data.geometry!))) return false;
    if (!data.actors.every(a => a && (a.kind === "player" || a.kind === "monster") && !!parsePosition(a.position, data.geometry!))) return false;
    if (data.player && (
      !Number.isSafeInteger(data.player.userId) || typeof data.player.name !== "string" ||
      ![data.player.energy, data.player.maxEnergy, data.player.health, data.player.maxHealth,
        data.player.mobility, data.player.agility, data.player.attackRange].every(Number.isSafeInteger) ||
      typeof data.player.characterType !== "string" || typeof data.player.isRanged !== "boolean" ||
      !parsePosition(data.player.position, data.geometry!)
    )) return false;
    return true;
  } catch { return false; }
}
