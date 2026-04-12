export type AttackStyle = "melee" | "ranged" | "magic";

export type CombatActorType = "player" | "monster";

export type CombatTargetRef = {
    id: number;
    type: CombatActorType;
};

export type CombatPoint = {
    x: number;
    y: number;
};

export type CombatActorSnapshot = CombatTargetRef & {
    position: CombatPoint;
    image?: string;
};

export type CombatStep =
    | {
          kind: "hit";
          source: CombatTargetRef;
          target: CombatTargetRef;
          damage: number;
          targetHpAfter: number;
      }
    | {
          kind: "counter";
          source: CombatTargetRef;
          target: CombatTargetRef;
          damage: number;
          targetHpAfter: number;
      }
    | {
          kind: "followup";
          source: CombatTargetRef;
          target: CombatTargetRef;
          damage: number;
          targetHpAfter: number;
      }
    | {
          kind: "bonus";
          source: CombatTargetRef;
          target: CombatTargetRef;
          damage: number;
          targetHpAfter: number;
      }
    | {
          kind: "death";
          target: CombatTargetRef;
      };

export type CombatEffect =
    | {
          kind: "armorBreak";
          source?: CombatTargetRef;
          target?: CombatTargetRef;
          value?: number;
          stacks?: number;
          durationTurns?: number;
          succeeded: boolean;
      }
    | {
          kind: "push";
          source?: CombatTargetRef;
          target?: CombatTargetRef;
          succeeded: boolean;
          positionAfter?: CombatPoint;
          bonusDamage?: number;
          energyGranted?: number;
      }
    | {
          kind: "energyDrain";
          source?: CombatTargetRef;
          target?: CombatTargetRef;
          succeeded: boolean;
          energyGranted?: number;
          energyDrained?: number;
          sourceEnergyAfter?: number;
          targetEnergyAfter?: number;
      };

export type CombatExchangePayload = {
    instanceId: string;
    exchangeId: string;
    attackerId: number;
    attackerType: CombatActorType;
    targetId: number;
    targetType: CombatActorType;
    attackStyle: AttackStyle;
    steps: CombatStep[];
    effects?: CombatEffect[];
};

export type QueuedCombatExchange = CombatExchangePayload & {
    attackerSnapshot: CombatActorSnapshot | null;
    targetSnapshot: CombatActorSnapshot | null;
};

export type ActiveAttackMotion =
    | {
          id: string;
          exchangeId: string;
          actorId: number;
          actorType: CombatActorType;
          kind: "lunge";
          startMs: number;
          durationMs: number;
          direction: { x: number; y: number };
          distanceTiles: number;
      }
    | {
          id: string;
          exchangeId: string;
          actorId: number;
          actorType: CombatActorType;
          kind: "recoil";
          startMs: number;
          durationMs: number;
          direction: { x: number; y: number };
          distanceTiles: number;
      }
    | {
          id: string;
          exchangeId: string;
          actorId: number;
          actorType: CombatActorType;
          kind: "castPulse";
          startMs: number;
          durationMs: number;
      };

export type ActiveEffect =
    | {
          id: string;
          exchangeId: string;
          kind: "projectile";
          source: CombatPoint;
          target: CombatPoint;
          startMs: number;
          durationMs: number;
          attackStyle: "ranged" | "magic";
      }
    | {
          id: string;
          exchangeId: string;
          kind: "hitFlash";
          cell: CombatPoint;
          startMs: number;
          durationMs: number;
      }
    | {
          id: string;
          exchangeId: string;
          kind: "floater";
          cell: CombatPoint;
          startMs: number;
          durationMs: number;
          value: number;
          isHeal: boolean;
      }
    | {
          id: string;
          exchangeId: string;
          kind: "deathBurst";
          cell: CombatPoint;
          startMs: number;
          durationMs: number;
      }
    | {
          id: string;
          exchangeId: string;
          kind: "deathFade";
          actor: CombatActorSnapshot;
          startMs: number;
          durationMs: number;
      };

export type CombatPresentationState = {
    queue: QueuedCombatExchange[];
    seenExchangeIds: Record<string, true>;
    activeExchangeId: string | null;
};
