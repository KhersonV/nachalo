"use client";

import UiText from "@/components/UiText";

import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useAuth } from "../contexts/AuthContext";
import { getHeroFullbody, getHeroPortrait } from "@/features/heroes/heroArtwork";
import { BONUS_FIELDS, compareItemBonuses, equipBlockReason, filterEquipmentItems, getAvailableEquipment, getGearCount, isTwoHanded, type InventoryFilters } from "@/features/equipment/equipmentView";
import ItemBonusComparison from "./equipment/ItemBonusComparison";
import styles from "../styles/EquipmentPage.module.css";
import { API_BASE } from "../utils/serviceUrls";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";
import type {
    ActiveSetBonus,
    CharacterEquipmentStats,
    EquipmentApiResponse,
    EquipmentBonuses,
    EquipmentCharacter,
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

const CLASS_SET_BY_HERO: Record<string, string> = {
    guardian: "aegiswarden_set",
    berserker: "bloodroot_set",
    ranger: "greenwisp_set",
    mystic: "sagecloth_set",
};

const CLASS_LABELS: Record<string, string> = {
    guardian: "Guardian",
    berserker: "Berserker",
    ranger: "Ranger",
    mystic: "Mystic",
};

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
        isTwoHanded: value.isTwoHanded === true,
        rarity: normalizeRarity(value.rarity),
        classRestriction,
        levelRequirement: numberOrUndefined(value.levelRequirement),
        imageUrl: stringOrEmpty(value.imageUrl),
        bonuses: normalizeBonuses(value.bonuses),
        sellPrice: numberOrUndefined(value.sellPrice),
        status: stringOrEmpty(value.status) || undefined,
        version: numberOrUndefined(value.version),
        equippedCharacterId: numberOrUndefined(value.equippedCharacterId),
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

function normalizeCharacter(value: unknown): EquipmentCharacter | null {
    if (!isRecord(value)) return null;
    const characterId = numberOrZero(value.characterId);
    const heroClassId = stringOrEmpty(value.heroClassId);
    if (!characterId || !heroClassId) return null;

    return {
        characterId,
        heroClassId,
        name: stringOrEmpty(value.name) || CLASS_LABELS[heroClassId] || formatEnum(heroClassId),
        level: numberOrZero(value.level) || 1,
    };
}

function normalizeEquippedByCharacter(value: unknown) {
    const raw = isRecord(value) ? value : {};
    const result: Record<string, Partial<Record<EquipmentSlot, EquipmentItem>>> = {};
    for (const [characterId, slotsRaw] of Object.entries(raw)) {
        if (!isRecord(slotsRaw)) continue;
        const equipped: Partial<Record<EquipmentSlot, EquipmentItem>> = {};
        for (const slot of EQUIPMENT_SLOTS) {
            const item = normalizeItem(slotsRaw[slot]);
            if (item) equipped[slot] = item;
        }
        result[characterId] = equipped;
    }
    return result;
}

function normalizeStatsByCharacter(value: unknown) {
    const raw = isRecord(value) ? value : {};
    const result: Record<string, CharacterEquipmentStats> = {};
    for (const [characterId, statsRaw] of Object.entries(raw)) {
        if (!isRecord(statsRaw)) continue;
        result[characterId] = {
            baseStats: normalizeStats(statsRaw.baseStats),
            bonusStats: normalizeBonuses(statsRaw.bonusStats),
            effectiveStats: normalizeStats(statsRaw.effectiveStats),
        };
    }
    return result;
}

function normalizeSetBonusesByCharacter(value: unknown) {
    const raw = isRecord(value) ? value : {};
    const result: Record<string, ActiveSetBonus[]> = {};
    for (const [characterId, bonusesRaw] of Object.entries(raw)) {
        if (!Array.isArray(bonusesRaw)) continue;
        result[characterId] = bonusesRaw
            .map(normalizeActiveSetBonus)
            .filter((bonus): bonus is ActiveSetBonus => bonus !== null);
    }
    return result;
}

function normalizeEquipmentState(value: unknown): EquipmentState {
    const raw = isRecord(value) ? value : {};

    const characters = Array.isArray(raw.characters)
        ? raw.characters
              .map(normalizeCharacter)
              .filter((character): character is EquipmentCharacter => character !== null)
        : [];

    const inventory = Array.isArray(raw.inventory)
        ? raw.inventory
              .map(normalizeItem)
              .filter((item): item is EquipmentItem => item !== null)
        : [];
    const inventoryItems = Array.isArray(raw.inventoryItems)
        ? raw.inventoryItems
              .map(normalizeItem)
              .filter((item): item is EquipmentItem => item !== null)
        : inventory;
    const availableEquipmentKnown = Array.isArray(raw.inventoryItems) || Array.isArray(raw.inventory);
    const ownedItems = Array.isArray(raw.ownedItems)
        ? raw.ownedItems
              .map(normalizeItem)
              .filter((item): item is EquipmentItem => item !== null)
        : inventory;

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
        activeHeroClassId: stringOrEmpty(raw.activeHeroClassId) || undefined,
        characters,
        inventory,
        inventoryItems,
        availableEquipmentKnown,
        ownedItems,
        equipped,
        equippedByCharacter: normalizeEquippedByCharacter(raw.equippedByCharacter),
        baseStats: normalizeStats(raw.baseStats),
        effectiveStats: normalizeStats(raw.effectiveStats),
        activeSetBonuses,
        statsByCharacter: normalizeStatsByCharacter(raw.statsByCharacter),
        activeSetBonusesByCharacter: normalizeSetBonusesByCharacter(
            raw.activeSetBonusesByCharacter,
        ),
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
    return BONUS_FIELDS.map((row) => ({
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
        const messages: Record<string, string> = {
            class_restricted: "This item cannot be equipped by the selected class.",
            level_too_low: "This hero does not meet the item's level requirement.",
            off_hand_blocked_by_two_handed_weapon: "Remove the two-handed weapon before equipping an off-hand item.",
            item_is_equipped: "This item is already equipped.",
            item_not_in_inventory: "This item is no longer available in your inventory.",
            item_unavailable: "This item is currently unavailable.",
        };
        return messages[payload.error] || payload.error.replace(/_/g, " ");
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
        return <div className={styles.emptyBonus}><UiText text="No stat bonuses" /></div>;
    }

    return (
        <div className={styles.bonusList}>
            {entries.map((entry) => (
                <span key={entry.key} className={styles.bonusPill}>
                    <UiText text={entry.label} /> {formatBonusValue(entry.value)}
                </span>
            ))}
        </div>
    );
}

function ItemMeta({ item }: { item: EquipmentItem }) {
    const itemTypeLabel = formatEnum(item.itemType || item.slot);
    const slotLabel = SLOT_LABELS[item.slot];
    return (
        <div className={styles.itemMeta}>
            <span><UiText text={itemTypeLabel} /></span>
            {itemTypeLabel.toLowerCase() !== slotLabel.toLowerCase() && <span><UiText text={slotLabel} /></span>}
            {item.handedness !== "none" && <span><UiText text={formatEnum(item.handedness)} /></span>}
            {item.classRestriction ? <span><UiText text={formatEnum(item.classRestriction)} /></span> : null}
            {item.levelRequirement ? <span><UiText text="Level" /> {item.levelRequirement}</span> : null}
        </div>
    );
}

function EquippedSlotCard({
    slot,
    item,
    blocked,
    blockedBy,
    busy,
    selected,
    error,
    onSelect,
    onUnequip,
}: {
    slot: EquipmentSlot;
    item?: EquipmentItem;
    blocked: boolean;
    blockedBy?: string;
    busy: boolean;
    selected: boolean;
    error?: string;
    onSelect: (slot: EquipmentSlot) => void;
    onUnequip: (slot: EquipmentSlot) => void;
}) {
    return (
        <article className={`${styles.slotCard} ${blocked ? styles.slotCardBlocked : ""} ${selected ? styles.slotCardSelected : ""}`} data-rarity={item?.rarity}>
            <button type="button" className={styles.slotPrimary} aria-pressed={selected} onClick={() => onSelect(slot)}>
                {item ? <ItemImage item={item} /> : <span className={styles.slotGlyph}>{blocked ? "🔒" : SLOT_LABELS[slot].slice(0, 2).toUpperCase()}</span>}
                <span className={styles.slotIdentity}>
                    <span className={styles.slotLabel}><UiText text={SLOT_LABELS[slot]} /></span>
                    <strong title={item?.name || (blocked ? `Blocked by ${blockedBy || "two-handed weapon"}` : undefined)}>{item?.name || (blocked ? <><UiText text="Blocked by" /> {blockedBy || <UiText text="two-handed weapon" />}</> : <UiText text="Empty slot" />)}</strong>
                    {item && <small><UiText text={formatEnum(item.rarity)} /></small>}
                </span>
            </button>
            {item && <button type="button" className={styles.slotUnequip} onClick={() => onUnequip(slot)} disabled={busy}><UiText text={busy ? "Working…" : "Unequip"} /></button>}
            {error && <span className={styles.inlineError} role="alert">{error}</span>}
        </article>
    );
}

function InventoryCard({
    item,
    selected,
    reason,
    onSelect,
}: {
    item: EquipmentItem;
    selected: boolean;
    reason: string | null;
    onSelect: (item: EquipmentItem) => void;
}) {
    return (
        <button type="button" className={`${styles.inventoryCard} ${selected ? styles.inventoryCardSelected : ""}`} data-rarity={item.rarity} aria-pressed={selected} onClick={() => onSelect(item)} title={item.name}>
            <ItemImage item={item} />
            <span className={styles.inventoryCardCopy}>
                <strong><UiText text={item.name} /></strong>
                <small><UiText text={SLOT_LABELS[item.slot]} /> · <UiText text={formatEnum(item.rarity)} /></small>
                {reason && <small className={styles.inventoryUnavailable}><UiText text={reason} /></small>}
            </span>
        </button>
    );
}

function StatsComparison({
    baseStats,
    effectiveStats,
}: {
    baseStats: EquipmentStats;
    effectiveStats: EquipmentStats;
}) {
    return (
        <div className={styles.statsList}>
            {STAT_ROWS.map((row) => {
                const baseValue = numberOrZero(baseStats[row.key]);
                const effectiveValue = numberOrZero(effectiveStats[row.key]);
                const changed = baseValue !== effectiveValue;

                return (
                    <div key={row.key} className={styles.statRow}>
                        <span><UiText text={row.label} /></span>
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
        return <div className={styles.emptyState}><UiText text="No active set bonuses." /></div>;
    }

    return (
        <div className={styles.setBonusList}>
            {bonuses.map((bonus, index) => (
                <article
                    className={styles.setBonusCard}
                    key={`${bonus.setCode || bonus.setName || "set"}-${bonus.piecesRequired || index}`}
                >
                    <div className={styles.setBonusTitle}>
                        <span><UiText text={bonus.setName || bonus.setCode || "Equipment Set"} /></span>
                        {bonus.pieces ? <span>{bonus.pieces} <UiText text="pieces" /></span> : null}
                    </div>
                    {bonus.piecesRequired ? (
                        <div className={styles.setBonusMeta}>
                            {bonus.piecesRequired} <UiText text="piece bonus" />
                        </div>
                    ) : null}
                    <p><UiText text={bonus.description} /></p>
                    <BonusList bonuses={bonus.bonuses} />
                </article>
            ))}
        </div>
    );
}

export default function EquipmentPage() {
    const { language } = useGameLanguage();
    const t = (source: string) => uiText(language, source);
    const { user } = useAuth();
    const [equipment, setEquipment] = useState<EquipmentState | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [busyAction, setBusyAction] = useState<string | null>(null);
    const busyActionRef = useRef(false);
    const [actionError, setActionError] = useState<{ key: string; message: string } | null>(null);
    const [selectedCharacterId, setSelectedCharacterId] = useState<number>(0);
    const [selectedItemId, setSelectedItemId] = useState<string | null>(null);
    const [selectedSlot, setSelectedSlot] = useState<EquipmentSlot | null>(null);
    const [filters, setFilters] = useState<InventoryFilters>({ slot: "all", heroClass: "all", rarity: "all", availableOnly: false });

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
                setSelectedCharacterId((current) => nextState.characters.some(character => character.characterId === current) ? current : nextState.activeCharacterId);
                return true;
            }
        } catch (err) {
            setError(getErrorMessage(err, "Failed to load equipment"));
        } finally {
            setLoading(false);
        }
        return false;
    }, [authHeaders, readResponse, user?.token]);

    useEffect(() => {
        void loadEquipment();
    }, [loadEquipment]);

    const postEquipmentAction = useCallback(
        async (path: string, body: unknown, actionKey: string) => {
            if (!user?.token || busyActionRef.current) return false;

            busyActionRef.current = true;
            setBusyAction(actionKey);
            setActionError(null);
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
                await readResponse(response, false);
                if (!await loadEquipment()) {
                    setEquipment(null);
                    setSelectedItemId(null);
                    throw new Error("Equipment changed, but the latest server state could not be loaded. Retry loading equipment.");
                }
                return true;
            } catch (err) {
                setActionError({ key: actionKey, message: getErrorMessage(err, "Equipment action failed") });
                return false;
            } finally {
                busyActionRef.current = false;
                setBusyAction(null);
            }
        },
        [authHeaders, loadEquipment, readResponse, user?.token],
    );

    const handleEquip = useCallback(
        (item: EquipmentItem) => {
            const characterId = selectedCharacterId || equipment?.activeCharacterId;
            if (!characterId) return;
            void postEquipmentAction(
                "/game/equipment/equip",
                {
                    characterId,
                    itemInstanceId: item.instanceId,
                },
                `equip-${item.instanceId}`,
            ).then((success) => { if (success) { setSelectedItemId(null); setSelectedSlot(item.slot); } });
        },
        [equipment?.activeCharacterId, postEquipmentAction, selectedCharacterId],
    );

    const handleUnequip = useCallback(
        (slot: EquipmentSlot) => {
            const characterId = selectedCharacterId || equipment?.activeCharacterId;
            if (!characterId) return;
            void postEquipmentAction(
                "/game/equipment/unequip",
                {
                    characterId,
                    slot,
                },
                `unequip-${slot}`,
            );
        },
        [equipment?.activeCharacterId, postEquipmentAction, selectedCharacterId],
    );
    const selectedCharacter = useMemo(() => {
        if (!equipment) return null;
        const characterId = selectedCharacterId || equipment.activeCharacterId;
        return equipment.characters.find((character) => character.characterId === characterId) || null;
    }, [equipment, selectedCharacterId]);

    const effectiveSelectedCharacterId =
        selectedCharacter?.characterId || equipment?.activeCharacterId || 0;
    const viewingActiveCharacter = effectiveSelectedCharacterId === equipment?.activeCharacterId;
    const selectedEquipped =
        equipment?.equippedByCharacter[String(effectiveSelectedCharacterId)] ||
        (viewingActiveCharacter ? equipment?.equipped : undefined) ||
        {};
    const selectedCharacterStats =
        equipment?.statsByCharacter[String(effectiveSelectedCharacterId)];
    const selectedBaseStats = selectedCharacterStats?.baseStats || (viewingActiveCharacter ? equipment?.baseStats : undefined) || {};
    const selectedEffectiveStats =
        selectedCharacterStats?.effectiveStats || (viewingActiveCharacter ? equipment?.effectiveStats : undefined) || {};
    const selectedSetBonuses =
        equipment?.activeSetBonusesByCharacter[String(effectiveSelectedCharacterId)] ||
        (viewingActiveCharacter ? equipment?.activeSetBonuses : undefined) ||
        [];
    const availableItems = useMemo(() => equipment ? getAvailableEquipment(equipment.availableEquipmentKnown ? equipment.inventoryItems : undefined, equipment.equippedByCharacter, equipment.equipped) : [], [equipment]);
    const availableSlots = useMemo(() => Array.from(new Set(availableItems.map(item => item.slot))), [availableItems]);
    const availableClasses = useMemo(() => Array.from(new Set(availableItems.map(item => item.classRestriction).filter((value): value is string => Boolean(value)))), [availableItems]);
    const availableRarities = useMemo(() => Array.from(new Set(availableItems.map(item => item.rarity))), [availableItems]);
    const filteredItems = useMemo(
        () => filterEquipmentItems(availableItems, filters, selectedCharacter, selectedEquipped),
        [availableItems, filters, selectedCharacter, selectedEquipped],
    );
    const selectedInventoryItem = availableItems.find(item => item.instanceId === selectedItemId) ?? null;
    const inspectedItem = selectedInventoryItem ?? (selectedSlot ? selectedEquipped[selectedSlot] ?? null : null);
    const comparedItem = selectedInventoryItem ? selectedEquipped[selectedInventoryItem.slot] : undefined;
    const itemComparison = compareItemBonuses(comparedItem, selectedInventoryItem ?? undefined);
    const selectedBlockReason = selectedInventoryItem ? equipBlockReason(selectedInventoryItem, selectedCharacter, selectedEquipped) : null;
    const { occupiedSlotsCount } = getGearCount(selectedEquipped);

    const handleGrantClassSet = useCallback(() => {
        const heroClassId =
            selectedCharacter?.heroClassId || equipment?.activeHeroClassId || "guardian";
        const setSlug = CLASS_SET_BY_HERO[heroClassId] || "";
        void postEquipmentAction(
            "/game/equipment/dev/grant-class-set",
            { heroClassId, setSlug },
            `grant-${heroClassId}`,
        );
    }, [equipment?.activeHeroClassId, postEquipmentAction, selectedCharacter]);

    const offHandBlocked =
        isTwoHanded(selectedEquipped.main_hand) &&
        !selectedEquipped.off_hand;

    return (
        <main className={styles.pageRoot}>
            <div className={styles.container}>
                <header className={styles.pageHeader}>
                    <div className={styles.headerCopy}>
                        <span className={styles.eyebrow}><UiText text="THE ARMORY" /></span>
                        <h1><UiText text="Inventory" /></h1>
                        <p><UiText text="Equip your heroes, inspect your gear, and prepare for the next battle." /></p>
                    </div>
                    <div className={styles.headerActions}>
                        <span>{equipment?.characters.length ?? 0} <UiText text="owned heroes ·" /> {equipment?.ownedItems.length ?? 0} <UiText text="items" /></span>
                        {equipmentDevGrantEnabled && <button type="button" className={styles.devButton} onClick={handleGrantClassSet} disabled={busyAction !== null}>{t(busyAction?.startsWith("grant-") ? "Granting…" : "Grant class set")}</button>}
                    </div>
                </header>

                {error && <div className={styles.errorPanel} role="alert"><span>{error}</span><button type="button" className={styles.secondaryButton} onClick={() => void loadEquipment()}><UiText text="Retry" /></button></div>}
                {actionError?.key.startsWith("grant-") && <div className={styles.errorPanel} role="alert">{actionError.message}</div>}
                {loading && !equipment && <div className={styles.statePanel}><UiText text="Loading equipment…" /></div>}

                {equipment && <>
                    <div className={styles.topGrid}>
                        <section className={styles.heroPanel} aria-label="Hero and equipment slots">
                            <div className={styles.panelHeader}>
                                <div><span className={styles.eyebrow}><UiText text="YOUR HERO" /></span><h2>{selectedCharacter?.name || t("Hero")}</h2><p>{selectedCharacter ? `${t(formatEnum(selectedCharacter.heroClassId))} · ${t("Hero level")} ${selectedCharacter.level}` : t("Choose a hero")}</p></div>
                                <span className={styles.viewBadge}>{t(viewingActiveCharacter ? "Match hero" : "Viewing only")}</span>
                            </div>
                            <div className={styles.characterTabs} role="group" aria-label="Choose hero to view">
                                {equipment.characters.map(character => <button key={character.characterId} type="button" className={`${styles.characterTab} ${character.characterId === effectiveSelectedCharacterId ? styles.characterTabActive : ""}`} aria-pressed={character.characterId === effectiveSelectedCharacterId} disabled={busyAction !== null} onClick={() => { setSelectedCharacterId(character.characterId); setSelectedItemId(null); setSelectedSlot(null); setActionError(null); }}>
                                    <img src={getHeroPortrait(character.heroClassId)} alt="" />
                                    <span><strong>{character.name}</strong><small><UiText text="Lv." /> {character.level}</small></span>
                                </button>)}
                            </div>
                            <div className={styles.heroStage}>
                                <div className={styles.slotRail}>
                                    {EQUIPMENT_SLOTS.slice(0, 4).map(slot => <EquippedSlotCard key={slot} slot={slot} item={selectedEquipped[slot]} blocked={false} busy={busyAction !== null} selected={selectedSlot === slot && !selectedItemId} error={actionError?.key === `unequip-${slot}` ? actionError.message : undefined} onSelect={(nextSlot) => { setSelectedSlot(nextSlot); setSelectedItemId(null); }} onUnequip={handleUnequip} />)}
                                </div>
                                <div className={styles.heroFigure}>
                                    {selectedCharacter && <img src={getHeroFullbody(selectedCharacter.heroClassId)} alt={`${selectedCharacter.name}, full-length class artwork`} />}
                                    <div className={styles.figureCaption}><span><UiText text="GEAR" /></span><strong>{occupiedSlotsCount} / {EQUIPMENT_SLOTS.length}</strong></div>
                                </div>
                                <div className={styles.slotRail}>
                                    {EQUIPMENT_SLOTS.slice(4).map(slot => <EquippedSlotCard key={slot} slot={slot} item={selectedEquipped[slot]} blocked={slot === "off_hand" && offHandBlocked} blockedBy={slot === "off_hand" ? selectedEquipped.main_hand?.name : undefined} busy={busyAction !== null} selected={selectedSlot === slot && !selectedItemId} error={actionError?.key === `unequip-${slot}` ? actionError.message : undefined} onSelect={(nextSlot) => { setSelectedSlot(nextSlot); setSelectedItemId(null); }} onUnequip={handleUnequip} />)}
                                </div>
                            </div>
                        </section>

                        <aside className={styles.statsPanel} aria-label="Hero statistics and set bonuses">
                            <div className={styles.panelHeader}><div><span className={styles.eyebrow}><UiText text="CHARACTER SHEET" /></span><h2><UiText text="Statistics" /></h2><p><UiText text="Base → equipped, as confirmed by the server" /></p></div></div>
                            <StatsComparison baseStats={selectedBaseStats} effectiveStats={selectedEffectiveStats} />
                            <div className={styles.gearSummary}><span><UiText text="Occupied slots" /> <strong>{occupiedSlotsCount} / {EQUIPMENT_SLOTS.length}</strong></span><span><UiText text="Main hand" /> <strong>{selectedEquipped.main_hand?.name || t("Empty")}</strong></span></div>
                            <div className={styles.statsDivider} />
                            <div className={styles.panelHeaderCompact}><h2><UiText text="Set bonuses" /></h2><span>{selectedSetBonuses.length} <UiText text="active" /></span></div>
                            <ActiveSetBonuses bonuses={selectedSetBonuses} />
                        </aside>
                    </div>

                    <section className={styles.inventoryPanel} aria-label="Available equipment">
                        <div className={styles.panelHeader}><div><span className={styles.eyebrow}><UiText text="YOUR COLLECTION" /></span><h2><UiText text="Available Equipment" /></h2><p><UiText text="Unequipped items ready to use." /></p></div><span>{filteredItems.length} / {availableItems.length} <UiText text="items" /></span></div>
                        <div className={styles.inventoryLayout}>
                            <div className={styles.inventoryCollection}>
                                <div className={styles.filterBar}>
                                    <div className={styles.filterToggle} role="group" aria-label="Hero compatibility"><button type="button" aria-pressed={!filters.availableOnly} onClick={() => setFilters(current => ({ ...current, availableOnly: false }))}><UiText text="All" /></button><button type="button" aria-pressed={filters.availableOnly} onClick={() => setFilters(current => ({ ...current, availableOnly: true }))}><UiText text="For hero" /></button></div>
                                    <label><UiText text="Slot" /> <select value={filters.slot} onChange={event => setFilters(current => ({ ...current, slot: event.target.value as InventoryFilters["slot"] }))}><option value="all"><UiText text="All slots" /></option>{EQUIPMENT_SLOTS.filter(slot => availableSlots.includes(slot)).map(slot => <option key={slot} value={slot}>{t(SLOT_LABELS[slot])}</option>)}</select></label>
                                    <label><UiText text="Class" /> <select value={filters.heroClass} onChange={event => setFilters(current => ({ ...current, heroClass: event.target.value }))}><option value="all"><UiText text="All classes" /></option>{availableClasses.map(heroClass => <option key={heroClass} value={heroClass}>{t(formatEnum(heroClass))}</option>)}</select></label>
                                    <label><UiText text="Rarity" /> <select value={filters.rarity} onChange={event => setFilters(current => ({ ...current, rarity: event.target.value as InventoryFilters["rarity"] }))}><option value="all"><UiText text="All rarities" /></option>{availableRarities.map(rarity => <option key={rarity} value={rarity}>{t(formatEnum(rarity))}</option>)}</select></label>
                                </div>
                                {!equipment.availableEquipmentKnown ? <div className={styles.emptyState}><UiText text="Available equipment data is unavailable. Retry loading equipment." /></div> : availableItems.length === 0 ? <div className={styles.emptyState}><UiText text="No unequipped items available." /></div> : filteredItems.length === 0 ? <div className={styles.emptyState}><UiText text="No items match these filters." /></div> : <div className={styles.inventoryGrid}>
                                    {filteredItems.map(item => {
                                        return <InventoryCard key={item.instanceId} item={item} selected={selectedItemId === item.instanceId} reason={equipBlockReason(item, selectedCharacter, selectedEquipped)} onSelect={selected => { setSelectedItemId(selected.instanceId); setSelectedSlot(null); setActionError(null); }} />;
                                    })}
                                </div>}
                            </div>

                            <aside className={styles.detailPanel} aria-label="Item details and comparison">
                                <div className={styles.detailHeading}><span className={styles.eyebrow}><UiText text="INSPECT" /></span><h3>{t(selectedInventoryItem ? "Compare equipment" : selectedSlot ? SLOT_LABELS[selectedSlot] : "Select an item")}</h3></div>
                                {selectedInventoryItem ? <>
                                    <div className={styles.compareCards}>
                                        <div className={styles.compareCard}><span><UiText text="NOW EQUIPPED" /></span>{comparedItem ? <><ItemImage item={comparedItem} /><strong>{comparedItem.name}</strong><small>{formatEnum(comparedItem.rarity)} · {SLOT_LABELS[comparedItem.slot]}</small>{(comparedItem.setName || comparedItem.setCode) && <small><UiText text="Set:" /> {comparedItem.setName || formatEnum(comparedItem.setCode || "")}</small>}</> : <p><UiText text="Empty" /> {SLOT_LABELS[selectedInventoryItem.slot]} <UiText text="slot" /></p>}</div>
                                        <div className={styles.compareCard}><span><UiText text="SELECTED ITEM" /></span><ItemImage item={selectedInventoryItem} /><strong><UiText text={selectedInventoryItem.name} /></strong><small>{formatEnum(selectedInventoryItem.rarity)} · {SLOT_LABELS[selectedInventoryItem.slot]}</small>{(selectedInventoryItem.setName || selectedInventoryItem.setCode) && <small><UiText text="Set:" /> <UiText text={selectedInventoryItem.setName || formatEnum(selectedInventoryItem.setCode || "")} /></small>}</div>
                                    </div>
                                    <ItemBonusComparison rows={itemComparison} />
                                    <div className={styles.itemRestrictions}><ItemMeta item={selectedInventoryItem} /></div>
                                    {selectedBlockReason && <p className={styles.restrictionMessage}>{t(selectedBlockReason)}</p>}
                                    <div className={styles.detailAction}>
                                        {actionError?.key === `equip-${selectedInventoryItem.instanceId}` && <p className={styles.inlineError} role="alert">{actionError.message}</p>}
                                        {actionError?.key === `unequip-${selectedInventoryItem.slot}` && <p className={styles.inlineError} role="alert">{actionError.message}</p>}
                                        {selectedEquipped[selectedInventoryItem.slot]?.instanceId === selectedInventoryItem.instanceId ? <button type="button" className={styles.secondaryButton} disabled={busyAction !== null} onClick={() => handleUnequip(selectedInventoryItem.slot)}>{t(busyAction ? "Working…" : "Unequip")}</button> : <button type="button" className={styles.primaryButton} disabled={busyAction !== null || !!selectedBlockReason} onClick={() => handleEquip(selectedInventoryItem)}>{t(busyAction?.startsWith("equip-") ? "Equipping…" : "Equip item")}</button>}
                                    </div>
                                </> : selectedSlot ? inspectedItem ? <div className={styles.inspectedSlot}><ItemImage item={inspectedItem} /><h4><UiText text={inspectedItem.name} /></h4><ItemMeta item={inspectedItem} /><BonusList bonuses={inspectedItem.bonuses} />{(inspectedItem.setName || inspectedItem.setCode) && <p><UiText text="Set:" /> <UiText text={inspectedItem.setName || formatEnum(inspectedItem.setCode || "")} /></p>}{actionError?.key === `unequip-${selectedSlot}` && <p className={styles.inlineError} role="alert">{actionError.message}</p>}<button type="button" className={styles.secondaryButton} disabled={busyAction !== null} onClick={() => handleUnequip(selectedSlot)}>{t(busyAction ? "Working…" : "Unequip")}</button></div> : <p className={styles.detailPrompt}>{selectedSlot === "off_hand" && offHandBlocked ? `${t("Blocked by")} ${selectedEquipped.main_hand?.name}.` : t("This slot is empty. Select a compatible item from Available Equipment.")}</p> : <p className={styles.detailPrompt}><UiText text="Choose an available item or an equipment slot to see its details here." /></p>}
                            </aside>
                        </div>
                    </section>
                </>}
            </div>
        </main>
    );
}
