//====================================
// src/components/ObjectiveTracker.tsx
//====================================

"use client";

import React from "react";
import type { Cell, PlayerState, RawInventoryItem } from "../types";
import styles from "../styles/ObjectiveTracker.module.css";

type ObjectivePhase =
    | "loading"
    | "find-artifact"
    | "find-portal"
    | "enter-portal"
    | "dead"
    | "escaped"
    | "finished";

interface ObjectiveTrackerProps {
    player?: PlayerState;
    players: PlayerState[];
    grid: Cell[];
    mode: string;
    questArtifactId: number;
    isMyTurn: boolean;
    isPlayerDefeated: boolean;
    hasEscaped: boolean;
    isMatchFinished: boolean;
}

type InventoryEntry = RawInventoryItem & {
    inventory_key?: string;
};

function coercePositiveInt(value: unknown): number {
    if (typeof value === "number" && Number.isFinite(value)) {
        return Math.max(0, Math.trunc(value));
    }

    if (typeof value === "string" && value.trim() !== "") {
        const parsed = Number(value);
        if (Number.isFinite(parsed)) {
            return Math.max(0, Math.trunc(parsed));
        }
    }

    return 0;
}

function normalizeInventoryEntry(value: any, keyHint?: string): InventoryEntry {
    const [hintType, hintId] = keyHint?.split("_") ?? [];
    const explicitType =
        value?.item_type === "resource" ||
        value?.item_type === "artifact" ||
        value?.item_type === "scroll"
            ? value.item_type
            : undefined;
    const itemType =
        explicitType ||
        (hintType === "resource" ||
        hintType === "artifact" ||
        hintType === "scroll"
            ? hintType
            : "resource");
    const itemId =
        coercePositiveInt(value?.item_id) ||
        coercePositiveInt(value?.id) ||
        coercePositiveInt(hintId);

    return {
        item_type: itemType,
        item_id: itemId,
        name: value?.name || value?.item_name || `${itemType}_${itemId}`,
        item_count: coercePositiveInt(value?.item_count ?? value?.count ?? 1),
        image: value?.image || value?.image_url || "",
        description: value?.description || value?.item_description || "",
        bonus: value?.bonus,
        effect: value?.effect,
        inventory_key: keyHint,
    };
}

function flattenInventory(inventory: unknown): InventoryEntry[] {
    let parsed = inventory;

    if (typeof inventory === "string") {
        try {
            parsed = JSON.parse(inventory);
        } catch {
            return [];
        }
    }

    if (!parsed || typeof parsed !== "object") {
        return [];
    }

    if (Array.isArray(parsed)) {
        return parsed.map((entry) => normalizeInventoryEntry(entry));
    }

    const record = parsed as Record<string, any>;
    const groupedEntries =
        record.resources || record.artifacts || record.scrolls
            ? [
                  ...Object.entries(record.resources ?? {}),
                  ...Object.entries(record.artifacts ?? {}),
                  ...Object.entries(record.scrolls ?? {}),
              ]
            : Object.entries(record);

    return groupedEntries.map(([key, value]) =>
        normalizeInventoryEntry(value, key),
    );
}

function playerHasQuestArtifact(
    player: PlayerState | undefined,
    questArtifactId: number,
) {
    if (!player) return false;
    if (questArtifactId <= 0) return true;

    return flattenInventory(player.inventory).some(
        (item) =>
            item.item_type === "artifact" &&
            item.item_id === questArtifactId &&
            item.item_count > 0,
    );
}

function manhattanDistance(
    a: { x: number; y: number },
    b: { x: number; y: number },
) {
    return Math.abs(a.x - b.x) + Math.abs(a.y - b.y);
}

function resolveWinConditionText(
    mode: string,
    player: PlayerState | undefined,
    players: PlayerState[],
) {
    const normalizedMode = mode.trim().toUpperCase();
    if (!normalizedMode || normalizedMode === "PVE") {
        return "Escape with the quest artifact.";
    }

    const groupId = player?.group_id ?? 0;
    const teammates =
        groupId > 0
            ? players.filter((candidate) => candidate.group_id === groupId)
            : [];

    if (teammates.length > 1) {
        return "Your team wins when any teammate escapes with the artifact. Enemy teams can do the same.";
    }

    return "Any player can win by escaping with the artifact first.";
}

function phaseText(phase: ObjectivePhase, portalDistance: number | null) {
    switch (phase) {
        case "dead":
            return {
                title: "You are defeated",
                detail: "The match is over for your character.",
            };
        case "escaped":
            return {
                title: "Escaped through the portal",
                detail: "Your match result is ready.",
            };
        case "finished":
            return {
                title: "Match finished",
                detail: "Results are ready.",
            };
        case "find-artifact":
            return {
                title: "Find the quest artifact",
                detail: "Open barrels and collect the quest item before heading to the exit.",
            };
        case "enter-portal":
            return {
                title: "Enter the portal to escape",
                detail:
                    portalDistance === 0
                        ? "You are at the exit."
                        : "The exit is next to you.",
            };
        case "find-portal":
            return {
                title: "Find the portal",
                detail: "Carry the quest artifact to the exit.",
            };
        case "loading":
        default:
            return {
                title: "Loading objective",
                detail: "Preparing match state.",
            };
    }
}

function getStepState(
    step: "artifact" | "portal" | "escape",
    phase: ObjectivePhase,
) {
    if (phase === "dead" || phase === "escaped" || phase === "finished") {
        return styles.stepDone;
    }

    switch (step) {
        case "artifact":
            if (phase === "find-artifact") return styles.stepActive;
            if (phase !== "loading") return styles.stepDone;
            return "";
        case "portal":
            if (phase === "find-portal") return styles.stepActive;
            if (phase === "enter-portal") return styles.stepDone;
            return "";
        case "escape":
            if (phase === "enter-portal") return styles.stepActive;
            return "";
        default:
            return "";
    }
}

export default React.memo(function ObjectiveTracker({
    player,
    players,
    grid,
    mode,
    questArtifactId,
    isMyTurn,
    isPlayerDefeated,
    hasEscaped,
    isMatchFinished,
}: ObjectiveTrackerProps) {
    const hasQuestArtifact = React.useMemo(
        () => playerHasQuestArtifact(player, questArtifactId),
        [player?.inventory, player?.user_id, questArtifactId],
    );
    const portalCell = React.useMemo(
        () => grid.find((cell) => cell.isPortal) ?? null,
        [grid],
    );
    const portalDistance = React.useMemo(() => {
        if (!player || !portalCell) return null;
        return manhattanDistance(player.position, portalCell);
    }, [
        player?.position.x,
        player?.position.y,
        portalCell?.x,
        portalCell?.y,
    ]);

    const phase: ObjectivePhase = React.useMemo(() => {
        if (hasEscaped) return "escaped";
        if (isPlayerDefeated || (player && player.health <= 0)) return "dead";
        if (isMatchFinished) return "finished";
        if (!player) return "loading";
        if (!hasQuestArtifact) return "find-artifact";
        if (portalDistance !== null && portalDistance <= 1) {
            return "enter-portal";
        }
        return "find-portal";
    }, [
        hasEscaped,
        isPlayerDefeated,
        isMatchFinished,
        player?.health,
        player?.user_id,
        hasQuestArtifact,
        portalDistance,
    ]);

    const copy = phaseText(phase, portalDistance);
    const winCondition = React.useMemo(
        () => resolveWinConditionText(mode, player, players),
        [mode, player?.group_id, players],
    );

    return (
        <section className={styles.panel} aria-live="polite">
            <span
                className={`${styles.turnChip} ${
                    isMyTurn ? styles.turnChipActive : styles.turnChipWaiting
                }`}
            >
                {isMyTurn ? "YOUR TURN" : "WAITING"}
            </span>
            <div className={styles.kicker}>Objective</div>
            <h2 className={styles.title}>{copy.title}</h2>
            <p className={styles.detail}>{copy.detail}</p>
            <p className={styles.winCondition}>{winCondition}</p>
            <div className={styles.steps} aria-hidden="true">
                <span
                    className={`${styles.step} ${getStepState(
                        "artifact",
                        phase,
                    )}`}
                />
                <span
                    className={`${styles.step} ${getStepState(
                        "portal",
                        phase,
                    )}`}
                />
                <span
                    className={`${styles.step} ${getStepState(
                        "escape",
                        phase,
                    )}`}
                />
            </div>
        </section>
    );
});
