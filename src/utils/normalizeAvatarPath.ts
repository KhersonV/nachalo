const AVATAR_ALIASES: Record<string, string> = {
    "/Character_1.webp": "/guardian/guardian.webp",
    "Character_1.webp": "/guardian/guardian.webp",
    "/Character_2.webp": "/berserk/berserk.webp",
    "Character_2.webp": "/berserk/berserk.webp",
    "/player-1.webp": "/ranger/ranger.webp",
    "player-1.webp": "/ranger/ranger.webp",
    "/player-2.webp": "/mag/mag.webp",
    "player-2.webp": "/mag/mag.webp",
    "/guardian.webp": "/guardian/guardian.webp",
    "guardian.webp": "/guardian/guardian.webp",
    "/ranger.webp": "/ranger/ranger.webp",
    "ranger.webp": "/ranger/ranger.webp",
    "/berserk-character.webp": "/berserk/berserk.webp",
    "berserk-character.webp": "/berserk/berserk.webp",
    "/berserk.webp": "/berserk/berserk.webp",
    "berserk.webp": "/berserk/berserk.webp",
    "/mag.webp": "/mag/mag.webp",
    "mag.webp": "/mag/mag.webp",
};

export function normalizeAvatarPath(image?: string | null): string {
    const raw = (image ?? "").trim();
    if (!raw) return "/ranger/ranger.webp";
    return AVATAR_ALIASES[raw] ?? raw;
}
