// The Mystic starter set kept its template codes when its artwork moved.
// Resolve old URLs as well, since existing item instances can retain them.
const relocatedMysticItems = [
    ["sagecloth_staff", "staff", "Elderbloom Staff.png"],
    ["sagecloth_jacket", "jacket", "Elderbloom Vest.png"],
    ["sagecloth_pants", "pants", "Elderbloom Pants.png"],
    ["sagecloth_boots", "boots", "Elderbloom Boots.png"],
    ["sagecloth_gloves", "gloves", "Elderbloom Gloves.png"],
    ["sagecloth_hood", "hood", "Elderbloom Hood.png"],
] as const;

const byCode = new Map<string, string>();
const byOldPath = new Map<string, string>();

for (const [code, oldFile, newFile] of relocatedMysticItems) {
    const current = `/equipment/mystic/Elderbloom/${newFile}`;
    byCode.set(code, current);
    byOldPath.set(`/equipment/mystic/sagecloth/${oldFile}.png`, current);
}

export function equipmentImage(imageUrl: string | null | undefined, code?: string | null): string {
    const url = imageUrl?.trim() ?? "";
    const relocated = url.replace("/guardian/aegiswarden/", "/guardian/Aegiswarden/")
        .replace("/berserker/bloodroot/", "/berserker/Bloodroot/")
        .replace("/ranger/greenwisp/", "/ranger/Greenwisp/")
        .replace("/guardian/stonewarden_final_png/", "/guardian/Stonewarden/")
        .replace("/berserker/Emberfang_Ravager_Uncommon_Set/", "/berserker/Emberfang/")
        .replace("/mystic/mooncrystal_set/", "/mystic/Mooncrystal/");
    return byOldPath.get(url) ?? (relocated || (code ? byCode.get(code) : undefined) || "");
}

// Display names only; item rarity itself always comes from the server.
export function equipmentRarityName(rarity: string): string {
    const labels: Record<string, string> = { gray: "Common", green: "Uncommon", blue: "Rare", purple: "Epic", orange: "Legendary" };
    return labels[rarity] || rarity;
}
