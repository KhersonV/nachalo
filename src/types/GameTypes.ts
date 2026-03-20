// ==============================
// src/types/GameTypes.ts
// ==============================

export type BonusAttributes = {
    energy?: number;
    maxEnergy?: number;
    visionRange?: number;
    health?: number;
    maxHealth?: number;
    attack?: number;
    defense?: number;
};

// Определяем, как приходит каждый элемент из бэка
export type RawInventoryItem = {
    item_type: "resource" | "artifact";
    item_id: number;
    name: string; // раньше было item_name
    item_count: number;
    image: string; // соответствует image_url на бэке
    description: string; // соответствует item_description на бэке
    bonus?: BonusAttributes;
    effect?: Record<string, number>;
    inventory_key?: string;
};

// Типизированный инвентарь уже с разделением
export type Inventory = {
    resources: Record<string, RawInventoryItem>;
    artifacts: Record<string, RawInventoryItem>;
};

export type PlayerState = {
    user_id: number;
    name: string;
    group_id?: number;
    health: number;
    maxHealth: number;
    attack: number;
    defense: number;
    vision: number;
    image: string;
    lastTurnAttacked?: number;
    position: { x: number; y: number };
    energy: number;
    maxEnergy: number;
    level: number;
    experience: number;
    maxExperience: number;
    sightRange: number;
    inventory: Inventory;
    mobility: number;
    agility: number;
    isRanged?: boolean;
    attackRange?: number;
};

export type MonsterType = {
    id: number;
    db_instance_id: number;
    name: string;
    type: string;
    health: number;
    maxHealth: number;
    attack: number;
    defense: number;
    speed: number;
    maneuverability: number;
    vision: number;
    image: string;
    created_at: string;
};

export type ResourceType = {
    id: number;
    type: string;
    description: string;
    effect: Record<string, number>;
    image: string;
};

export interface Cell {
    cell_id: number;
    x: number;
    y: number;
    tileCode: number;
    resource: ResourceType | null;
    barbel: ResourceType | null;
    monster: MonsterType | null;
    isPortal: boolean;
    isPlayer: boolean;
    structure_type?: string;
    structure_owner_user_id?: number;
    structure_health?: number;
    structure_defense?: number;
    structure_attack?: number;
    is_under_construction?: boolean;
    construction_turns_left?: number;
}

export type GameState = {
    instanceId: string;
    mode: string;
    grid: Cell[];
    mapWidth: number;
    mapHeight: number;
    players: PlayerState[];
    active_user: number;
    turnNumber: number;
    isMapLoaded: boolean;
    questArtifactId: number;
    questArtifactName: string;
    questArtifactImage: string;
    questArtifactDescription: string;
    questFoundNotification: string | null;
};

// Здесь перечисляем возможные направления
export type Dir = "up" | "down" | "left" | "right";
