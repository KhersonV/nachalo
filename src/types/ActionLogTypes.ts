export type ActionLogCategory =
  | "movement" | "attack" | "counterattack" | "effect" | "defeat"
  | "resource" | "barrel" | "artifact" | "equipment" | "portal"
  | "match" | "turn" | "error";

export type ActionLogTone = "info" | "success" | "warning" | "danger";

export type ActionLogEntry = {
  id: string;
  timestamp: number;
  category: ActionLogCategory;
  message: string;
  tone?: ActionLogTone;
  dedupeKey?: string;
};
