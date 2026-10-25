import type { EquipmentBonuses, EquipmentCharacter, EquipmentItem, EquipmentSlot, ItemRarity } from "../../types/equipment";

export type InventoryFilters = {
    slot: EquipmentSlot | "all";
    heroClass: string;
    rarity: ItemRarity | "all";
    availableOnly: boolean;
};

export const BONUS_FIELDS: ReadonlyArray<{ key: keyof EquipmentBonuses; label: string }> = [
    { key: "attack", label: "Attack" },
    { key: "defense", label: "Defense" },
    { key: "mobility", label: "Mobility" },
    { key: "agility", label: "Reflex" },
    { key: "maxHealth", label: "Max Health" },
    { key: "sightRange", label: "Sight Range" },
    { key: "attackRange", label: "Attack Range" },
    { key: "pocketBonus", label: "Additional pockets" },
];

export function isTwoHanded(item?: EquipmentItem): boolean {
    return item?.handedness === "two_hand" || item?.isTwoHanded === true;
}

export function getAvailableEquipment(
    serverInventory: EquipmentItem[] | undefined,
    equippedByCharacter: Record<string, Partial<Record<EquipmentSlot, EquipmentItem>>>,
    activeEquipped: Partial<Record<EquipmentSlot, EquipmentItem>>,
): EquipmentItem[] {
    if (!serverInventory) return [];
    const equippedIds = new Set<string>();
    for (const slots of [...Object.values(equippedByCharacter), activeEquipped]) {
        for (const item of Object.values(slots)) {
            if (item?.instanceId) equippedIds.add(item.instanceId);
        }
    }
    const seen = new Set<string>();
    return serverInventory.filter(item => {
        if (!item.instanceId || seen.has(item.instanceId)) return false;
        seen.add(item.instanceId);
        return item.status === "inventory" && !item.equippedCharacterId && !equippedIds.has(item.instanceId);
    });
}

export function getGearCount(equipped: Partial<Record<EquipmentSlot, EquipmentItem>>) {
    const instanceIds = new Set<string>();
    let occupiedSlotsCount = 0;
    for (const item of Object.values(equipped)) {
        if (!item?.instanceId) continue;
        instanceIds.add(item.instanceId);
        occupiedSlotsCount += 1;
    }
    if (equipped.main_hand?.instanceId && isTwoHanded(equipped.main_hand) && !equipped.off_hand) {
        occupiedSlotsCount += 1;
    }
    return { equippedItemsCount: instanceIds.size, occupiedSlotsCount };
}

export function equipBlockReason(
    item: EquipmentItem,
    hero: EquipmentCharacter | null,
    equipped: Partial<Record<EquipmentSlot, EquipmentItem>>,
): string | null {
    if (!hero) return "Choose a hero";
    if (item.equippedCharacterId || item.status === "equipped") return "Already equipped";
    if (item.status !== "inventory") return "Item availability unconfirmed";
    if (item.classRestriction && item.classRestriction.toLowerCase() !== hero.heroClassId.toLowerCase()) return "Different hero class";
    if (item.levelRequirement && hero.level < item.levelRequirement) return `Requires hero level ${item.levelRequirement}`;
    if (item.slot === "off_hand" && isTwoHanded(equipped.main_hand)) return "Blocked by two-handed weapon";
    return null;
}

export function filterEquipmentItems(
    items: EquipmentItem[],
    filters: InventoryFilters,
    hero: EquipmentCharacter | null,
    equipped: Partial<Record<EquipmentSlot, EquipmentItem>>,
): EquipmentItem[] {
    return items.filter(item => {
        if (filters.slot !== "all" && item.slot !== filters.slot) return false;
        if (filters.rarity !== "all" && item.rarity !== filters.rarity) return false;
        if (filters.heroClass !== "all" && item.classRestriction && item.classRestriction.toLowerCase() !== filters.heroClass.toLowerCase()) return false;
        if (filters.availableOnly && equipBlockReason(item, hero, equipped)) return false;
        return true;
    });
}

export function compareItemBonuses(current?: EquipmentItem, selected?: EquipmentItem) {
    return BONUS_FIELDS.map(field => {
        const currentValue = current?.bonuses[field.key] ?? 0;
        const selectedValue = selected?.bonuses[field.key] ?? 0;
        return { ...field, currentValue, selectedValue, difference: selectedValue - currentValue };
    }).filter(row => row.currentValue !== 0 || row.selectedValue !== 0);
}
