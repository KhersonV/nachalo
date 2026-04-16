import type {
    MatchHistoryParticipant,
    MatchHistoryReward,
    MatchHistoryWinner,
    MatchPlayerResultSummary,
} from "@/types";

function normalizeMode(mode: string): string {
    const normalized = mode.trim();
    if (!normalized) return "Unknown";
    if (normalized.toUpperCase() === "PVE") return "PVE";
    return normalized.toUpperCase();
}

export function formatMatchMode(mode: string): string {
    return normalizeMode(mode);
}

export function formatMatchFinishedAt(value: string): string {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
        return value || "Unknown finish time";
    }

    return date.toLocaleString(undefined, {
        year: "numeric",
        month: "short",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
    });
}

export function rewardLabel(type: string): string {
    const normalized = type.trim().toLowerCase();
    switch (normalized) {
        case "balance":
        case "coin":
        case "coins":
        case "gold":
            return "Currency";
        default:
            return type || "Reward";
    }
}

function groupParticipantsByTeam(
    participants: MatchHistoryParticipant[],
): Map<number, MatchHistoryParticipant[]> {
    const groups = new Map<number, MatchHistoryParticipant[]>();
    for (const participant of participants) {
        const groupKey = participant.groupId > 0 ? participant.groupId : participant.userId;
        const current = groups.get(groupKey) ?? [];
        current.push(participant);
        groups.set(groupKey, current);
    }
    return groups;
}

export function formatParticipantsSummary(
    participants: MatchHistoryParticipant[],
): string {
    if (participants.length === 0) {
        return "Participants unavailable";
    }

    const groups = groupParticipantsByTeam(participants);
    const isTeamMatch = Array.from(groups.values()).some((team) => team.length > 1);
    if (!isTeamMatch) {
        return participants
            .map((participant) => participant.name)
            .filter(Boolean)
            .join(", ");
    }

    return Array.from(groups.entries())
        .sort(([left], [right]) => left - right)
        .map(([groupId, team]) => {
            const names = team
                .map((participant) => participant.name)
                .filter(Boolean)
                .join(", ");
            return `Team ${groupId}: ${names}`;
        })
        .join(" | ");
}

export function formatWinnerSummary(
    winner: MatchHistoryWinner,
    participants: MatchHistoryParticipant[],
): string {
    if (!winner.id) {
        return "Winner not determined";
    }

    if (winner.type === "group") {
        const teamMembers = participants.filter(
            (participant) => participant.groupId === winner.id,
        );
        if (teamMembers.length === 0) {
            return `Winning team: ${winner.id}`;
        }
        return `Winning team: ${winner.id} (${teamMembers
            .map((participant) => participant.name)
            .join(", ")})`;
    }

    const winnerById = participants.find(
        (participant) => participant.userId === winner.id,
    );
    if (winnerById) {
        return `Winner: ${winnerById.name}`;
    }

    if (winner.userIds?.length) {
        const names = winner.userIds
            .map((userId) =>
                participants.find((participant) => participant.userId === userId)?.name,
            )
            .filter((name): name is string => Boolean(name));
        if (names.length > 0) {
            return `Winner: ${names.join(", ")}`;
        }
    }

    return `Winner: player ${winner.id}`;
}

export function formatResultStatus(
    result: MatchPlayerResultSummary,
): string {
    if (result.isWinner) return "Winner";
    if (result.survived) return "Survived";
    return "Eliminated";
}

export function formatRewardSummary(rewards: MatchHistoryReward[]): string {
    if (rewards.length === 0) {
        return "No rewards";
    }

    return rewards
        .map((reward) => `${rewardLabel(reward.type)}: ${reward.amount}`)
        .join(", ");
}
