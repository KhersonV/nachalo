export type ReflexHeroClass = "guardian" | "berserker" | "ranger" | "mystic";

export const BASE_REFLEX_BY_CLASS: Record<ReflexHeroClass, number> = {
    guardian: 5,
    berserker: 5,
    ranger: 5,
    mystic: 5,
};

export type ReflexEffectInfo = {
    name: string;
    description: string;
};

export const CLASS_REFLEX_EFFECTS: Record<ReflexHeroClass, ReflexEffectInfo> = {
    guardian: {
        name: "Shield Block",
        description: "Block incoming direct attacks.",
    },
    berserker: {
        name: "Blood Feast",
        description: "Heal 50% of dealt damage on proc.",
    },
    ranger: {
        name: "Critical Shot",
        description: "Critical Shot deals +42% damage.",
    },
    mystic: {
        name: "Arcane Overburn",
        description: "Arcane Overburn burns 4 energy and restores 3 energy.",
    },
};

export function getReflexProcChance(reflex: number): number {
    if (reflex <= 0) return 0;

    const chance = Math.floor((30 * reflex) / (reflex + 10));

    return Math.min(25, chance);
}

export function getBaseReflexForHero(heroClassId?: string): number {
    const normalized = normalizeReflexHeroClass(heroClassId);
    return normalized ? BASE_REFLEX_BY_CLASS[normalized] : 5;
}

export function getReflexEffectDescription(heroClassId?: string): string {
    return getClassReflexEffectDescription(heroClassId);
}

export function getClassReflexEffectName(heroClassId?: string): string {
    const normalized = normalizeReflexHeroClass(heroClassId);
    return normalized ? CLASS_REFLEX_EFFECTS[normalized].name : "Class effect";
}

export function getClassReflexEffectDescription(heroClassId?: string): string {
    const normalized = normalizeReflexHeroClass(heroClassId);
    return normalized
        ? CLASS_REFLEX_EFFECTS[normalized].description
        : "Trigger a class-specific combat reaction.";
}

function normalizeReflexHeroClass(heroClassId?: string): ReflexHeroClass | null {
    const normalized = heroClassId?.trim().toLowerCase();
    if (
        normalized === "guardian" ||
        normalized === "berserker" ||
        normalized === "ranger" ||
        normalized === "mystic"
    ) {
        return normalized;
    }
    return null;
}
