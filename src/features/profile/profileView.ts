export type Relation = "self" | "friend" | "outgoing" | "incoming" | "none";
export type SocialTab = "friends" | "incoming" | "outgoing" | "search";
export type RelationAction = "send" | "remove" | "accept" | "decline" | "cancel";

export function profileViewId(raw: string | null): number | null {
    const id = Number.parseInt(raw ?? "", 10);
    return Number.isSafeInteger(id) && id > 0 ? id : null;
}

export function matchesProfileView(viewId: number | null, loadedId: number | undefined): boolean {
    return viewId !== null && viewId === loadedId;
}

export function relationActions(relation: Relation, targetId: number, ownId: number): RelationAction[] {
    if (targetId === ownId || relation === "self") return [];
    if (relation === "friend") return ["remove"];
    if (relation === "outgoing") return ["cancel"];
    if (relation === "incoming") return ["accept", "decline"];
    return relation === "none" ? ["send"] : [];
}

export function ownedHeroes<T extends { owned: boolean; enabled: boolean; sortOrder: number }>(heroes: T[]): T[] {
    return heroes.filter(hero => hero.owned && hero.enabled).sort((a, b) => a.sortOrder - b.sortOrder);
}

export function xpProgress(experience: number | undefined, maxExperience: number | undefined): number | null {
    if (!Number.isFinite(experience) || !Number.isFinite(maxExperience) || !maxExperience || maxExperience <= 0) return null;
    return Math.max(0, Math.min(100, Math.round((experience! / maxExperience) * 100)));
}

export async function performConfirmedSocialAction(request: () => Promise<void>, refresh: () => Promise<void>): Promise<void> {
    await request();
    await refresh();
}
