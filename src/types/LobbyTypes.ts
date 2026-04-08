export type ShopItem = {
    id: number;
    type: string;
    category: "resource" | "blueprint" | "scroll";
    name: string;
    description: string;
    image: string;
    effect: Record<string, number>;
    price: number;
    inventoryKey: string;
    requiresForge: boolean;
};

export type PlayerShopState = {
    user_id: number;
    balance: number;
    inventory: string;
};

export type ForgeRecipe = {
    id: string;
    name: string;
    description: string;
};

export type BuildingState = {
    level: number;
    built: boolean;
    costs: {
        wood: number;
        stone: number;
        iron: number;
    };
    resources: {
        wood: number;
        stone: number;
        iron: number;
    };
    canBuild: boolean;
    unlockables: ForgeRecipe[];
};

export type BaseState = {
    forgeLevel: number;
    built: boolean;
    costs: {
        wood: number;
        stone: number;
        iron: number;
    };
    resources: {
        wood: number;
        stone: number;
        iron: number;
    };
    canBuild: boolean;
    recipes: ForgeRecipe[];
    forge: BuildingState;
    library: BuildingState;
};
