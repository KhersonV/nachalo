import { parseGeometry, parsePosition } from "./geometry";
import type { HexCoord } from "@/utils/hexGrid";
import type { EquipmentSlot } from "./equipment";

export type ProfessionNode = { category: "ore" | "forest" | "remains" | "herbs"; class: string; profession: string; professionName: string };
export type HexResource = { professionNode?: ProfessionNode; id: number; type: string; description: string; image: string; item_type?: "resource" | "artifact" };
export type HexCell = { cellId: number; position: HexCoord; terrain: "floor" | "rock"; visibility?: "unexplored" | "explored" | "visible"; room?: number; environmentType?: "brazier"; resource?: HexResource; barrel?: HexResource; isPortal?: boolean; structureType?: string; structureOwnerUserId?: number; structureHealth?: number; structureDefense?: number; structureAttack?: number; isUnderConstruction?: boolean; constructionTurnsLeft?: number };
export type HexEdge = { position: HexCoord; direction: number; kind: "wall" };
export type HexHeroAppearance = {
  archetype: "guardian" | "berserker" | "ranger" | "mystic";
  equipment: Partial<Record<EquipmentSlot, { code: string }>>;
};
export type HexActor = { id: string; kind: "player" | "monster"; userId?: number; teamId?: number; position: HexCoord; health?: number; maxHealth?: number; monsterRefId?: number; name?: string; type?: string; image?: string; attack?: number; defense?: number; speed?: number; maneuverability?: number; vision?: number; armorBreakOwner?: number; armorBreakStacks?: number; mpDrain?: boolean; appearance?: HexHeroAppearance };
export type HexStart = { userId: number; teamId: number; position: HexCoord };
export type HexQuestArtifact = { id: number; name: string; image?: string; description?: string; status: "not_found" | "found" | "carried" | "ground" | "extracted"; carrierUserId?: number; carrierName?: string; position?: HexCoord };
// All AP/MP values in API are integer half points; divide by 2 for display.
export type HexBudgets = { ap: number; maxAP: number; mp: number; maxMP: number; reactions: number; maxReactions: number; personalTurn: number; waterAvailableTurn: number; mpDrain: boolean; overloadCount: number; overloadTarget: string; mysticReflexUsed: boolean; inGuardianAura: boolean; auraSlowUsed: boolean };
export type CraftConsumableStatus = { cooldowns?: Record<string, number>; critical?: { charges?: number; until?: number }; vampirism?: { until?: number }; speed?: { until?: number }; immuneUntil?: number };
export type HexPlayer = HexBudgets & {
  pocketCapacity?: number; pocketsUsed?: number;
  consumableStatus?: CraftConsumableStatus;
  professionUnlocked?: boolean; professionGatherCount?: number; professionGatherLimit?: number; professionLevel?: number;
  userId: number; teamId?: number; name: string; position: HexCoord; sightRange: number;
  health: number; maxHealth: number; mobility: number; agility: number;
  characterType: string; isRanged: boolean; attackRange: number;
  inventory?: Record<string, { item_count?: number; item_name?: string; name?: string }>;
  equipmentLoot?: Array<{ itemInstanceId: string; name: string; rarity: string; image: string; pocketBonus?: number }>;
  balance?: number;
};
export type HexResultItem = { name: string; image?: string; status?: string };
export type HexResultStats = { playerKills: number; monsterKills: number; damageTotal: number; damageToPlayers: number; damageToMonsters: number; damageTaken: number };
export type HexPlayerResult = { method: "artifact" | "paid" | "defeat" | "victory" | "team_victory" | "opponent_extracted"; isWinner?: boolean; reason?: string; winnerUserId?: number; winnerTeamId?: number; goldEarned: number; goldSpent: number; xp: number; transferred: Record<string, { item_count?: number; item_name?: string; name?: string; image?: string }>; lost: Record<string, { item_count?: number; item_name?: string; name?: string; image?: string }>; equipment: string[]; lostEquipment: string[]; equipmentItems?: HexResultItem[]; lostEquipmentItems?: HexResultItem[]; wornEquipment?: HexResultItem[]; stats?: HexResultStats; activeUser?: number; turnNumber?: number };
export type HexRoute = { destination?: HexCoord; reachablePosition?: HexCoord; affordableSteps?: number; reachableCost?: number; stepCosts?: number[]; previewToken?: string; partial?: boolean; mapRevision?: number; path: HexCoord[]; stepCost: number; totalCost: number; mp: number; affordable: boolean; position: HexCoord; auraMPCost?: number; auraHits?: Array<{ position: HexCoord; damage: number; sourceId: number }>; auraEnded?: boolean; stateRevision?: number };
export type HexVisualMove = { commandId: string; instanceId: string; from: HexCoord; position: HexCoord; path?: HexCoord[]; stateRevision?: number };
export type HexAttackPreview = { route?: HexRoute; attackPosition?: HexCoord; plannedTargetPosition?: HexCoord; canAttack?: boolean; reason?: string; previewToken?: string; previewRevision?: number; previewMapRevision?: number; targetId: number; distance: number; cost: number; mode: "melee" | "ranged"; ap: number; targetHP: number };
export type HexCombatRef = { id: number; type: "player" | "monster" };
export type HexCombatStep = { kind: string; source?: HexCombatRef; target: HexCombatRef; damage?: number; targetHpAfter?: number };
export type HexCombatEffect = { kind: string; source?: HexCombatRef; target?: HexCombatRef; succeeded?: boolean; amount?: number; bonusDamage?: number; mpGranted?: number; stacks?: number; durationTurns?: number; positionAfter?: { x: number; y: number } };
export type HexAttackResult = HexAttackPreview & { budget?: HexBudgets; damage: number; counterDamage: number; newTargetHP: number; newAttackerHP: number; monsterDied: boolean; playerDied?: boolean; effects: HexCombatEffect[]; steps: HexCombatStep[]; drops: Array<{ name: string; ownerUserId: number }>; stateRevision: number; mapRevision: number; attackerPosition?: HexCoord; targetPosition?: HexCoord; attackerHPBefore?: number };
export type HexCombatIndicator = { id: string; position: HexCoord; text: string; tone: "damage" | "critical" | "heal" | "energyLoss" | "energyGain" | "aura" | "status" | "push"; lane: number };
export type HexVisualNotice = { instanceId: string; schemaVersion: 2; commandId: string; stateRevision: number; kind: "attack" | "move"; attackerPosition?: HexCoord; targetPosition?: HexCoord; attackerHPBefore?: number; targetHP?: number; steps?: HexCombatStep[]; effects?: HexCombatEffect[]; position?: HexCoord; auraMPCost?: number; auraHits?: Array<{ position: HexCoord; damage: number; sourceId: number }>; auraEnded?: boolean };
export type HexSnapshot = {
  schemaVersion: 2;
  geometry: { kind: "hex"; coordinates: "axial"; orientation: "pointy"; version: 1 };
  instanceId: string; mode?: string; turnOrder?: number[]; mapRevision: number; stateRevision: number; seed: number; radius?: number;
  starts?: HexStart[]; portal?: HexCoord;
  cells: HexCell[]; edges: HexEdge[]; actors: HexActor[];
  player?: HexPlayer; activeUser?: number; activePlayerName?: string; turnNumber?: number; turnDeadline?: string; turnSeconds?: number; disconnectedPlayers?: { userId: number; deadline: string }[]; finished?: boolean; questArtifactId?: number; questArtifact?: HexQuestArtifact; result?: HexPlayerResult;
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
      ![data.player.ap, data.player.maxAP, data.player.mp, data.player.maxMP, data.player.reactions, data.player.maxReactions, data.player.health, data.player.maxHealth,
        data.player.mobility, data.player.agility, data.player.attackRange].every(Number.isSafeInteger) ||
      typeof data.player.characterType !== "string" || typeof data.player.isRanged !== "boolean" ||
      !parsePosition(data.player.position, data.geometry!)
    )) return false;
    return true;
  } catch { return false; }
}

export type HexEngagementResult = HexAttackResult & { movement?: HexRoute; attacked: boolean; reason?: string };
