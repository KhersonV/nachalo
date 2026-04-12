import * as React from "react";
import { useDispatch, useSelector } from "react-redux";
import type { AppDispatch, RootState } from "@/store";
import {
    finishCombatExchange,
    startCombatExchange,
} from "@/store/slices/combatPresentationSlice";
import type {
    ActiveAttackMotion,
    ActiveEffect,
    CombatActorSnapshot,
    CombatTargetRef,
    QueuedCombatExchange,
} from "@/types/combat";

type PlaybackSuppression = {
    playerIds: number[];
    monsterIds: number[];
};

type CombatPlaybackState = {
    nowMs: number;
    activeEffects: ActiveEffect[];
    activeMotions: ActiveAttackMotion[];
    suppression: PlaybackSuppression;
};

const MELEE_MOTION_MS = 320;
const MELEE_IMPACT_OFFSET_MS = 190;
const RANGED_RECOIL_MS = 180;
const RANGED_PROJECTILE_DELAY_MS = 100;
const RANGED_PROJECTILE_MS = 160;
const MAGIC_CAST_MS = 220;
const MAGIC_PROJECTILE_DELAY_MS = 140;
const MAGIC_PROJECTILE_MS = 180;
const COUNTER_DELAY_MS = 180;
const COUNTER_MOTION_MS = 240;
const COUNTER_IMPACT_OFFSET_MS = 150;
const HIT_FLASH_MS = 520;
const FLOATER_MS = 1100;
const DEATH_BURST_MS = 260;
const DEATH_FADE_MS = 420;

function getSnapshotForRef(
    exchange: QueuedCombatExchange,
    ref: CombatTargetRef,
): CombatActorSnapshot | null {
    if (
        exchange.attackerSnapshot &&
        exchange.attackerSnapshot.id === ref.id &&
        exchange.attackerSnapshot.type === ref.type
    ) {
        return exchange.attackerSnapshot;
    }
    if (
        exchange.targetSnapshot &&
        exchange.targetSnapshot.id === ref.id &&
        exchange.targetSnapshot.type === ref.type
    ) {
        return exchange.targetSnapshot;
    }
    return null;
}

function addDamageEffects(
    effects: ActiveEffect[],
    exchangeId: string,
    impactSnapshot: CombatActorSnapshot,
    index: number,
    startMs: number,
    damage: number,
) {
    effects.push({
        id: `${exchangeId}:flash:${index}`,
        exchangeId,
        kind: "hitFlash",
        cell: impactSnapshot.position,
        startMs,
        durationMs: HIT_FLASH_MS,
    });
    effects.push({
        id: `${exchangeId}:floater:${index}`,
        exchangeId,
        kind: "floater",
        cell: impactSnapshot.position,
        startMs,
        durationMs: FLOATER_MS,
        value: damage,
        isHeal: false,
    });
}

function buildCombatPlaybackPlan(exchange: QueuedCombatExchange, baseMs: number) {
    const effects: ActiveEffect[] = [];
    const motions: ActiveAttackMotion[] = [];

    const attacker = exchange.attackerSnapshot;
    const target = exchange.targetSnapshot;
    let lastImpactMs = baseMs;
    let latestBlockingEndMs = baseMs;

    if (attacker && target && attacker.type === "player") {
        const dx = Math.sign(target.position.x - attacker.position.x);
        const dy = Math.sign(target.position.y - attacker.position.y);

        if (exchange.attackStyle === "melee") {
            motions.push({
                id: `${exchange.exchangeId}:motion:attack`,
                exchangeId: exchange.exchangeId,
                actorId: attacker.id,
                actorType: attacker.type,
                kind: "lunge",
                startMs: baseMs,
                durationMs: MELEE_MOTION_MS,
                direction: { x: dx, y: dy },
                distanceTiles: 0.18,
            });
            lastImpactMs = baseMs + MELEE_IMPACT_OFFSET_MS;
            latestBlockingEndMs = Math.max(
                latestBlockingEndMs,
                baseMs + MELEE_MOTION_MS,
            );
        } else if (exchange.attackStyle === "ranged") {
            motions.push({
                id: `${exchange.exchangeId}:motion:attack`,
                exchangeId: exchange.exchangeId,
                actorId: attacker.id,
                actorType: attacker.type,
                kind: "recoil",
                startMs: baseMs,
                durationMs: RANGED_RECOIL_MS,
                direction: { x: -dx, y: -dy },
                distanceTiles: 0.08,
            });
            effects.push({
                id: `${exchange.exchangeId}:projectile:attack`,
                exchangeId: exchange.exchangeId,
                kind: "projectile",
                source: attacker.position,
                target: target.position,
                startMs: baseMs + RANGED_PROJECTILE_DELAY_MS,
                durationMs: RANGED_PROJECTILE_MS,
                attackStyle: "ranged",
            });
            lastImpactMs =
                baseMs + RANGED_PROJECTILE_DELAY_MS + RANGED_PROJECTILE_MS;
            latestBlockingEndMs = Math.max(
                latestBlockingEndMs,
                baseMs + RANGED_RECOIL_MS,
                baseMs + RANGED_PROJECTILE_DELAY_MS + RANGED_PROJECTILE_MS,
            );
        } else {
            motions.push({
                id: `${exchange.exchangeId}:motion:attack`,
                exchangeId: exchange.exchangeId,
                actorId: attacker.id,
                actorType: attacker.type,
                kind: "castPulse",
                startMs: baseMs,
                durationMs: MAGIC_CAST_MS,
            });
            effects.push({
                id: `${exchange.exchangeId}:projectile:attack`,
                exchangeId: exchange.exchangeId,
                kind: "projectile",
                source: attacker.position,
                target: target.position,
                startMs: baseMs + MAGIC_PROJECTILE_DELAY_MS,
                durationMs: MAGIC_PROJECTILE_MS,
                attackStyle: "magic",
            });
            lastImpactMs =
                baseMs + MAGIC_PROJECTILE_DELAY_MS + MAGIC_PROJECTILE_MS;
            latestBlockingEndMs = Math.max(
                latestBlockingEndMs,
                baseMs + MAGIC_CAST_MS,
                baseMs + MAGIC_PROJECTILE_DELAY_MS + MAGIC_PROJECTILE_MS,
            );
        }
    }

    for (const [index, step] of exchange.steps.entries()) {
        if (step.kind === "hit") {
            const snapshot = getSnapshotForRef(exchange, step.target);
            if (snapshot && step.damage > 0) {
                addDamageEffects(
                    effects,
                    exchange.exchangeId,
                    snapshot,
                    index,
                    lastImpactMs,
                    step.damage,
                );
            }
            continue;
        }

        if (step.kind === "counter") {
            const counterSource = getSnapshotForRef(exchange, step.source);
            const counterTarget = getSnapshotForRef(exchange, step.target);
            const counterStartMs = lastImpactMs + COUNTER_DELAY_MS;

            if (
                counterSource &&
                counterTarget &&
                counterSource.type === "player"
            ) {
                motions.push({
                    id: `${exchange.exchangeId}:motion:counter`,
                    exchangeId: exchange.exchangeId,
                    actorId: counterSource.id,
                    actorType: counterSource.type,
                    kind: "lunge",
                    startMs: counterStartMs,
                    durationMs: COUNTER_MOTION_MS,
                    direction: {
                        x: Math.sign(
                            counterTarget.position.x -
                                counterSource.position.x,
                        ),
                        y: Math.sign(
                            counterTarget.position.y -
                                counterSource.position.y,
                        ),
                    },
                    distanceTiles: 0.15,
                });
                latestBlockingEndMs = Math.max(
                    latestBlockingEndMs,
                    counterStartMs + COUNTER_MOTION_MS,
                );
            }

            const counterImpactMs = counterStartMs + COUNTER_IMPACT_OFFSET_MS;
            if (counterTarget && step.damage > 0) {
                addDamageEffects(
                    effects,
                    exchange.exchangeId,
                    counterTarget,
                    index,
                    counterImpactMs,
                    step.damage,
                );
            }
            lastImpactMs = counterImpactMs;
            latestBlockingEndMs = Math.max(
                latestBlockingEndMs,
                counterImpactMs,
            );
            continue;
        }

        const dead = getSnapshotForRef(exchange, step.target);
        if (!dead) continue;

        effects.push({
            id: `${exchange.exchangeId}:death-burst:${index}`,
            exchangeId: exchange.exchangeId,
            kind: "deathBurst",
            cell: dead.position,
            startMs: lastImpactMs,
            durationMs: DEATH_BURST_MS,
        });
        effects.push({
            id: `${exchange.exchangeId}:death-fade:${index}`,
            exchangeId: exchange.exchangeId,
            kind: "deathFade",
            actor: dead,
            startMs: lastImpactMs,
            durationMs: DEATH_FADE_MS,
        });
        latestBlockingEndMs = Math.max(
            latestBlockingEndMs,
            lastImpactMs + DEATH_FADE_MS,
        );
    }

    const latestEndMs = [...effects, ...motions].reduce((maxEnd, item) => {
        return Math.max(maxEnd, item.startMs + item.durationMs);
    }, baseMs);

    return {
        effects,
        motions,
        blockingDurationMs: latestBlockingEndMs - baseMs,
        cleanupDurationMs: latestEndMs - baseMs,
    };
}

export function useCombatPresentationPlayback(): CombatPlaybackState {
    const dispatch = useDispatch<AppDispatch>();
    const queue = useSelector((state: RootState) => state.combatPresentation.queue);
    const activeExchangeId = useSelector(
        (state: RootState) => state.combatPresentation.activeExchangeId,
    );

    const [nowMs, setNowMs] = React.useState(0);
    const [activeEffects, setActiveEffects] = React.useState<ActiveEffect[]>([]);
    const [activeMotions, setActiveMotions] = React.useState<
        ActiveAttackMotion[]
    >([]);

    const rafRef = React.useRef<number | null>(null);
    const timerIdsRef = React.useRef(new Set<number>());

    const stopTicker = React.useCallback(() => {
        if (rafRef.current !== null) {
            window.cancelAnimationFrame(rafRef.current);
            rafRef.current = null;
        }
    }, []);

    const startTicker = React.useCallback(() => {
        if (rafRef.current !== null) return;

        const tick = (ts: number) => {
            setNowMs(ts);
            rafRef.current = window.requestAnimationFrame(tick);
        };

        rafRef.current = window.requestAnimationFrame(tick);
    }, []);

    React.useEffect(() => {
        if (activeExchangeId || queue.length === 0) return;

        const exchange = queue[0];
        const baseMs = performance.now();
        const plan = buildCombatPlaybackPlan(exchange, baseMs);

        dispatch(startCombatExchange(exchange.exchangeId));
        setActiveEffects((prev) => [...prev, ...plan.effects]);
        setActiveMotions((prev) => [...prev, ...plan.motions]);
        setNowMs(baseMs);
        startTicker();

        const advanceTimer = window.setTimeout(() => {
            dispatch(finishCombatExchange(exchange.exchangeId));
            timerIdsRef.current.delete(advanceTimer);
        }, Math.max(0, Math.ceil(plan.blockingDurationMs)) + 32);

        const cleanupTimer = window.setTimeout(() => {
            setActiveEffects((prev) =>
                prev.filter((item) => item.exchangeId !== exchange.exchangeId),
            );
            setActiveMotions((prev) =>
                prev.filter((item) => item.exchangeId !== exchange.exchangeId),
            );
            timerIdsRef.current.delete(cleanupTimer);
        }, Math.max(0, Math.ceil(plan.cleanupDurationMs)) + 32);

        timerIdsRef.current.add(advanceTimer);
        timerIdsRef.current.add(cleanupTimer);
    }, [activeExchangeId, dispatch, queue, startTicker]);

    React.useEffect(() => {
        if (activeEffects.length > 0 || activeMotions.length > 0) {
            startTicker();
            return;
        }
        stopTicker();
    }, [activeEffects.length, activeMotions.length, startTicker, stopTicker]);

    React.useEffect(() => {
        return () => {
            timerIdsRef.current.forEach((timerId) => {
                window.clearTimeout(timerId);
            });
            timerIdsRef.current.clear();
            stopTicker();
        };
    }, [stopTicker]);

    const suppression = React.useMemo<PlaybackSuppression>(() => {
        const exchange =
            (activeExchangeId
                ? queue.find((item) => item.exchangeId === activeExchangeId)
                : queue[0]) ?? null;

        if (!exchange) {
            return { playerIds: [], monsterIds: [] };
        }

        return {
            playerIds: [
                exchange.attackerType === "player"
                    ? exchange.attackerId
                    : null,
                exchange.targetType === "player" ? exchange.targetId : null,
            ].filter((id): id is number => typeof id === "number"),
            monsterIds: [
                exchange.attackerType === "monster"
                    ? exchange.attackerId
                    : null,
                exchange.targetType === "monster" ? exchange.targetId : null,
            ].filter((id): id is number => typeof id === "number"),
        };
    }, [activeExchangeId, queue]);

    return {
        nowMs,
        activeEffects,
        activeMotions,
        suppression,
    };
}

export type { CombatPlaybackState };
