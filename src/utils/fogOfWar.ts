import type { PlayerState } from "@/types";

export type VisibilitySource = {
    position: { x: number; y: number };
    sightRange: number;
};

export function getCellIndex(x: number, y: number, mapWidth: number): number {
    return y * mapWidth + x;
}

export function buildTeamExplorationBaseKey(
    instanceId: string,
    myPlayerId?: number,
    myGroupId?: number,
) {
    if (!instanceId || !myPlayerId) {
        return "";
    }

    if (typeof myGroupId === "number" && myGroupId > 0) {
        return `fog-explored:${instanceId}:team:${myGroupId}`;
    }

    return `fog-explored:${instanceId}:player:${myPlayerId}`;
}

export function isSameTeamPlayer(
    player: Pick<PlayerState, "user_id" | "group_id">,
    myPlayerId?: number,
    myGroupId?: number,
) {
    if (!myPlayerId) {
        return false;
    }

    if (player.user_id === myPlayerId) {
        return true;
    }

    return (
        typeof myGroupId === "number" &&
        myGroupId > 0 &&
        player.group_id === myGroupId
    );
}

export function collectVisibleCellIndices(
    visibilitySources: VisibilitySource[],
    mapWidth: number,
    mapHeight: number,
) {
    const next = new Set<number>();

    if (mapWidth <= 0 || mapHeight <= 0) {
        return next;
    }

    for (const source of visibilitySources) {
        const sightRange = Math.max(0, Math.floor(source.sightRange));
        const minX = Math.max(0, source.position.x - sightRange);
        const maxX = Math.min(mapWidth - 1, source.position.x + sightRange);
        const minY = Math.max(0, source.position.y - sightRange);
        const maxY = Math.min(mapHeight - 1, source.position.y + sightRange);

        for (let y = minY; y <= maxY; y++) {
            for (let x = minX; x <= maxX; x++) {
                next.add(getCellIndex(x, y, mapWidth));
            }
        }
    }

    return next;
}

export function buildExplorationStorageKey(
    storageKey: string,
    mapWidth: number,
    mapHeight: number,
) {
    return storageKey ? `${storageKey}:${mapWidth}x${mapHeight}` : "";
}

export function loadExploredCells(storageKey: string): Set<number> {
    if (!storageKey || typeof window === "undefined") {
        return new Set<number>();
    }

    try {
        const raw = window.sessionStorage.getItem(storageKey);
        if (!raw) return new Set<number>();

        const parsed = JSON.parse(raw);
        if (!Array.isArray(parsed)) return new Set<number>();

        return new Set<number>(
            parsed.filter(
                (value): value is number =>
                    Number.isInteger(value) && value >= 0,
            ),
        );
    } catch {
        return new Set<number>();
    }
}

export function persistExploredCells(
    storageKey: string,
    exploredCells: Set<number>,
) {
    if (!storageKey || typeof window === "undefined") {
        return;
    }

    try {
        const encoded = JSON.stringify(
            Array.from(exploredCells.values()).sort((a, b) => a - b),
        );
        window.sessionStorage.setItem(storageKey, encoded);
    } catch {
        // ignore storage failures
    }
}
