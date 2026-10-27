export type EquipmentSlot =
    | "main_hand"
    | "off_hand"
    | "helmet"
    | "chest"
    | "pants"
    | "boots"
    | "gloves"
    | "ring"
    | "amulet";

export type ItemRarity = "gray" | "green" | "blue" | "purple" | "orange";

export type ItemHandedness = "none" | "one_hand" | "two_hand";

export type EquipmentBonuses = {
    pocketBonus?: number;
    attack?: number;
    defense?: number;
    mobility?: number;
    agility?: number;
    maxHealth?: number;
    maxEnergy?: number;
    sightRange?: number;
    attackRange?: number;
};

export type EquipmentItem = {
    description?: string;
    combatBonuses?: Record<string, number>;
    jewelrySet?: { name: string; bonus: EquipmentBonuses; required: number };
    instanceId: string;
    templateId: number;
    code: string;
    name: string;
    setCode?: string;
    setName?: string;
    slot: EquipmentSlot;
    itemType: string;
    handedness: ItemHandedness;
    rarity: ItemRarity;
    classRestriction?: string | null;
    levelRequirement?: number;
    imageUrl: string;
    bonuses: EquipmentBonuses;
    sellPrice?: number;
    canSell?: boolean;
    sellBlockReason?: string;
    isTwoHanded?: boolean;
    status?: string;
    version?: number;
    equippedCharacterId?: number;
};

export type EquipmentStats = {
    maxAP?: number;
    maxMP?: number;
    maxReactions?: number;
    maxHealth?: number;
    maxEnergy?: number;
    attack?: number;
    defense?: number;
    mobility?: number;
    agility?: number;
    sightRange?: number;
    attackRange?: number;
};

export type ActiveSetBonus = {
    setCode?: string;
    setName?: string;
    pieces?: number;
    piecesRequired?: number;
    description: string;
    bonuses?: EquipmentBonuses;
};

export type EquipmentCharacter = {
    characterId: number;
    heroClassId: string;
    name: string;
    level: number;
};

export type CharacterEquipmentStats = {
    baseStats: EquipmentStats;
    bonusStats: EquipmentBonuses;
    effectiveStats: EquipmentStats;
};

export type EquipmentState = {
    activeCharacterId: number;
    activeHeroClassId?: string;
    characters: EquipmentCharacter[];
    inventory: EquipmentItem[];
    inventoryItems?: EquipmentItem[];
    availableEquipmentKnown?: boolean;
    ownedItems: EquipmentItem[];
    equipped: Partial<Record<EquipmentSlot, EquipmentItem>>;
    equippedByCharacter: Record<string, Partial<Record<EquipmentSlot, EquipmentItem>>>;
    baseStats: EquipmentStats;
    effectiveStats: EquipmentStats;
    activeSetBonuses: ActiveSetBonus[];
    statsByCharacter: Record<string, CharacterEquipmentStats>;
    activeSetBonusesByCharacter: Record<string, ActiveSetBonus[]>;
};

export type EquipmentApiResponse = {
    status?: string;
    data?: Partial<EquipmentState>;
    error?: string;
};
