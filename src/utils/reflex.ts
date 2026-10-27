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
        description: "Base block chance from Reflex. Class counters apply in PvP.",
    },
    berserker: {
        name: "Blood Feast",
        description: "Heal 50% of actual primary damage on proc; equipment increases the coefficient.",
    },
    ranger: {
        name: "Critical Shot",
        description: "Base critical damage is 142%; equipped bow and boots increase it.",
    },
    mystic: {
        name: "Arcane Burst",
        description: "Deal +3 direct damage on Reflex, at most once per personal turn; gloves increase it.",
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

// Presentation only; the server rolls the effect, independently of ordinary reactions.
export function getClassReflexChance(reflex: number, owner?: string, opponent?: string, pvp = false): number {
    const base = getReflexProcChance(reflex);
    if (!pvp) return base;
    const counters: Record<string,string> = { guardian: "berserker", berserker: "ranger", ranger: "mystic", mystic: "guardian" };
    const own = owner?.toLowerCase() ?? "", other = opponent?.toLowerCase() ?? "";
    if (counters[other] === own) return 0;
    return Math.min(35, base + (counters[own] === other ? 10 : 0));
}
