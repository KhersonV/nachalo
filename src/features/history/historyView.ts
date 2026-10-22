import type { MatchHistoryListItem, MatchPlayerResultSummary } from "../../types/MatchHistoryTypes";

export type MatchFilter = { mode: "all" | "pve" | "pvp"; heroClass: string; result: "all" | "winner" | "survived" | "eliminated" };
export const DEFAULT_MATCH_FILTER: MatchFilter = { mode: "all", heroClass: "all", result: "all" };

const PVP_MODES = new Set(["1x1", "1x2", "2x2", "3x3", "5x5"]);

export function modeFamily(mode: string): "pve" | "pvp" | null {
    const value = mode.trim().toLowerCase();
    if (value === "pve") return "pve";
    return PVP_MODES.has(value) ? "pvp" : null;
}

export function personalResult(result: MatchPlayerResultSummary | null | undefined): { label: string; tone: "success" | "danger" | "neutral"; key: MatchFilter["result"] } {
    if (result?.status === "winner") return { label: "Victory", tone: "success", key: "winner" };
    if (result?.status === "survived") return { label: "Survived", tone: "neutral", key: "survived" };
    if (result?.status === "eliminated") return { label: "Eliminated", tone: "danger", key: "eliminated" };
    return { label: "Result unavailable", tone: "neutral", key: "all" };
}

export function filterMatches(matches: MatchHistoryListItem[], filter: MatchFilter): MatchHistoryListItem[] {
    return matches.filter(match =>
        (filter.mode === "all" || modeFamily(match.mode) === filter.mode) &&
        (filter.heroClass === "all" || match.characterType?.toLowerCase() === filter.heroClass) &&
        (filter.result === "all" || personalResult(match.playerResultSummary).key === filter.result),
    );
}

export type ClassProgress = { characterType: string; matchesPlayed: number; wins: number; losses: number; winRate: number; playerKills: number; monsterKills: number; deaths: number; damageTotal: number; damageTaken: number; totalExpGained: number };
export type OverallProgress = { matchesPlayed: number; wins: number; losses: number; winRate: number; playerKills: number; monsterKills: number; deaths: number; damageTotal: number; damageTaken: number; totalExpGained: number; classes: ClassProgress[] };

export function representedClasses(progress: OverallProgress | null | undefined): ClassProgress[] {
    return (progress?.classes ?? []).filter(item => Number.isFinite(item.matchesPlayed) && item.matchesPlayed > 0);
}

export function hasOverallProgress(progress: OverallProgress | null | undefined): progress is OverallProgress {
    return !!progress && [progress.matchesPlayed, progress.wins, progress.losses, progress.winRate].every(Number.isFinite);
}
