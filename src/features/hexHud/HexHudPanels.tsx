"use client";

import type { ReactNode } from "react";
import type { HexGameMessages, GameLanguage } from "@/i18n/hexGame";
import { getHeroPortrait } from "@/features/heroes/heroArtwork";
import type { HexHudView } from "./hexHudView";
import type { HexQuestArtifact, HexAttackResult, HexRoute, HexSnapshot } from "@/types/HexGameTypes";
import styles from "./HexBattleShell.module.css";
import UiText from "@/components/UiText";

export function HexBattleShell({ top, left, children, right, bottom }: { top: ReactNode; left: ReactNode; children: ReactNode; right: ReactNode; bottom: ReactNode }) {
  return <div className={styles.shell}>
    <header className={styles.top}>{top}</header>
    <aside className={styles.left}>{left}</aside>
    <main className={styles.field}>{children}</main>
    <aside className={styles.right}>{right}</aside>
    <footer className={styles.bottom}>{bottom}</footer>
  </div>;
}

export function HexTopBar({ snapshot, view, connection, language, onLanguage, labels: t }: { snapshot: HexSnapshot; view: HexHudView; connection: "connecting" | "live" | "reconnecting" | "unavailable"; language: GameLanguage; onLanguage: (language: GameLanguage) => void; labels: HexGameMessages }) {
  const turnLabel = snapshot.result ? snapshot.result.method === "defeat" ? t.defeatedWaiting : t.extraction : snapshot.finished ? t.finished : view.hero && view.hero.health <= 0 ? t.defeatedWaiting : view.isMyTurn ? t.yourTurn : t.waiting;
  const connectionLabel = connection === "live" ? { en: "Synchronized", ru: "Синхронизировано", de: "Synchronisiert" }[language] : connection === "connecting" ? { en: "Connecting", ru: "Подключение", de: "Verbinden" }[language] : connection === "unavailable" ? t.unavailable : { en: "Reconnecting", ru: "Переподключение", de: "Wiederverbinden" }[language];
  return <>
    <div className={styles.brand}><span className={styles.brandName}>NACHALO</span><span className={styles.brandMark}>⬡</span><div><strong><UiText text="HEX BATTLE" /></strong><small>{snapshot.mode || "HEX"}</small></div></div>
    <div className={styles.turn}><span className={styles.overline}>{t.turn} {snapshot.turnNumber ?? "—"}</span><strong>{turnLabel}</strong><small>{t.activePlayer}: {snapshot.activePlayerName || snapshot.actors.find(actor => actor.userId === snapshot.activeUser)?.name || `${t.player} ${snapshot.activeUser ?? "—"}`}</small></div>
    {!snapshot.finished && <div className={styles.timer}><span>{t.turnTime}</span><strong aria-live="off">{String(Math.floor(view.remaining / 60)).padStart(2, "0")}:{String(view.remaining % 60).padStart(2, "0")}</strong><progress max={view.turnSeconds} value={Math.min(view.turnSeconds, view.remaining)} aria-label={t.turnTime} /></div>}
    <div className={styles.topTools}><span className={`${styles.connection} ${connection === "live" ? styles.connectionLive : styles.connectionPending}`} role="status">● {connectionLabel}</span><label className={styles.language}>{t.language}<select aria-label={t.language} value={language} onChange={event => onLanguage(event.target.value as GameLanguage)}><option value="en">EN</option><option value="ru">RU</option><option value="de">DE</option></select></label></div>
  </>;
}

export function HexHeroPanel({ view, labels: t, language, logOpen, onLog }: { view: HexHudView; labels: HexGameMessages; language: GameLanguage; logOpen: boolean; onLog: () => void }) {
  const hero = view.hero;
  if (!hero) return null;
  const hpRatio = hero.maxHealth > 0 ? Math.max(0, Math.min(1, hero.health / hero.maxHealth)) : 0;
  const energyRatio = hero.maxEnergy > 0 ? Math.max(0, Math.min(1, hero.energy / hero.maxEnergy)) : 0;
  const condition = hero.health <= 0 ? { en: "Fallen", ru: "Погиб", de: "Gefallen" }[language] : hpRatio <= .25 ? { en: "Critical", ru: "Критическое HP", de: "Kritisch" }[language] : hpRatio <= .6 ? { en: "Wounded", ru: "Ранен", de: "Verwundet" }[language] : { en: "Ready", ru: "В строю", de: "Bereit" }[language];
  return <section className={styles.card} aria-label={hero.name}>
    <div className={styles.heroHead}><img src={getHeroPortrait(hero.characterType)} alt="" /><div><span className={styles.overline}><UiText text={hero.characterType} /></span><h2>{hero.name}</h2><span className={styles.condition}>{condition}</span></div></div>
    <div className={styles.vital}><div><span>{t.hp}</span><strong>{hero.health} / {hero.maxHealth}</strong></div><div className={`${styles.track} ${styles.healthTrack}`}><i style={{ width: `${hpRatio * 100}%` }} /></div></div>
    <div className={styles.vital}><div><span>{t.energy}</span><strong>{hero.energy} / {hero.maxEnergy}</strong></div><div className={`${styles.track} ${styles.energyTrack}`}><i style={{ width: `${energyRatio * 100}%` }} /></div></div>
    <div className={styles.heroMeta}><span>{t.attackType}<strong>{hero.isRanged ? t.ranged : t.melee}</strong></span><span>{t.range}<strong>{hero.isRanged ? hero.attackRange : 1}</strong></span><span>{t.reflex}<strong>{hero.agility}</strong></span></div>
    <button type="button" className={styles.logToggle} aria-expanded={logOpen} onClick={onLog}>{t.actionLog} <span aria-hidden="true">⌄</span></button>
  </section>;
}

export function HexTargetPanel({ view, lastAttack, approach, onCloseTarget, labels: t }: { view: HexHudView; lastAttack: HexAttackResult | null; approach?: HexRoute; onCloseTarget: () => void; labels: HexGameMessages }) {
  const { target, preview, cell, route, hero } = view;
  if (!target && !cell) return null;
  return <section className={styles.card} aria-label={target ? target.name || t.monster : t.selectedCell}>
    <span className={styles.overline}>{target ? target.kind === "player" ? t.player : t.monster : t.selectedCell}</span>
    {target ? <>
      <div className={styles.targetHeading}><h2>{target.name || target.id}</h2><button className={styles.iconButton} onClick={onCloseTarget} aria-label={t.close}>×</button></div>
      {target.health !== undefined && <p className={styles.detail}>HP <strong>{target.health}{target.maxHealth !== undefined ? ` / ${target.maxHealth}` : ""}</strong></p>}
      {preview ? <p className={styles.detail}>{t.distance} <strong>{preview.distance}</strong> · {preview.mode} · {preview.cost} {t.energy}</p> : approach ? <p className={styles.hint}>{t.route}: {approach.path.length} {t.steps} · {approach.totalCost} {t.energy}. {t.attack} after movement and server confirmation.</p> : <p className={styles.hint}>{t.attackChecking}</p>}
      {lastAttack && <p className={styles.hint}>{t.lastHit}: {lastAttack.damage} {t.damage} · {t.counterattack} {lastAttack.counterDamage}{lastAttack.effects.length ? ` · ${t.effects}: ${lastAttack.effects.map(effect => effect.kind).join(", ")}` : ""}</p>}
    </> : cell && <>
      <h2>{t.cell} ({cell.position.q}, {cell.position.r})</h2><p className={styles.hint}>{cell.terrain === "floor" ? t.floor : t.rock}</p>
      {cell.structureType && <p className={styles.detail}>{cell.structureType} · HP {cell.structureHealth}/30 · {t.team} {cell.structureOwnerUserId}{cell.isUnderConstruction ? ` · ${cell.constructionTurnsLeft} ${t.turn}` : ""}</p>}
      {cell.structureType && !cell.isUnderConstruction && hero && (cell.structureOwnerUserId !== hero.userId || cell.structureType === "wall") && <p className={styles.hint}>{t.attack} · 6 {t.energy}</p>}
      {route && <p className={styles.detail}>{t.route}: {route.path.length} {t.steps} · {route.totalCost} {t.energy} · {route.affordable ? t.confirmRoute : t.insufficientEnergy}</p>}
      {cell.resource && <p className={styles.detail}>{t.resource}: <UiText text={cell.resource.type} />{hero && (cell.position.q !== hero.position.q || cell.position.r !== hero.position.r) ? ` · ${t.standOnResource}` : ""}</p>}
      {cell.barrel && hero && (cell.position.q !== hero.position.q || cell.position.r !== hero.position.r) && <p className={styles.hint}>{t.barrel} · {t.standOnBarrel}</p>}
      {cell.isPortal && <p className={styles.hint}>{t.portal}</p>}
    </>}
  </section>;
}

export function HexQuestPanel({ quest, status, labels: t }: { quest?: HexQuestArtifact; status: string; labels: HexGameMessages }) {
  if (!quest) return null;
  return <section className={`${styles.card} ${styles.quest}`} aria-label={t.questArtifact}><div className={styles.questHead}>{quest.image ? <img src={quest.image} alt="" /> : <span aria-hidden="true">◆</span>}<div><span className={styles.overline}>{t.questArtifact}</span><strong><UiText text={quest.name} /></strong></div></div><p>{status}</p><small>{t.questGoal}</small></section>;
}

export function HexParticipantsPanel({ snapshot, labels: t }: { snapshot: HexSnapshot; labels: HexGameMessages }) {
  const players = snapshot.actors.filter(actor => actor.kind === "player" && actor.userId !== snapshot.player?.userId);
  if (!players.length) return null;
  return <section className={`${styles.card} ${styles.participants}`} aria-label={t.player}>
    <span className={styles.overline}>{t.player}</span>
    <ul>{players.map(actor => <li key={actor.id}><span>{actor.name || `${t.player} ${actor.userId}`} · {t.team} {actor.teamId}</span>{actor.health !== undefined && <strong>HP {actor.health}{actor.maxHealth !== undefined ? `/${actor.maxHealth}` : ""}</strong>}</li>)}</ul>
  </section>;
}

export function HexActionBar({ view, busy, inventoryOpen, contextAction, atPortal, hasQuest, approachReady, structureAction, idleActionLabel, bagLabel, onAttack, onStructureAttack, onInventory, onContext, onExtract, onEndTurn, labels: t }: { view: HexHudView; busy: boolean; inventoryOpen: boolean; contextAction: "collect" | "open" | null; atPortal: boolean; hasQuest: boolean; approachReady: boolean; structureAction: boolean; idleActionLabel: string; bagLabel: string; onAttack: () => void; onStructureAttack: () => void; onInventory: () => void; onContext: () => void; onExtract: () => void; onEndTurn: () => void; labels: HexGameMessages }) {
  const canAct = !busy && view.isMyTurn;
  const action = view.target ? { icon: "⚔", label: t.attack, enabled: approachReady || !!view.preview && view.preview.energy >= view.preview.cost, run: onAttack }
    : structureAction ? { icon: "⚔", label: t.attack, enabled: (view.hero?.energy ?? 0) >= 6, run: onStructureAttack }
    : atPortal && !view.finished ? { icon: "◇", label: hasQuest ? t.extractArtifact : t.extractPaid, enabled: hasQuest || (view.hero?.balance ?? 0) >= 50, run: onExtract }
    : contextAction ? { icon: contextAction === "collect" ? "✦" : "▣", label: contextAction === "collect" ? t.pickUp : t.open, enabled: true, run: onContext }
    : { icon: "✧", label: idleActionLabel, enabled: false, run: onContext };
  return <>
    <button className={styles.bagButton} aria-expanded={inventoryOpen} onClick={onInventory}><span aria-hidden="true">▣</span>{inventoryOpen ? t.closeInventory : bagLabel}<span className={styles.count}>{view.inventoryCount}</span></button>
    <button className={styles.contextButton} disabled={!canAct || !action.enabled} onClick={action.run}><span aria-hidden="true">{action.icon}</span>{busy ? "…" : action.label}</button>
    <button className={styles.endTurn} disabled={!canAct} onClick={onEndTurn}><span aria-hidden="true">⌛</span>{busy ? "…" : t.endTurn}</button>
  </>;
}
