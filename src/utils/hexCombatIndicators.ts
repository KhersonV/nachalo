import type { HexCombatEffect, HexCombatIndicator, HexCombatRef, HexCombatStep, HexSnapshot } from "@/types/HexGameTypes";
import type { HexCoord } from "@/utils/hexGrid";

const actorKey = (ref: HexCombatRef) => `${ref.type}-${ref.id}`;

type AttackVisual = { steps?: HexCombatStep[]; effects?: HexCombatEffect[]; targetHP?: number; attackerHPBefore?: number; attackerPosition?: HexCoord; targetPosition?: HexCoord };
type MoveVisual = { position: HexCoord; auraEnergyCost?: number; auraHits?: Array<{ position: HexCoord; damage: number }>; auraEnded?: boolean };

export function hexAttackIndicators(snapshot: HexSnapshot, result: AttackVisual, eventId: string): HexCombatIndicator[] {
  const positions = new Map(snapshot.actors.map(actor => [actor.id, actor.position]));
  const health = new Map(snapshot.actors.map(actor => [actor.id, actor.health ?? 0]));
  const initialHit = result.steps?.find(step => step.kind === "hit");
  if (initialHit?.source && result.attackerPosition) positions.set(actorKey(initialHit.source), result.attackerPosition);
  if (initialHit?.target && result.targetPosition) positions.set(actorKey(initialHit.target), result.targetPosition);
  if (initialHit?.source && typeof result.attackerHPBefore === "number") health.set(actorKey(initialHit.source), result.attackerHPBefore);
  if (initialHit?.target && typeof result.targetHP === "number") health.set(actorKey(initialHit.target), result.targetHP);
  const out: HexCombatIndicator[] = [];
  const add = (ref: HexCombatRef | undefined, text: string, tone: HexCombatIndicator["tone"], position?: HexCoord) => {
    const at = position ?? (ref && positions.get(actorKey(ref)));
    if (!at) return;
    const lane = out.filter(item => item.position.q === at.q && item.position.r === at.r).length;
    out.push({ id: `${eventId}:${out.length}`, position: at, text, tone, lane });
  };
  const criticalTargets = new Set((result.effects ?? []).filter(effect => effect.kind === "crit" && effect.succeeded !== false && effect.target).map(effect => actorKey(effect.target!)));
  const criticalShown = new Set<string>();
  const pendingHeal = new Map((result.effects ?? []).filter(effect => effect.kind === "lifesteal" && effect.source && (effect.amount ?? 0) > 0).map(effect => [actorKey(effect.source!), effect.amount!]));
  for (const step of result.steps ?? []) {
    if (typeof step.damage !== "number" || step.damage <= 0) continue;
    const key = actorKey(step.target);
    const before = health.get(key);
    const lost = before === undefined ? step.damage : Math.min(step.damage, Math.max(0, before));
    if (typeof step.targetHpAfter === "number") health.set(key, step.targetHpAfter);
    if (lost <= 0) continue;
    const critical = step.kind === "hit" && criticalTargets.has(key) && !criticalShown.has(key);
    if (critical) criticalShown.add(key);
    add(step.target, critical ? `КРИТ! -${lost} HP` : `-${lost} HP`, critical ? "critical" : "damage");
    if (step.kind === "hit" && step.source) {
      const sourceKey = actorKey(step.source);
      const heal = pendingHeal.get(sourceKey);
      if (heal) { health.set(sourceKey, (health.get(sourceKey) ?? 0) + heal); pendingHeal.delete(sourceKey); }
    }
  }
  for (const effect of result.effects ?? []) {
    if (effect.succeeded === false && effect.kind !== "push") continue;
    if (effect.kind === "lifesteal" && (effect.amount ?? 0) > 0) add(effect.source, `+${effect.amount} HP`, "heal");
    if (effect.kind === "energyDrain") {
      if ((effect.energyDrained ?? effect.amount ?? 0) > 0) add(effect.target, `-${effect.energyDrained ?? effect.amount} энергии`, "energyLoss");
      if ((effect.energyGranted ?? 0) > 0) add(effect.source, `+${effect.energyGranted} энергии`, "energyGain");
    }
    if (effect.kind === "push") {
      const destination = effect.positionAfter && { q: effect.positionAfter.x, r: effect.positionAfter.y };
      add(effect.target, effect.succeeded ? "Отталкивание" : "Отталкивание блокировано", "push", destination);
      const targetKey = effect.target && actorKey(effect.target);
      if (!effect.succeeded && targetKey && (effect.bonusDamage ?? 0) > 0 && !(result.steps ?? []).some(step => step.kind === "bonus" && actorKey(step.target) === targetKey)) {
        const lost = Math.min(effect.bonusDamage!, Math.max(0, health.get(targetKey) ?? effect.bonusDamage!));
        if (lost > 0) add(effect.target, `-${lost} HP`, "damage");
      }
      if ((effect.energyGranted ?? 0) > 0) add(effect.source, `+${effect.energyGranted} энергии`, "energyGain");
    }
    if (effect.kind === "armorBreak") add(effect.target, `Пробитие брони${effect.stacks ? ` ×${effect.stacks}` : ""}`, "status");
    if (effect.kind === "armorBreakEnd") add(effect.target, "Пробитие снято", "status", effect.positionAfter && { q: effect.positionAfter.x, r: effect.positionAfter.y });
    if (effect.kind === "guardianAuraEnd") add(effect.target, "Аура снята", "status", effect.positionAfter && { q: effect.positionAfter.x, r: effect.positionAfter.y });
    if (effect.kind === "block") add(effect.target, "Блок", "status");
    if (effect.kind === "arcaneOverburn") add(effect.target, "Перегрузка", "status");
    if (effect.kind === "pureDamage" && (effect.amount ?? 0) > 0 && !(result.steps ?? []).some(step => effect.target && actorKey(step.target) === actorKey(effect.target))) add(effect.target, `-${effect.amount} HP`, "damage");
  }
  return out;
}

export function hexMoveIndicators(result: MoveVisual, eventId: string): HexCombatIndicator[] {
  const out: HexCombatIndicator[] = [];
  const add = (position: HexCoord, text: string, tone: HexCombatIndicator["tone"]) => out.push({ id: `${eventId}:${out.length}`, position, text, tone, lane: out.filter(item => item.position.q === position.q && item.position.r === position.r).length });
  if ((result.auraEnergyCost ?? 0) > 0) add(result.position, `-${result.auraEnergyCost} энергии · аура`, "aura");
  for (const hit of result.auraHits ?? []) if (hit.damage > 0) add(hit.position, `-${hit.damage} HP · аура`, "aura");
  if (result.auraEnded) add(result.position, "Аура снята", "status");
  return out;
}
