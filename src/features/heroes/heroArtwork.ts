export type HeroClass = "guardian" | "ranger" | "berserker" | "mystic";

export type HeroArtwork = {
    portrait: string;
    fullbody: string;
    gameImage: string;
    legacyUiImage: string;
};

export const HERO_ARTWORK_FALLBACK = "/assets/ui/heroes/hero-fallback.svg";

const ROOT = "/assets/ui/heroes";
const artwork: Record<HeroClass, HeroArtwork> = {
    guardian: {
        portrait: `${ROOT}/portraits/guardian.webp`,
        fullbody: `${ROOT}/fullbody/guardian.webp`,
        gameImage: "/guardian/guardian.webp",
        legacyUiImage: `${ROOT}/guardian.webp`,
    },
    ranger: {
        portrait: `${ROOT}/portraits/ranger.webp`,
        fullbody: `${ROOT}/fullbody/ranger.webp`,
        gameImage: "/ranger/ranger.webp",
        legacyUiImage: `${ROOT}/ranger.webp`,
    },
    berserker: {
        portrait: `${ROOT}/portraits/berserker.webp`,
        fullbody: `${ROOT}/fullbody/berserker.webp`,
        gameImage: "/berserk/berserk.webp",
        legacyUiImage: `${ROOT}/berserker.webp`,
    },
    mystic: {
        portrait: `${ROOT}/portraits/mystic.webp`,
        fullbody: `${ROOT}/fullbody/mystic.webp`,
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
