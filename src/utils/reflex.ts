export type ReflexHeroClass = "guardian" | "berserker" | "ranger" | "mystic";

export const BASE_REFLEX_BY_CLASS: Record<ReflexHeroClass, number> = {
    guardian: 5,
    berserker: 5,
    ranger: 5,
    mystic: 5,
};

export const REFLEX_EFFECT_DESCRIPTIONS: Record<ReflexHeroClass, string> = {
    guardian: "Reflex gives a chance to block direct attacks.",
    berserker: "Reflex gives a chance to heal from dealt damage.",
    ranger: "Reflex gives a chance to land a critical shot.",
    mystic: "Reflex gives a chance to overburn enemy energy.",
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
    const normalized = normalizeReflexHeroClass(heroClassId);
    return normalized
        ? REFLEX_EFFECT_DESCRIPTIONS[normalized]
        : "Reflex gives a chance to trigger a class-specific combat reaction.";
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
