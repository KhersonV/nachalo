import type { GameMode } from "./MatchmakingTypes";

export type PlayerInfo = {
    playerId: number;
    level: number;
};

export type QueueSizeResponse =
    | {
          totalPlayers?: number;
      }
    | unknown[];

export type CurrentMatchResponse = {
    instance_id?: string | null;
};

export type InQueueResponse = {
    inQueue?: boolean;
    mode?: GameMode | string;
};
