export function getCellIndex(x: number, y: number, mapWidth: number): number {
    return y * mapWidth + x;
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
