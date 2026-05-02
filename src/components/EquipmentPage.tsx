"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import { useAuth } from "../contexts/AuthContext";
import LobbyHeader from "./LobbyHeader";
import styles from "../styles/EquipmentPage.module.css";
import { API_BASE } from "../utils/serviceUrls";
import type {
    ActiveSetBonus,
    EquipmentApiResponse,
    EquipmentBonuses,
    EquipmentItem,
    EquipmentSlot,
    EquipmentState,
    EquipmentStats,
    ItemHandedness,
    ItemRarity,
} from "../types/equipment";

const EQUIPMENT_SLOTS: EquipmentSlot[] = [
    "helmet",
    "chest",
    "pants",
    "boots",
    "gloves",
    "main_hand",
    "off_hand",
    "ring",
    "amulet",
];

const SLOT_LABELS: Record<EquipmentSlot, string> = {
    main_hand: "Main Hand",
    off_hand: "Off Hand",
    helmet: "Helmet",
    chest: "Chest",
    pants: "Pants",
    boots: "Boots",
    gloves: "Gloves",
    ring: "Ring",
    amulet: "Amulet",
};

const STAT_ROWS: Array<{ key: keyof EquipmentStats; label: string }> = [
    { key: "maxHealth", label: "Max Health" },
    { key: "maxEnergy", label: "Max Energy" },
    { key: "attack", label: "Attack" },
    { key: "defense", label: "Defense" },
    { key: "mobility", label: "Mobility" },
    { key: "agility", label: "Reflex" },
    { key: "sightRange", label: "Sight Range" },
    { key: "attackRange", label: "Attack Range" },
];

const BONUS_ROWS: Array<{ key: keyof EquipmentBonuses; label: string }> = [
    { key: "attack", label: "Attack" },
    { key: "defense", label: "Defense" },
    { key: "mobility", label: "Mobility" },
    { key: "agility", label: "Reflex" },
    { key: "maxHealth", label: "Max Health" },
    { key: "maxEnergy", label: "Max Energy" },
    { key: "sightRange", label: "Sight Range" },
    { key: "attackRange", label: "Attack Range" },
];

const EQUIPMENT_SLOT_SET = new Set<EquipmentSlot>(EQUIPMENT_SLOTS);
const RARITY_SET = new Set<ItemRarity>(["green", "blue", "purple", "orange"]);
const HANDEDNESS_SET = new Set<ItemHandedness>(["none", "one_hand", "two_hand"]);

const equipmentDevGrantEnabled =
    process.env.NEXT_PUBLIC_EQUIPMENT_DEV_GRANT_ENABLED === "true";

function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === "object" && value !== null;
}

function isEquipmentSlot(value: unknown): value is EquipmentSlot {
    return typeof value === "string" && EQUIPMENT_SLOT_SET.has(value as EquipmentSlot);
}

function normalizeRarity(value: unknown): ItemRarity {
    return typeof value === "string" && RARITY_SET.has(value as ItemRarity)
        ? (value as ItemRarity)
        : "green";
}

function normalizeHandedness(value: unknown): ItemHandedness {
    return typeof value === "string" && HANDEDNESS_SET.has(value as ItemHandedness)
        ? (value as ItemHandedness)
        : "none";
}

function numberOrZero(value: unknown): number {
    return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function numberOrUndefined(value: unknown): number | undefined {
    return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function stringOrEmpty(value: unknown): string {
    return typeof value === "string" ? value : "";
}

function normalizeBonuses(value: unknown): EquipmentBonuses {
    const raw = isRecord(value) ? value : {};
    return {
        attack: numberOrZero(raw.attack),
        defense: numberOrZero(raw.defense),
        mobility: numberOrZero(raw.mobility),
        agility: numberOrZero(raw.agility),
        maxHealth: numberOrZero(raw.maxHealth),
        maxEnergy: numberOrZero(raw.maxEnergy),
        sightRange: numberOrZero(raw.sightRange),
        attackRange: numberOrZero(raw.attackRange),
    };
}

function normalizeStats(value: unknown): EquipmentStats {
    const raw = isRecord(value) ? value : {};
    return {
        maxHealth: numberOrZero(raw.maxHealth),
        maxEnergy: numberOrZero(raw.maxEnergy),
        attack: numberOrZero(raw.attack),
        defense: numberOrZero(raw.defense),
        mobility: numberOrZero(raw.mobility),
        agility: numberOrZero(raw.agility),
        sightRange: numberOrZero(raw.sightRange),
        attackRange: numberOrZero(raw.attackRange),
    };
}

function normalizeItem(value: unknown): EquipmentItem | null {
    if (!isRecord(value)) return null;
    if (!isEquipmentSlot(value.slot)) return null;

    const instanceId = stringOrEmpty(value.instanceId);
    if (!instanceId) return null;

    const code = stringOrEmpty(value.code);
    const name = stringOrEmpty(value.name) || code || "Unknown item";
    const classRestriction =
        typeof value.classRestriction === "string" ? value.classRestriction : null;

    return {
        instanceId,
        templateId: numberOrZero(value.templateId),
        code,
        name,
        setCode: stringOrEmpty(value.setCode) || undefined,
        setName: stringOrEmpty(value.setName) || undefined,
        slot: value.slot,
        itemType: stringOrEmpty(value.itemType),
        handedness: normalizeHandedness(value.handedness),
        rarity: normalizeRarity(value.rarity),
        classRestriction,
        levelRequirement: numberOrUndefined(value.levelRequirement),
        imageUrl: stringOrEmpty(value.imageUrl),
        bonuses: normalizeBonuses(value.bonuses),
        status: stringOrEmpty(value.status) || undefined,
        version: numberOrUndefined(value.version),
    };
}

function normalizeActiveSetBonus(value: unknown): ActiveSetBonus | null {
    if (!isRecord(value)) return null;

    const description = stringOrEmpty(value.description);
    if (!description) return null;

    return {
        setCode: stringOrEmpty(value.setCode) || undefined,
        setName: stringOrEmpty(value.setName) || undefined,
        pieces: numberOrUndefined(value.pieces),
        piecesRequired: numberOrUndefined(value.piecesRequired),
        description,
        bonuses: normalizeBonuses(value.bonuses),
    };
}

function normalizeEquipmentState(value: unknown): EquipmentState {
    const raw = isRecord(value) ? value : {};

    const inventory = Array.isArray(raw.inventory)
        ? raw.inventory
              .map(normalizeItem)
              .filter((item): item is EquipmentItem => item !== null)
        : [];

    const equippedRaw = isRecord(raw.equipped) ? raw.equipped : {};
    const equipped: Partial<Record<EquipmentSlot, EquipmentItem>> = {};
    for (const slot of EQUIPMENT_SLOTS) {
        const item = normalizeItem(equippedRaw[slot]);
        if (item) {
            equipped[slot] = item;
        }
    }

    const activeSetBonuses = Array.isArray(raw.activeSetBonuses)
        ? raw.activeSetBonuses
              .map(normalizeActiveSetBonus)
              .filter((bonus): bonus is ActiveSetBonus => bonus !== null)
        : [];

    return {
        activeCharacterId: numberOrZero(raw.activeCharacterId),
        inventory,
        equipped,
        baseStats: normalizeStats(raw.baseStats),
        effectiveStats: normalizeStats(raw.effectiveStats),
        activeSetBonuses,
    };
}

function formatEnum(value: string): string {
    return value
        .split("_")
        .filter(Boolean)
        .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
        .join(" ");
}

function formatBonusValue(value: number): string {
    return value > 0 ? `+${value}` : `${value}`;
}

function getBonusEntries(bonuses?: EquipmentBonuses) {
    return BONUS_ROWS.map((row) => ({
        ...row,
        value: numberOrZero(bonuses?.[row.key]),
    })).filter((row) => row.value !== 0);
}

function getErrorMessage(error: unknown, fallback: string): string {
    if (error instanceof Error && error.message) {
        return error.message;
    }
    return fallback;
}

function readableApiError(payload: EquipmentApiResponse, status: number): string {
    if (payload.error) {
        return payload.error.replace(/_/g, " ");
    }
    return `Equipment request failed (${status})`;
}

function ItemImage({ item }: { item: EquipmentItem }) {
    const initials = item.name
        .split(" ")
        .map((part) => part[0])
        .join("")
        .slice(0, 2)
        .toUpperCase();

    return (
        <div className={styles.itemImageWrap} aria-hidden="true">
            <span className={styles.itemImageFallback}>{initials || "EQ"}</span>
            {item.imageUrl ? (
                <img
                    src={item.imageUrl}
                    alt=""
                    className={styles.itemImage}
                    draggable={false}
                    onError={(event) => {
                        event.currentTarget.style.display = "none";
                    }}
                />
            ) : null}
        </div>
    );
}

function BonusList({ bonuses }: { bonuses?: EquipmentBonuses }) {
    const entries = getBonusEntries(bonuses);
    if (entries.length === 0) {
        return <div className={styles.emptyBonus}>No stat bonuses</div>;
    }

    return (
        <div className={styles.bonusList}>
            {entries.map((entry) => (
                <span key={entry.key} className={styles.bonusPill}>
                    {entry.label} {formatBonusValue(entry.value)}
                </span>
            ))}
        </div>
    );
}

function ItemMeta({ item }: { item: EquipmentItem }) {
    return (
        <div className={styles.itemMeta}>
            <span>{formatEnum(item.itemType || item.slot)}</span>
            <span>{SLOT_LABELS[item.slot]}</span>
            <span>{formatEnum(item.handedness)}</span>
            {item.classRestriction ? <span>{formatEnum(item.classRestriction)}</span> : null}
            {item.levelRequirement ? <span>Level {item.levelRequirement}</span> : null}
        </div>
    );
}

function EquippedSlotCard({
    slot,
    item,
    blocked,
    busy,
    onUnequip,
}: {
    slot: EquipmentSlot;
    item?: EquipmentItem;
    blocked: boolean;
    busy: boolean;
    onUnequip: (slot: EquipmentSlot) => void;
}) {
    return (
        <article
            className={`${styles.slotCard} ${blocked ? styles.slotCardBlocked : ""}`}
        >
            <div className={styles.slotHeader}>
                <span className={styles.slotLabel}>{SLOT_LABELS[slot]}</span>
                {item ? (
                    <span className={`${styles.rarityBadge} ${styles[`rarity_${item.rarity}`]}`}>
                        {formatEnum(item.rarity)}
                    </span>
                ) : null}
            </div>

            {item ? (
                <div className={styles.equippedBody}>
                    <ItemImage item={item} />
                    <div className={styles.itemText}>
                        <h3>{item.name}</h3>
                        <ItemMeta item={item} />
                        <BonusList bonuses={item.bonuses} />
                    </div>
                    <button
                        type="button"
                        className={styles.secondaryButton}
                        onClick={() => onUnequip(slot)}
                        disabled={busy}
                    >
                        {busy ? "Unequipping..." : "Unequip"}
                    </button>
                </div>
            ) : (
                <div className={styles.emptySlot}>
                    {blocked ? "Blocked by two-handed weapon" : "Empty slot"}
                </div>
            )}
        </article>
    );
}

function InventoryCard({
    item,
    busy,
    onEquip,
}: {
    item: EquipmentItem;
    busy: boolean;
    onEquip: (item: EquipmentItem) => void;
}) {
    return (
        <article className={styles.inventoryCard}>
            <div className={styles.inventoryItemTop}>
                <ItemImage item={item} />
                <div className={styles.itemText}>
                    <div className={styles.itemTitleRow}>
                        <h3>{item.name}</h3>
                        <span className={`${styles.rarityBadge} ${styles[`rarity_${item.rarity}`]}`}>
                            {formatEnum(item.rarity)}
                        </span>
                    </div>
                    <ItemMeta item={item} />
                </div>
            </div>
            <BonusList bonuses={item.bonuses} />
            <button
                type="button"
                className={styles.primaryButton}
                onClick={() => onEquip(item)}
                disabled={busy}
            >
                {busy ? "Equipping..." : "Equip"}
            </button>
        </article>
    );
}

function StatsComparison({ state }: { state: EquipmentState }) {
    return (
        <div className={styles.statsList}>
            {STAT_ROWS.map((row) => {
                const baseValue = numberOrZero(state.baseStats[row.key]);
                const effectiveValue = numberOrZero(state.effectiveStats[row.key]);
                const changed = baseValue !== effectiveValue;

                return (
                    <div key={row.key} className={styles.statRow}>
                        <span>{row.label}</span>
                        <span className={styles.statNumbers}>
                            <span>{baseValue}</span>
                            <span className={styles.statArrow}>-&gt;</span>
                            <span className={changed ? styles.statChanged : ""}>
                                {effectiveValue}
                            </span>
                        </span>
                    </div>
                );
            })}
        </div>
    );
}

function ActiveSetBonuses({ bonuses }: { bonuses: ActiveSetBonus[] }) {
    if (bonuses.length === 0) {
        return <div className={styles.emptyState}>No active set bonuses.</div>;
    }

    return (
        <div className={styles.setBonusList}>
            {bonuses.map((bonus, index) => (
                <article
                    className={styles.setBonusCard}
                    key={`${bonus.setCode || bonus.setName || "set"}-${bonus.piecesRequired || index}`}
                >
                    <div className={styles.setBonusTitle}>
                        <span>{bonus.setName || bonus.setCode || "Equipment Set"}</span>
                        {bonus.pieces ? <span>{bonus.pieces} pieces</span> : null}
                    </div>
                    {bonus.piecesRequired ? (
                        <div className={styles.setBonusMeta}>
                            {bonus.piecesRequired} piece bonus
                        </div>
                    ) : null}
                    <p>{bonus.description}</p>
                    <BonusList bonuses={bonus.bonuses} />
                </article>
            ))}
        </div>
    );
}

export default function EquipmentPage() {
    const { user } = useAuth();
    const [equipment, setEquipment] = useState<EquipmentState | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [busyAction, setBusyAction] = useState<string | null>(null);

    const authHeaders = useMemo(
        () => ({
            Authorization: `Bearer ${user?.token || ""}`,
        }),
        [user?.token],
    );

    const readResponse = useCallback(async (response: Response, requireData = true) => {
        const text = await response.text();
        let payload: EquipmentApiResponse = {};
        if (text) {
            try {
                payload = JSON.parse(text) as EquipmentApiResponse;
            } catch {
                payload = {};
            }
        }

        if (!response.ok) {
            throw new Error(readableApiError(payload, response.status));
        }

        if (!payload.data) {
            if (requireData) {
                throw new Error("Equipment response did not include data");
            }
            return null;
        }

        return normalizeEquipmentState(payload.data);
    }, []);

    const loadEquipment = useCallback(async () => {
        if (!user?.token) return;

        setLoading(true);
        setError(null);
        try {
            const response = await fetch(`${API_BASE}/game/equipment`, {
                method: "GET",
                headers: authHeaders,
                cache: "no-store",
            });
            const nextState = await readResponse(response);
            if (nextState) {
                setEquipment(nextState);
            }
        } catch (err) {
            setError(getErrorMessage(err, "Failed to load equipment"));
        } finally {
            setLoading(false);
        }
    }, [authHeaders, readResponse, user?.token]);

    useEffect(() => {
        void loadEquipment();
    }, [loadEquipment]);

    const postEquipmentAction = useCallback(
        async (path: string, body: unknown, actionKey: string) => {
            if (!user?.token) return;

            setBusyAction(actionKey);
            setError(null);
            try {
                const response = await fetch(`${API_BASE}${path}`, {
                    method: "POST",
                    headers: {
                        ...authHeaders,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify(body),
                    cache: "no-store",
                });
                const nextState = await readResponse(response, false);
                if (nextState) {
                    setEquipment(nextState);
                } else {
                    await loadEquipment();
                }
            } catch (err) {
                setError(getErrorMessage(err, "Equipment action failed"));
            } finally {
                setBusyAction(null);
            }
        },
        [authHeaders, loadEquipment, readResponse, user?.token],
    );

    const handleEquip = useCallback(
        (item: EquipmentItem) => {
            if (!equipment?.activeCharacterId) return;
            void postEquipmentAction(
                "/game/equipment/equip",
                {
                    characterId: equipment.activeCharacterId,
                    itemInstanceId: item.instanceId,
                },
                `equip-${item.instanceId}`,
            );
        },
        [equipment?.activeCharacterId, postEquipmentAction],
    );

    const handleUnequip = useCallback(
        (slot: EquipmentSlot) => {
            if (!equipment?.activeCharacterId) return;
            void postEquipmentAction(
                "/game/equipment/unequip",
                {
                    characterId: equipment.activeCharacterId,
                    slot,
                },
                `unequip-${slot}`,
            );
        },
        [equipment?.activeCharacterId, postEquipmentAction],
    );

    const handleGrantSagecloth = useCallback(() => {
        void postEquipmentAction(
            "/game/equipment/grant-sagecloth-dev",
            {},
            "grant-sagecloth",
        );
    }, [postEquipmentAction]);

    const offHandBlocked =
        equipment?.equipped.main_hand?.handedness === "two_hand" &&
        !equipment?.equipped.off_hand;

    return (
        <main className={styles.pageRoot}>
            <div className={styles.container}>
                <LobbyHeader />

                <section className={styles.pageHeader}>
                    <div>
                        <h1>Equipment</h1>
                        <p>
                            Active character{" "}
                            {equipment?.activeCharacterId
                                ? `#${equipment.activeCharacterId}`
                                : "-"}
                        </p>
                    </div>
                    {equipmentDevGrantEnabled ? (
                        <button
                            type="button"
                            className={styles.devButton}
                            onClick={handleGrantSagecloth}
                            disabled={busyAction !== null}
                        >
                            {busyAction === "grant-sagecloth"
                                ? "Granting..."
                                : "Grant test Sagecloth Set"}
                        </button>
                    ) : null}
                </section>

                {error ? (
                    <section className={styles.errorPanel}>
                        <span>{error}</span>
                        <button
                            type="button"
                            className={styles.secondaryButton}
                            onClick={() => void loadEquipment()}
                        >
                            Retry
                        </button>
                    </section>
                ) : null}

                {loading && !equipment ? (
                    <section className={styles.statePanel}>Loading equipment...</section>
                ) : null}

                {equipment ? (
                    <>
                        <div className={styles.topGrid}>
                            <section className={styles.panel}>
                                <div className={styles.panelHeader}>
                                    <h2>Character Slots</h2>
                                </div>
                                <div className={styles.slotsGrid}>
                                    {EQUIPMENT_SLOTS.map((slot) => (
                                        <EquippedSlotCard
                                            key={slot}
                                            slot={slot}
                                            item={equipment.equipped[slot]}
                                            blocked={slot === "off_hand" && offHandBlocked}
                                            busy={busyAction === `unequip-${slot}`}
                                            onUnequip={handleUnequip}
                                        />
                                    ))}
                                </div>
                            </section>

                            <aside className={styles.panel}>
                                <div className={styles.panelHeader}>
                                    <h2>Stats</h2>
                                </div>
                                <StatsComparison state={equipment} />

                                <div className={styles.panelHeaderCompact}>
                                    <h2>Set Bonuses</h2>
                                </div>
                                <ActiveSetBonuses bonuses={equipment.activeSetBonuses} />
                            </aside>
                        </div>

                        <section className={styles.panel}>
                            <div className={styles.panelHeader}>
                                <h2>Inventory</h2>
                                <span>{equipment.inventory.length} items</span>
                            </div>

                            {equipment.inventory.length === 0 ? (
                                <div className={styles.emptyState}>
                                    No equipment items yet.
                                </div>
                            ) : (
                                <div className={styles.inventoryGrid}>
                                    {equipment.inventory.map((item) => (
                                        <InventoryCard
                                            key={item.instanceId}
                                            item={item}
                                            busy={busyAction === `equip-${item.instanceId}`}
                                            onEquip={handleEquip}
                                        />
                                    ))}
                                </div>
                            )}
                        </section>
                    </>
                ) : null}
            </div>
        </main>
    );
}
