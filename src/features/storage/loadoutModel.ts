export type StoredItem = {
    name?: string;
    item_name?: string;
    item_count?: number;
    image?: string;
    description?: string;
    item_type?: string;
    rarity?: string;
};

export type StorageState = {
    inventory: Record<string, StoredItem>;
    loadout: Record<string, number>;
    eligibleKeys: string[];
    balance: number;
    equipment?: { name: string; type: string; status: string }[];
};

export type SaveStatus = "saving" | "saved" | "error";
export type QuantityAction = -5 | -1 | 1 | 5 | "zero" | "max";

export function eligibleItems(state: StorageState): [string, StoredItem][] {
    return [...new Set(state.eligibleKeys ?? [])]
        .filter((key) => (state.inventory[key]?.item_count ?? 0) > 0)
        .sort((a, b) => a.localeCompare(b))
        .map((key) => [key, state.inventory[key]]);
}

export function normalizedLoadout(state: StorageState): Record<string, number> {
    const eligible = new Set(eligibleItems(state).map(([key]) => key));
    return Object.fromEntries(
        Object.entries(state.loadout ?? {}).flatMap(([key, count]) => {
            const max = Math.min(9999, state.inventory[key]?.item_count ?? 0);
            return eligible.has(key) && Number.isInteger(count) && count > 0 && max > 0
                ? [[key, Math.min(Math.floor(count), max)]]
                : [];
        }),
    );
}

export function nextLoadout(
    current: Record<string, number>,
    key: string,
    action: QuantityAction,
    total: number,
): Record<string, number> {
    const max = Math.max(0, Math.min(9999, Math.floor(total)));
    const previous = current[key] ?? 0;
    const proposed = action === "zero" ? 0 : action === "max" ? max : previous + action;
    const count = Math.max(0, Math.min(max, proposed));
    if (count === previous) return current;
    const next = { ...current };
    if (count === 0) delete next[key];
    else next[key] = count;
    return next;
}

export function remainingCount(total: number, selected: number): number {
    return Math.max(0, total - selected);
}

/** Serializes full-loadout writes. A response for an older revision never marks a newer one saved. */
export class SerializedLoadoutSaver {
    private desired: Record<string, number> = {};
    private revision = 0;
    private savedRevision = 0;
    private inFlight: Promise<void> | null = null;
    private timer: ReturnType<typeof setTimeout> | null = null;
    private error: Error | null = null;
    private readonly save: (loadout: Record<string, number>) => Promise<void>;
    private readonly onStatus: (status: SaveStatus, error?: Error) => void;
    private readonly debounceMs: number;

    constructor(
        save: (loadout: Record<string, number>) => Promise<void>,
        onStatus: (status: SaveStatus, error?: Error) => void,
        debounceMs = 180,
    ) {
        this.save = save;
        this.onStatus = onStatus;
        this.debounceMs = debounceMs;
    }

    initialize(loadout: Record<string, number>): void {
        this.desired = { ...loadout };
        this.revision = 0;
        this.savedRevision = 0;
        this.error = null;
        this.onStatus("saved");
    }

    change(loadout: Record<string, number>): void {
        this.desired = { ...loadout };
        this.revision += 1;
        this.error = null;
        this.onStatus("saving");
        this.schedule();
    }

    private schedule(): void {
        if (this.timer) clearTimeout(this.timer);
        this.timer = setTimeout(() => {
            this.timer = null;
            void this.flush().catch(() => undefined);
        }, this.debounceMs);
    }

    private run(): Promise<void> {
        if (this.inFlight) return this.inFlight;
        this.inFlight = (async () => {
            while (this.savedRevision < this.revision) {
                const revision = this.revision;
                const payload = { ...this.desired };
                try {
                    await this.save(payload);
                } catch (reason) {
                    this.error = reason instanceof Error ? reason : new Error(String(reason));
                    this.onStatus("error", this.error);
                    throw this.error;
                }
                this.savedRevision = revision;
                if (revision === this.revision) {
                    this.error = null;
                    this.onStatus("saved");
                }
            }
        })().finally(() => { this.inFlight = null; });
        return this.inFlight;
    }

    async flush(): Promise<void> {
        if (this.timer) { clearTimeout(this.timer); this.timer = null; }
        if (this.savedRevision < this.revision && this.error) {
            this.error = null;
            this.onStatus("saving");
        }
        while (this.savedRevision < this.revision) {
            await this.run();
        }
        if (this.error) throw this.error;
    }

    dispose(): void {
        if (this.timer) clearTimeout(this.timer);
        this.timer = null;
    }
}
