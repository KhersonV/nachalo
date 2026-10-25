"use client";

import type { ReactNode } from "react";
import type { HexGameMessages, GameLanguage } from "@/i18n/hexGame";
import { getHeroPortrait } from "@/features/heroes/heroArtwork";
import type { HexHudView } from "./hexHudView";
import type { HexQuestArtifact, HexAttackResult, HexSnapshot } from "@/types/HexGameTypes";
import styles from "./HexBattleShell.module.css";
import DraggableHudPanel, { useHudPanel, useDesktopHud, resetHudLayout } from "./DraggableHudPanel";
import { resourceArt } from "@/utils/hexArt";
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
    <div className={styles.topTools}><details className={styles.hudMenu}><summary>HUD</summary><button type="button" onClick={resetHudLayout}>Сбросить расположение панелей</button></details><span className={`${styles.connection} ${connection === "live" ? styles.connectionLive : styles.connectionPending}`} role="status">● {connectionLabel}</span><label className={styles.language}>{t.language}<select aria-label={t.language} value={language} onChange={event => onLanguage(event.target.value as GameLanguage)}><option value="en">EN</option><option value="ru">RU</option><option value="de">DE</option></select></label></div>
  </>;
}

export function HexHeroPanel({ view, labels: t, language, logOpen, onLog, journal, actor, water }: { water: { count: number; turns: number; reason: string; onUse: () => void }; journal: ReactNode; actor?: HexSnapshot["actors"][number]; view: HexHudView; labels: HexGameMessages; language: GameLanguage; logOpen: boolean; onLog: () => void }) {
  const panel = useHudPanel("hero", "expanded");
  const desktop = useDesktopHud();
  const hero = view.hero;
  if (!hero) return null;
  const hpRatio = hero.maxHealth > 0 ? Math.max(0, Math.min(1, hero.health / hero.maxHealth)) : 0;

  const condition = hero.health <= 0 ? { en: "Fallen", ru: "Погиб", de: "Gefallen" }[language] : hpRatio <= .25 ? { en: "Critical", ru: "Критическое HP", de: "Kritisch" }[language] : hpRatio <= .6 ? { en: "Wounded", ru: "Ранен", de: "Verwundet" }[language] : { en: "Ready", ru: "В строю", de: "Bereit" }[language];
  const bars: Array<[string, number, number]> = [[t.hp, hero.health, hero.maxHealth], ["AP", hero.ap / 2, hero.maxAP / 2], ["MP", hero.mp / 2, hero.maxMP / 2]];
  const vitals = <>
    {bars.map(([label, value, maximum]) => <div className={styles.vital} key={label}><div><span>{label}</span><strong>{value} / {maximum}</strong></div><div className={`${styles.track} ${label === t.hp ? styles.healthTrack : styles.energyTrack}`}><i style={{ width: `${maximum > 0 ? Math.max(0, Math.min(100, value / maximum * 100)) : 0}%` }} /></div></div>)}
    <div className={styles.reactions}>Реакции · {hero.reactions} / {hero.maxReactions}</div>
    <button type="button" className={styles.waterButton} aria-disabled={!!water.reason} title={water.reason || "Выпить воду: +1 AP · 0 AP / 0 MP · кулдаун 2 личных хода"} onClick={water.onUse}>Вода ×{water.count} · {water.turns > 0 ? `${water.turns} ход.` : water.reason ? "недоступна" : "выпить +1 AP"}</button>
  </>;
  return <DraggableHudPanel panel={panel} title={`${hero.name} · ${hero.characterType}`} anchor="hero" width={264} closable={false} enabled={desktop} compact={vitals}>
    <section className={styles.card} aria-label={hero.name}>
      <div className={styles.heroHead}><img src={getHeroPortrait(hero.characterType)} alt="" /><div><span className={styles.overline}><UiText text={hero.characterType} /></span><h2>{hero.name}</h2><span className={styles.condition}>{condition}</span></div></div>
      {vitals}
      <div className={styles.effectBadges}>
        {hero.mpDrain && <span title="−1 MP в начале следующего хода">Истощение −1 MP</span>}
        {hero.characterType === "mystic" && <><span title="Перегруз: бонус +0 / +2 / +5 HP">Перегруз {Math.min(3, hero.overloadCount)}/3</span><span title="Состояние классового Reflex">Reflex · {hero.mysticReflexUsed ? "использован" : "готов"}</span></>}
        {hero.inGuardianAura && <span title="Вражеская аура Стража: доплата за первый шаг внутри ауры">Аура · {hero.auraSlowUsed ? "доплата применена" : "+1 MP за шаг"}</span>}
        {!!actor?.armorBreakStacks && <span title={`Владелец #${actor.armorBreakOwner}`}>Пробитие ×{actor.armorBreakStacks}</span>}
      </div>
      <details className={styles.secondaryStats}><summary>Характеристики</summary><div className={styles.heroMeta}>
        <span>Attack<strong>{actor?.attack ?? "—"}</strong></span><span>Defense<strong>{actor?.defense ?? "—"}</strong></span>
        <span>Mobility<strong>{hero.mobility}</strong></span><span>{t.reflex}<strong>{hero.agility}</strong></span>
        <span>Sight<strong>{hero.sightRange}</strong></span><span>{t.range}<strong>{hero.isRanged ? hero.attackRange : 1}</strong></span>
        <span>{t.attackType}<strong>{hero.isRanged ? t.ranged : t.melee}</strong></span>
      </div></details>
      <button type="button" className={styles.logToggle} aria-expanded={logOpen} onClick={onLog}>{t.actionLog} ⌄</button>
      {logOpen && journal}
    </section>
  </DraggableHudPanel>;
}

export function HexTargetPanel({ view, lastAttack, onCloseTarget, labels: t }: { onCloseTarget?: () => void; view: HexHudView; lastAttack: HexAttackResult | null; labels: HexGameMessages }) {
  const { target, preview, cell, route, hero } = view;
  const object = cell?.resource || cell?.barrel;
  const objectImage = resourceArt(object);
  if (!target && !cell) return null;
  return <section className={styles.card} aria-label={target ? target.name || t.monster : t.selectedCell}>
    <span className={styles.overline}>{target ? target.kind === "player" ? t.player : t.monster : t.selectedCell}</span>
    {target ? <>
      <div className={styles.targetHeading}><h2>{target.name || target.id}</h2>{onCloseTarget && <button className={styles.iconButton} onClick={onCloseTarget} aria-label={t.close}>×</button>}</div>
      {(target.armorBreakStacks ?? 0) > 0 && <p>Пробитие ×{target.armorBreakStacks} · владелец #{target.armorBreakOwner}</p>}{target.mpDrain && <p>Истощение −1 MP</p>}
      {target.health !== undefined && <p className={styles.detail}>HP <strong>{target.health}{target.maxHealth !== undefined ? ` / ${target.maxHealth}` : ""}</strong></p>}
      {preview ? <p className={styles.detail}>{t.distance} <strong>{preview.distance}</strong> · {preview.mode} · {preview.cost / 2} AP</p> : <p className={styles.hint}>{t.attackChecking}</p>}
      {preview?.route && <p className={styles.detail}>
        {t.route}: {preview.route.affordableSteps ?? 0}/{preview.route.path.length} {t.steps} · {(preview.route.reachableCost ?? 0) / 2}/{preview.route.totalCost / 2} MP<br />
        Позиция атаки: ({preview.attackPosition?.q}, {preview.attackPosition?.r})<br />
        Конец движения: ({preview.route.reachablePosition?.q}, {preview.route.reachablePosition?.r})<br />
        {preview.canAttack ? "После движения доступна атака" : preview.reason}
      </p>}
      {preview && !preview.route && preview.reason && <p className={styles.hint}>{preview.reason}</p>}
      {lastAttack && <p className={styles.hint}>{t.lastHit}: {lastAttack.damage} {t.damage} · {t.counterattack} {lastAttack.counterDamage}{lastAttack.effects.length ? ` · ${t.effects}: ${lastAttack.effects.map(effect => effect.kind).join(", ")}` : ""}</p>}
    </> : cell && <>
      <h2>{t.cell} ({cell.position.q}, {cell.position.r})</h2><p className={styles.hint}>{cell.terrain === "floor" ? t.floor : t.rock}</p>
      {objectImage && <img className={styles.objectImage} src={objectImage} alt="" />}
      {object?.description && <p className={styles.hint}><UiText text={object.description} /></p>}
      {cell.structureType && <p className={styles.detail}>{cell.structureType} · HP {cell.structureHealth}/30 · {t.team} {cell.structureOwnerUserId}{cell.isUnderConstruction ? ` · ${cell.constructionTurnsLeft} ${t.turn}` : ""}</p>}
      {cell.structureType && !cell.isUnderConstruction && hero && (cell.structureOwnerUserId !== hero.userId || cell.structureType === "wall") && <p className={styles.hint}>{t.attack} · 1 AP</p>}
      {route && <p className={styles.detail}>{t.route}: {route.affordableSteps ?? (route.affordable ? route.path.length : 0)}/{route.path.length} {t.steps} · {(route.reachableCost ?? route.totalCost) / 2}/{route.totalCost / 2} MP · {route.affordable ? t.confirmRoute : (route.affordableSteps ?? 0) > 0 ? "Подтвердить частичное перемещение" : t.insufficientEnergy} · ({route.reachablePosition?.q ?? hero?.position.q}, {route.reachablePosition?.r ?? hero?.position.r})</p>}
      {cell.resource && <p className={styles.detail}>{t.resource}: <UiText text={cell.resource.type} />{hero && (cell.position.q !== hero.position.q || cell.position.r !== hero.position.r) ? ` · ${t.standOnResource}` : ""}</p>}
      {cell.barrel && hero && (cell.position.q !== hero.position.q || cell.position.r !== hero.position.r) && <p className={styles.hint}>{t.barrel} · {t.standOnBarrel}</p>}
      {cell.isPortal && <p className={styles.hint}>{t.portal}</p>}
    </>}
  </section>;
}

export function HexQuestPanel({ quest, status, labels: t }: { quest?: HexQuestArtifact; status: string; labels: HexGameMessages }) {
  const panel = useHudPanel("quest");
  const desktop = useDesktopHud();
  if (!quest) return null;
  return <DraggableHudPanel panel={panel} title={`${t.questArtifact} · ${quest.status === "not_found" ? "Не найден" : "Найден"}`} anchor="quest" width={252} enabled={desktop}><section className={`${styles.card} ${styles.quest}`} aria-label={t.questArtifact}><div className={styles.questHead}>{quest.image ? <img src={quest.image} alt="" /> : <span aria-hidden="true">◆</span>}<div><span className={styles.overline}>{t.questArtifact}</span><strong><UiText text={quest.name} /></strong></div></div><p>{status}</p><small>{quest.description || t.questGoal}</small><p>{t.questGoal}</p><small>{t.portal}: {t.extractArtifact}</small></section></DraggableHudPanel>;
}

export function HexParticipantsPanel({ snapshot, labels: t }: { snapshot: HexSnapshot; labels: HexGameMessages }) {
  const panel = useHudPanel("participants", "closed");
  const desktop = useDesktopHud();
  const players = snapshot.actors.filter(actor => actor.kind === "player" && actor.userId !== snapshot.player?.userId);
  if (!players.length) return null;
  return <DraggableHudPanel panel={panel} title={t.player} anchor="participants" width={240} enabled={desktop}><section className={`${styles.card} ${styles.participants}`} aria-label={t.player}>
    <span className={styles.overline}>{t.player}</span>
    <ul>{players.map(actor => <li key={actor.id}><span>{actor.name || `${t.player} ${actor.userId}`} · {t.team} {actor.teamId}</span>{actor.health !== undefined && <strong>HP {actor.health}{actor.maxHealth !== undefined ? `/${actor.maxHealth}` : ""}</strong>}</li>)}</ul>
  </section></DraggableHudPanel>;
}

export function HexActionBar({ view, busy, inventoryOpen, bagLabel, onInventory, onEndTurn, onExchange, smartAction, canInspect, onInspect, labels: t }: { view: HexHudView; busy: boolean; inventoryOpen: boolean; bagLabel: string; onInventory: () => void; onEndTurn: () => void; onExchange: () => void; smartAction: ReactNode; canInspect: boolean; onInspect: () => void; labels: HexGameMessages }) {
  const canAct = !busy && view.isMyTurn;
  return <>
    <button className={styles.bagButton} aria-expanded={inventoryOpen} onClick={onInventory}><span aria-hidden="true">▣</span>{inventoryOpen ? t.closeInventory : bagLabel}<span className={styles.count}>{view.inventoryCount}</span></button>
    <div className={styles.dockCenter}>
      <button disabled={!canAct || (view.hero?.ap ?? 0) < 2 || (view.hero?.mp ?? 0) > 12} onClick={onExchange} title="1 AP → 2 MP; запас не выше 8 MP">1 AP → 2 MP</button>
      {smartAction}
      <button type="button" className={styles.infoButton} disabled={!canInspect} onClick={onInspect} aria-label="Подробности выбранного объекта" title="Подробности выбранного объекта"><svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true"><circle cx="12" cy="12" r="9" /><path d="M12 10v7m0-11v2" /></svg></button>
    </div>
    <button className={styles.endTurn} disabled={!canAct} onClick={onEndTurn}><span aria-hidden="true">⌛</span>{busy ? "…" : t.endTurn}</button>
  </>;
}
