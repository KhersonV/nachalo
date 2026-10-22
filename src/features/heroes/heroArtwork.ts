export type HeroClass = "guardian" | "ranger" | "berserker" | "mystic";

export type HeroArtwork = {
    portrait: string;
    fullbody: string;
    portraitPng: string;
    fullbodyPng: string;
    gameImage: string;
    legacyUiImage: string;
};

export const HERO_ARTWORK_FALLBACK = "/assets/ui/heroes/hero-fallback.svg";

const ROOT = "/assets/ui/heroes";
const artwork: Record<HeroClass, HeroArtwork> = {
    guardian: {
        portrait: `${ROOT}/optimized/portraits/guardian.webp`,
        fullbody: `${ROOT}/optimized/fullbody/guardian.webp`,
        portraitPng: `${ROOT}/portraits/guardian.png`,
        fullbodyPng: `${ROOT}/fullbody/guardian.png`,
        gameImage: "/guardian/guardian.webp",
        legacyUiImage: `${ROOT}/guardian.webp`,
    },
    ranger: {
        portrait: `${ROOT}/optimized/portraits/ranger.webp`,
        fullbody: `${ROOT}/optimized/fullbody/ranger.webp`,
        portraitPng: `${ROOT}/portraits/ranger.png`,
        fullbodyPng: `${ROOT}/fullbody/ranger.png`,
        gameImage: "/ranger/ranger.webp",
        legacyUiImage: `${ROOT}/ranger.webp`,
    },
    berserker: {
        portrait: `${ROOT}/optimized/portraits/berserker.webp`,
        fullbody: `${ROOT}/optimized/fullbody/berserker.webp`,
        portraitPng: `${ROOT}/portraits/berserker.png`,
        fullbodyPng: `${ROOT}/fullbody/berserker.png`,
        gameImage: "/berserk/berserk.webp",
        legacyUiImage: `${ROOT}/berserker.webp`,
    },
    mystic: {
        portrait: `${ROOT}/optimized/portraits/mystic.webp`,
        fullbody: `${ROOT}/optimized/fullbody/mystic.webp`,
        portraitPng: `${ROOT}/portraits/mystic.png`,
        fullbodyPng: `${ROOT}/fullbody/mystic.png`,
        gameImage: "/mag/mag.webp",
        legacyUiImage: `${ROOT}/mystic.webp`,
    },
};

export function normalizeHeroClass(value: string | number | null | undefined): HeroClass | null {
    if (typeof value !== "string") return null;
    const key = value.trim().toLowerCase();
    if (key === "berserk") return "berserker";
    if (key === "mag") return "mystic";
    return Object.prototype.hasOwnProperty.call(artwork, key) ? key as HeroClass : null;
}

export function getHeroArtwork(heroClass: string | number | null | undefined): HeroArtwork | null {
    const normalized = normalizeHeroClass(heroClass);
    return normalized ? artwork[normalized] : null;
}

// The API's default game image is class metadata. A different image may be an
// individually selected skin, so keep it when one is supplied.
export function getHeroPortrait(heroClass: string | number | null | undefined, heroImage?: string | null): string {
    const entry = getHeroArtwork(heroClass);
    const supplied = typeof heroImage === "string" ? heroImage.trim() : "";
    if (supplied && (!entry || (supplied !== entry.gameImage && supplied !== entry.legacyUiImage))) return supplied;
    return entry?.portrait ?? HERO_ARTWORK_FALLBACK;
}

export function getHeroFullbody(heroClass: string | number | null | undefined): string {
    return getHeroArtwork(heroClass)?.fullbody ?? HERO_ARTWORK_FALLBACK;
}

export function getHeroGameImage(heroClass: string | number | null | undefined): string {
    return getHeroArtwork(heroClass)?.gameImage ?? HERO_ARTWORK_FALLBACK;
}
