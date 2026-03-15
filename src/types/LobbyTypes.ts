export type ShopItem = {
    id: number;
    type: "food" | "water";
    name: string;
    description: string;
    image: string;
    effect: Record<string, number>;
    price: number;
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
};
