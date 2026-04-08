export type GameMode = "PVE" | "1x1" | "1x2" | "2x2" | "3x3" | "5x5";

export type FriendSummary = {
    userId: number;
    name: string;
    image: string;
    characterType: string;
    level: number;
    activityStatus: "in_match" | "in_lobby" | "offline";
};

export type PartyMemberState = {
    user_id: number;
    name: string;
    image: string;
    characterType: string;
    level: number;
};

export type PartyStateResponse = {
    inParty: boolean;
    partyId?: string;
    leaderId: number;
    isLeader: boolean;
    members: PartyMemberState[];
    partySize: number;
    queueMode?: string;
};

export type PartyInviteState = {
    leader: PartyMemberState;
    partyId?: string;
    createdAt: string;
};

export type PartyInvitesResponse = {
    status: string;
    invites: PartyInviteState[];
};
