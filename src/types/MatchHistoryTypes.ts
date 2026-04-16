export type MatchHistoryReward = {
    type: string;
    amount: number;
};

export type MatchHistoryParticipant = {
    userId: number;
    name: string;
    groupId: number;
    characterType?: string;
    placement?: number;
    isWinner: boolean;
    survived: boolean;
};

export type MatchHistoryWinner = {
    type: "user" | "group";
    id: number;
    userIds?: number[];
};

export type MatchPlayerRewardSummary = {
    expGained: number;
    currencyGained: number;
    rewards: MatchHistoryReward[];
};

export type MatchPlayerResultSummary = {
    placement: number;
    isWinner: boolean;
    survived: boolean;
    deaths: number;
    kills: number;
    status: "winner" | "survived" | "eliminated" | string;
};

export type MatchHistoryListItem = {
    matchId: string;
    finishedAt: string;
    mode: string;
    participants: MatchHistoryParticipant[];
    winner: MatchHistoryWinner;
    playerRewardSummary: MatchPlayerRewardSummary;
    playerResultSummary: MatchPlayerResultSummary;
    status: string;
};

export type MatchHistoryArtifact = {
    itemId: number;
    name: string;
    count: number;
    image?: string;
    description?: string;
};

export type MatchHistoryBonus = {
    type: string;
    label: string;
    amount?: number;
    expBonus?: number;
};

export type CurrentPlayerMatchStats = {
    rewards: MatchHistoryReward[];
    expGained: number;
    currencyGained: number;
    damageDealt: number;
    damageTaken: number;
    damageToPlayers: number;
    damageToMonsters: number;
    kills: number;
    playerKills: number;
    monsterKills: number;
    deaths: number;
    survived: boolean;
    placement: number;
    artifacts: MatchHistoryArtifact[];
    otherBonuses: MatchHistoryBonus[];
};

export type MatchHistoryDetailItem = {
    matchId: string;
    finishedAt: string;
    mode: string;
    participants: MatchHistoryParticipant[];
    winner: MatchHistoryWinner;
    playerRewardSummary: MatchPlayerRewardSummary;
    playerResultSummary: MatchPlayerResultSummary;
    currentPlayerStats: CurrentPlayerMatchStats;
    status: string;
};

export type MatchHistoryListResponse = {
    status: string;
    data: MatchHistoryListItem[];
};

export type MatchHistoryDetailResponse = {
    status: string;
    data: MatchHistoryDetailItem;
};
