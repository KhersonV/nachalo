import type {
    ActionLogEntryInput,
    Cell,
    GameState,
    Inventory,
    PlayerState,
    RawInventoryItem,
} from "@/types";
import type {
    CombatEffect,
    CombatExchangePayload,
    CombatStep,
    CombatTargetRef,
} from "@/types/combat";

type KnownWsPayload = Record<string, any>;

function asNumber(value: unknown): number | null {
    if (typeof value === "number" && Number.isFinite(value)) return value;
    if (typeof value === "string" && value.trim() !== "") {
        const parsed = Number(value);
        if (Number.isFinite(parsed)) return parsed;
    }
    return null;
}

function fieldNumber(source: unknown, ...keys: string[]): number | null {
    if (!source || typeof source !== "object") return null;
    const rec = source as Record<string, unknown>;
    for (const key of keys) {
        const value = asNumber(rec[key]);
        if (value !== null) return value;
    }
    return null;
}

function formatItemName(value: unknown, fallback: string) {
    if (typeof value !== "string") return fallback;
    const trimmed = value.trim();
    if (!trimmed) return fallback;
    return trimmed
        .replace(/[_-]+/g, " ")
        .replace(/\b\w/g, (letter) => letter.toUpperCase());
}

function getPayloadUserId(value: unknown): number | null {
    if (!value || typeof value !== "object") return null;
    const rec = value as Record<string, unknown>;
    return asNumber(rec.user_id ?? rec.userId ?? rec.player_id);
}

function getCurrentPlayer(state: GameState, currentUserId?: number) {
    if (!currentUserId) return null;
    return (
        state.players.find((player) => player.user_id === currentUserId) ?? null
    );
}

function combatRefMatchesCurrentPlayer(
    ref: CombatTargetRef | undefined,
    currentUserId?: number,
) {
    return !!currentUserId && ref?.type === "player" && ref.id === currentUserId;
}

export function isCombatExchangeRelevantToCurrentPlayer(
    payload: CombatExchangePayload,
    currentUserId?: number,
) {
    if (!currentUserId) return false;

    if (
        payload.attackerType === "player" &&
        payload.attackerId === currentUserId
    ) {
        return true;
    }
    if (payload.targetType === "player" && payload.targetId === currentUserId) {
        return true;
    }

    const hasRelevantStep = payload.steps.some((step) => {
        const source =
            "source" in step
                ? (step.source as CombatTargetRef | undefined)
                : undefined;
        return (
            combatRefMatchesCurrentPlayer(source, currentUserId) ||
            combatRefMatchesCurrentPlayer(step.target, currentUserId)
        );
    });
    if (hasRelevantStep) return true;

    if (
        payload.drops?.some(
            (drop) =>
                drop.ownerUserId === currentUserId ||
                drop.userId === currentUserId,
        ) ?? false
    ) {
        return true;
    }

    return (
        payload.effects?.some(
            (effect) => {
                return (
                    combatRefMatchesCurrentPlayer(effect.source, currentUserId) ||
                    combatRefMatchesCurrentPlayer(effect.target, currentUserId)
                );
            },
        ) ?? false
    );
}

function payloadMatchesCurrentPlayer(
    payload: KnownWsPayload,
    state: GameState,
    currentUserId?: number,
) {
    if (!currentUserId) return false;

    const directIds = [
        payload.userId,
        payload.user_id,
        payload.playerId,
        payload.player_id,
        payload.actorId,
        payload.actor_id,
        payload.attackerId,
        payload.attacker_id,
        payload.defenderId,
        payload.defender_id,
        payload.targetId,
        payload.target_id,
        payload.ownerId,
        payload.owner_id,
        payload.ownerUserId,
        payload.owner_user_id,
        getPayloadUserId(payload.updatedPlayer),
        getPayloadUserId(payload.player),
    ]
        .map(asNumber)
        .filter((value): value is number => value !== null);

    if (directIds.length > 0) {
        return directIds.includes(currentUserId);
    }

    const currentPlayer = getCurrentPlayer(state, currentUserId);
    const payloadPlayerName =
        typeof payload.playerName === "string" ? payload.playerName.trim() : "";
    return !!currentPlayer?.name && payloadPlayerName === currentPlayer.name;
}

function getPlayerName(
    state: GameState,
    userId: number,
    currentUserId?: number,
) {
    if (currentUserId && userId === currentUserId) return "You";
    return (
        state.players.find((player) => player.user_id === userId)?.name ??
        `Player ${userId}`
    );
}

function getActorLabel(
    state: GameState,
    ref: CombatTargetRef,
    currentUserId?: number,
) {
    if (ref.type === "player") {
        return getPlayerName(state, ref.id, currentUserId);
    }

    const monster = state.grid.find(
        (cell) =>
            cell.monster?.db_instance_id === ref.id ||
            cell.monster?.id === ref.id,
    )?.monster;

    return monster?.name ?? `Monster ${ref.id}`;
}

function actorVerb(label: string, verb: string) {
    return label === "You" ? `You ${verb}` : `${label} ${verb}`;
}

function targetLabel(label: string) {
    return label === "You" ? "you" : label;
}

function possessiveLabel(label: string) {
    return label === "You" ? "your" : `${label}'s`;
}

function cellFromPayload(state: GameState, payload: KnownWsPayload): Cell | null {
    const updatedCell = payload.updatedCell;
    const x = asNumber(updatedCell?.x);
    const y = asNumber(updatedCell?.y);
    if (x === null || y === null) return null;
    return state.grid.find((cell) => cell.x === x && cell.y === y) ?? null;
}

function inventoryItems(inventory?: Inventory | null) {
    const items: Array<{
        key: string;
        name: string;
        count: number;
        bucket: "resource" | "artifact";
    }> = [];

    const pushBucket = (
        bucketName: "resources" | "artifacts",
        bucketType: "resource" | "artifact",
    ) => {
        const bucket = inventory?.[bucketName] ?? {};
        Object.entries(bucket).forEach(([key, item]) => {
            const typed = item as RawInventoryItem | undefined;
            if (!typed) return;
            const itemKey = `${bucketType}:${typed.inventory_key ?? typed.item_id ?? key}`;
            items.push({
                key: itemKey,
                name: formatItemName(typed.name ?? key, bucketType),
                count: Number(typed.item_count ?? 0),
                bucket: bucketType,
            });
        });
    };

    pushBucket("resources", "resource");
    pushBucket("artifacts", "artifact");

    return items;
}

function findInventoryGain(
    before?: Inventory | null,
    after?: Inventory | null,
) {
    const beforeCounts = new Map(
        inventoryItems(before).map((item) => [item.key, item.count]),
    );

    return inventoryItems(after).find((item) => {
        return item.count > (beforeCounts.get(item.key) ?? 0);
    });
}

export function humanizeActionError(raw: unknown, fallback = "Action failed") {
    const text =
        typeof raw === "string"
            ? raw.replace(/^"+|"+$/g, "").trim()
            : String(raw ?? "").trim();
    const normalized = text.toLowerCase();

    if (!text) return fallback;
    if (normalized.includes("not your turn")) return "It's not your turn.";
    if (normalized.includes("недостаточно энергии")) {
        return "Not enough energy.";
    }
    if (normalized.includes("forbidden")) {
        return "Action blocked by session ownership.";
    }
    if (normalized.includes("target is already defeated")) {
        return "That target is already defeated.";
    }
    if (normalized.includes("attacker is already defeated")) {
        return "You are already defeated.";
    }
    if (normalized.includes("союзника")) {
        return "You cannot attack an ally.";
    }
    if (normalized.includes("сосед") || normalized.includes("adjacent")) {
        return "Move one adjacent tile at a time.";
    }
    if (
        normalized.includes("quest_artifact_missing") ||
        normalized.includes("quest artifact")
    ) {
        return "Find the quest artifact before entering the portal.";
    }
    if (normalized.includes("resource already collected")) {
        return "That resource was already collected.";
    }
    if (normalized.includes("бочка не найдена")) {
        return "There is no barrel on that tile.";
    }
    if (normalized.includes("match not found")) {
        return "Match is no longer available.";
    }

    return text.length > 120 ? `${text.slice(0, 117)}...` : text;
}

export function buildMoveLogEntry(
    payload: KnownWsPayload,
    currentUserId?: number,
): ActionLogEntryInput | null {
    const userId = asNumber(payload.userId ?? payload.playerId);
    if (!userId || (currentUserId && userId !== currentUserId)) return null;
    const x = asNumber(payload.newPosition?.x);
    const y = asNumber(payload.newPosition?.y);
    const suffix = x !== null && y !== null ? ` to ${x}:${y}` : "";

    return {
        category: "movement",
        tone: "info",
        message: `Moved${suffix}.`,
        dedupeKey: `move:${userId}:${x ?? "?"}:${y ?? "?"}`,
    };
}

export function buildTurnLogEntry(
    payload: KnownWsPayload,
    currentUserId?: number,
): ActionLogEntryInput | null {
    const activeUser = asNumber(payload.active_user ?? payload.userId);
    const turnNumber = asNumber(payload.turnNumber ?? payload.turn_number);
    if (!currentUserId || activeUser !== currentUserId) return null;

    return {
        category: "turn",
        tone: "info",
        message: "Your turn started.",
        dedupeKey: `turn:${turnNumber ?? "?"}:${activeUser}`,
    };
}

export function buildResourceLogEntry(
    payload: KnownWsPayload,
    state: GameState,
    currentUserId?: number,
): ActionLogEntryInput | null {
    if (!payloadMatchesCurrentPlayer(payload, state, currentUserId)) {
        return null;
    }

    const previousCell = cellFromPayload(state, payload);
    const resource = previousCell?.resource as any;
    const itemName = formatItemName(resource?.type, "resource");
    const isArtifact = resource?.item_type === "artifact";

    return {
        category: isArtifact ? "artifact" : "resource",
        tone: "success",
        message: isArtifact
            ? `Collected artifact: ${itemName}.`
            : `Collected resource: ${itemName}.`,
        dedupeKey: `resource:${payload.instanceId}:${previousCell?.x ?? "?"}:${previousCell?.y ?? "?"}`,
    };
}

export function buildBarrelLogEntry(
    payload: KnownWsPayload,
    state: GameState,
    eventType: "BARREL_RESOURCE" | "BARREL_ARTIFACT",
    currentUserId?: number,
): ActionLogEntryInput | null {
    if (!payloadMatchesCurrentPlayer(payload, state, currentUserId)) {
        return null;
    }

    const updatedPlayer = payload.updatedPlayer as PlayerState | undefined;
    const updatedPlayerId = getPayloadUserId(updatedPlayer);

    const previousPlayer = state.players.find(
        (player) => player.user_id === updatedPlayerId,
    );
    const gained = findInventoryGain(
        previousPlayer?.inventory,
        updatedPlayer?.inventory,
    );
    const isArtifact = eventType === "BARREL_ARTIFACT";
    const itemName = gained?.name ?? (isArtifact ? "an artifact" : "a reward");

    return {
        category: isArtifact ? "artifact" : "barrel",
        tone: "success",
        message: isArtifact
            ? `Barrel opened: found ${itemName}.`
            : `Barrel opened: gained ${itemName}.`,
        dedupeKey: `barrel:${payload.instanceId}:${payload.updatedCell?.x ?? "?"}:${payload.updatedCell?.y ?? "?"}`,
    };
}

export function buildBarrelDamageLogEntry(
    payload: KnownWsPayload,
    state: GameState,
    currentUserId?: number,
): ActionLogEntryInput | null {
    const userId = asNumber(payload.userId ?? payload.user_id);
    if (!userId) return null;
    if (!currentUserId || userId !== currentUserId) return null;
    const amount = asNumber(payload.amount);
    const hp = asNumber(payload.hp);
    const hpText = hp !== null ? ` HP: ${hp}.` : "";

    return {
        category: "barrel",
        tone: "danger",
        message: `Barrel trap hit you for ${amount ?? "?"} damage.${hpText}`,
        dedupeKey: `barrel-damage:${payload.instanceId}:${userId}:${amount ?? "?"}:${hp ?? "?"}`,
    };
}

function buildCombatStepMessage(
    step: CombatStep,
    state: GameState,
    currentUserId?: number,
) {
    if (step.kind === "death") {
        const target = getActorLabel(state, step.target, currentUserId);
        return target === "You"
            ? "You were defeated."
            : `${target} was defeated.`;
    }

    const source = step.source
        ? getActorLabel(state, step.source, currentUserId)
        : "An effect";
    const target = getActorLabel(state, step.target, currentUserId);
    const damage = fieldNumber(step, "damage", "Damage");
    const hpAfter = fieldNumber(step, "targetHpAfter", "target_hp_after");
    const damageText = damage !== null ? ` for ${damage} damage` : "";
    const tookDamageText =
        damage !== null ? `${damage} damage` : "damage";
    const hpText = hpAfter !== null ? ` HP: ${hpAfter}.` : "";

    switch (step.kind) {
        case "counter":
            return `${actorVerb(source, "counterattacked")} ${targetLabel(target)}${damageText}.${hpText}`;
        case "followup":
            return `${actorVerb(source, "followed up")} on ${targetLabel(target)}${damageText}.${hpText}`;
        case "guardianResponse":
            return `${actorVerb(source, "answered the counterattack")} against ${targetLabel(target)}${damageText}.${hpText}`;
        case "bonus":
            return `${actorVerb(source, "landed a bonus hit")} on ${targetLabel(target)}${damageText}.${hpText}`;
        case "auraExit":
            return `${target} took ${tookDamageText} from aura pressure.${hpText}`;
        case "artifactCurse":
            return `${target} took ${tookDamageText} from the Dragon Eye curse.${hpText}`;
        case "artifactBacklash":
            return `${target} took ${tookDamageText} from the Fire Amulet backlash.${hpText}`;
        default:
            return `${actorVerb(source, "hit")} ${targetLabel(target)}${damageText}.${hpText}`;
    }
}

function buildCombatEffectMessage(
    effect: CombatEffect,
    state: GameState,
    currentUserId?: number,
) {
    if (!effect.succeeded) return null;

    const source = effect.source
        ? getActorLabel(state, effect.source, currentUserId)
        : "Effect";
    const target = effect.target
        ? getActorLabel(state, effect.target, currentUserId)
        : "the target";

    if (effect.kind === "armorBreak") {
        const amount =
            typeof effect.value === "number" ? ` by ${effect.value}` : "";
        return `${actorVerb(source, "broke")} ${possessiveLabel(target)} armor${amount}.`;
    }

    if (effect.kind === "energyDrain") {
        const drained =
            typeof effect.energyDrained === "number"
                ? ` ${effect.energyDrained} energy`
                : " energy";
        const gained =
            typeof effect.energyGranted === "number"
                ? ` and gained ${effect.energyGranted}`
                : "";
        return `${actorVerb(source, "drained")}${drained} from ${targetLabel(target)}${gained}.`;
    }

    if (effect.kind === "push") {
        return `${actorVerb(source, "pushed")} ${targetLabel(target)}.`;
    }

    if (effect.kind === "block") {
        return `${targetLabel(target)} blocked the attack.`;
    }

    if (effect.kind === "lifesteal") {
        if (typeof effect.amount !== "number" || effect.amount <= 0) {
            return null;
        }
        return `${actorVerb(source, "restored")} ${effect.amount} HP with Blood Feast.`;
    }

    if (effect.kind === "crit") {
        return `${actorVerb(source, "landed")} a Critical Shot on ${targetLabel(target)}.`;
    }

    if (effect.kind === "arcaneOverburn") {
        return `${actorVerb(source, "triggered")} Arcane Overburn on ${targetLabel(target)}.`;
    }

    if (effect.kind === "pureDamage") {
        if (typeof effect.amount !== "number" || effect.amount <= 0) {
            return null;
        }
        return `${actorVerb(source, "dealt")} ${effect.amount} pure damage to ${targetLabel(target)}.`;
    }

    return null;
}

export function buildCombatLogEntries(
    payload: CombatExchangePayload,
    state: GameState,
    currentUserId?: number,
): ActionLogEntryInput[] {
    if (!isCombatExchangeRelevantToCurrentPlayer(payload, currentUserId)) {
        return [];
    }

    const entries: ActionLogEntryInput[] = [];
    const dropEntries: ActionLogEntryInput[] = [];

    payload.steps.forEach((step, index) => {
        const damage = fieldNumber(step, "damage", "Damage");
        if (step.kind !== "death" && damage !== null && damage <= 0) return;
        const message = buildCombatStepMessage(step, state, currentUserId);
        const isDeath = step.kind === "death";
        entries.push({
            category:
                step.kind === "counter"
                    ? "counterattack"
                    : isDeath
                      ? "defeat"
                      : "attack",
            tone:
                isDeath && step.target.type === "monster"
                    ? "success"
                    : isDeath || step.target.id === currentUserId
                      ? "danger"
                      : "info",
            message,
            dedupeKey: isDeath
                ? step.target.type === "player"
                    ? `defeat:${payload.instanceId}:${step.target.id}`
                    : `defeat:${payload.instanceId}:monster:${step.target.id}`
                : `combat:${payload.exchangeId}:step:${index}`,
        });
    });

    payload.effects?.forEach((effect, index) => {
        const message = buildCombatEffectMessage(effect, state, currentUserId);
        if (!message) return;
        entries.push({
            category: "effect",
            tone:
                effect.kind === "energyDrain" ||
                effect.kind === "block" ||
                effect.kind === "lifesteal" ||
                effect.kind === "arcaneOverburn"
                    ? "success"
                    : "warning",
            message,
            dedupeKey: `combat:${payload.exchangeId}:effect:${index}`,
        });
    });

    payload.drops?.forEach((drop, index) => {
        if (
            !currentUserId ||
            (drop.ownerUserId !== currentUserId && drop.userId !== currentUserId)
        ) {
            return;
        }
        const itemName = formatItemName(
            drop.item?.name || drop.name || drop.item?.templateCode || drop.templateCode,
            "equipment",
        );
        dropEntries.push({
            category: "equipment",
            tone: "success",
            message: `Loot found: ${itemName}.`,
            dedupeKey: `equipment-drop:${payload.instanceId}:${drop.itemInstanceId || drop.instanceId || index}`,
        });
    });

    return [
        ...entries.slice(0, Math.max(0, 6 - dropEntries.length)),
        ...dropEntries,
    ].slice(0, 6);
}

export function buildEquipmentDroppedLogEntry(
    payload: KnownWsPayload,
    _state: GameState,
    currentUserId?: number,
): ActionLogEntryInput | null {
    const ownerUserId = asNumber(
        payload.ownerUserId ?? payload.userId ?? payload.owner_user_id,
    );
    if (!currentUserId || ownerUserId !== currentUserId) return null;

    const item =
        payload.item && typeof payload.item === "object"
            ? (payload.item as Record<string, unknown>)
            : {};
    const itemName = formatItemName(
        item.name ?? payload.name ?? item.templateCode ?? payload.templateCode,
        "equipment",
    );
    const itemInstanceId =
        item.itemInstanceId ??
        payload.itemInstanceId ??
        payload.instanceItemId ??
        payload.dropInstanceId;

    return {
        category: "equipment",
        tone: "success",
        message: `You found ${itemName}.`,
        dedupeKey: `equipment-drop:${payload.instanceId ?? "match"}:${String(itemInstanceId ?? itemName)}`,
    };
}

export function buildPlayerDefeatedLogEntry(
    payload: KnownWsPayload,
    state: GameState,
    currentUserId?: number,
): ActionLogEntryInput | null {
    const userId = asNumber(payload.userId ?? payload.user_id);
    if (!userId) return null;
    if (!currentUserId || userId !== currentUserId) return null;
    const actor = getPlayerName(state, userId, currentUserId);

    return {
        category: "defeat",
        tone: userId === currentUserId ? "danger" : "warning",
        message: actor === "You" ? "You were defeated." : `${actor} was defeated.`,
        dedupeKey: `defeat:${payload.instanceId}:${userId}`,
    };
}

export function buildQuestArtifactLogEntry(
    payload: KnownWsPayload,
    state: GameState,
    currentUserId?: number,
): ActionLogEntryInput | null {
    if (!payloadMatchesCurrentPlayer(payload, state, currentUserId)) {
        return null;
    }

    const playerName = payload.playerName || "A player";
    return {
        category: "artifact",
        tone: "success",
        message: `${playerName} found the quest artifact.`,
        dedupeKey: `quest-artifact:${payload.instanceId}:${payload.playerName ?? "unknown"}`,
    };
}

export function buildPortalLogEntry(
    payload: KnownWsPayload,
    state: GameState,
    currentUserId?: number,
): ActionLogEntryInput | null {
    if (!payloadMatchesCurrentPlayer(payload, state, currentUserId)) {
        return null;
    }

    const playerName = payload.playerName || "A player";
    return {
        category: "portal",
        tone: "success",
        message: `${playerName} escaped through the portal.`,
        dedupeKey: `portal:${payload.instanceId}:${payload.playerName ?? "unknown"}`,
    };
}

export function buildMatchEndedLogEntry(
    payload: KnownWsPayload,
): ActionLogEntryInput {
    const winnerId = asNumber(payload.winnerId);
    const winnerText =
        winnerId && payload.winnerType === "group"
            ? ` Winning team: ${winnerId}.`
            : winnerId
              ? ` Winner: player ${winnerId}.`
              : "";

    return {
        category: "match",
        tone: "info",
        message: `Match ended.${winnerText}`,
        dedupeKey: `match-ended:${payload.instanceId}`,
    };
}
