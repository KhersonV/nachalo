import type { HexActor, HexAttackPreview, HexCell, HexSnapshot } from "@/types/HexGameTypes";
import { uiText } from "@/i18n/uiText";
import type { GameLanguage } from "@/i18n/hexGame";
import { hexDistance, hexKey } from "@/utils/hexGrid";

export type SmartActionKind = "none" | "attack" | "structure" | "collect" | "pickup" | "barrel" | "portal";
export type SmartAction = {
  icon?: "ore" | "forest" | "remains" | "herbs";
  kind: SmartActionKind;
  target?: HexActor | HexCell;
  label: string;
  enabled: boolean;
  cost: { ap: number; mp: number }; // API half points, like the server snapshot.
  disabledReason: string;
  pending: boolean;
  distance?: number;
  hint?: string;
};
type Options = { language?: GameLanguage; snapshot: HexSnapshot; selected: { q: number; r: number } | null; target?: HexActor; preview?: HexAttackPreview; failure?: string; pending: boolean; unavailable: boolean };
const budgetReason = (name: string, required: number, available: number) => `Недостаточно ${name}: нужно ${required / 2}, доступно ${available / 2}`;

/** Client UX only. Attack preview and commands still enforce visibility, LOS and rules on the server. */
export function resolveSmartAction({ snapshot, selected, target, preview, failure, pending, unavailable, language = "en" }: Options): SmartAction {
  const l = (source: string) => uiText(language, source);
  const hero = snapshot.player;
  const action: SmartAction = { kind: "none", label: "Нет доступного действия", enabled: false, cost: { ap: 0, mp: 0 }, disabledReason: "Выберите объект", pending };
  if (!hero) return action;
  const visibleCell = (cell: HexCell) => cell.visibility === undefined || cell.visibility === "visible";
  const current = snapshot.cells.find(cell => hexKey(cell.position) === hexKey(hero.position));
  const cell = selected ? snapshot.cells.find(item => hexKey(item.position) === hexKey(selected)) : current;
  if (target && (target.health ?? 0) > 0 && target.userId !== hero.userId && !(target.kind === "player" && hero.teamId !== undefined && target.teamId === hero.teamId)) {
    action.kind = "attack"; action.target = target; action.label = `Атаковать: ${target.name || target.id}`; action.cost.ap = preview?.cost ?? 2;
    action.distance = hexDistance(hero.position, target.position);
    const targetCell = snapshot.cells.find(item => hexKey(item.position) === hexKey(target.position));
    const steps = preview?.route?.affordableSteps ?? 0;
    action.disabledReason = !targetCell || !visibleCell(targetCell) ? "Цель не видна" : failure || (preview?.canAttack === false && steps === 0 ? preview.reason || "Атака недоступна" : "");
    action.cost.mp = preview?.route?.reachableCost ?? 0;
    action.label = !preview ? "Предпросмотр атаки" : preview.route ? preview.canAttack ? "Подойти и атаковать" : "Частично сблизиться" : `Атаковать: ${target.name || target.id}`;
    action.hint = preview?.reason || (preview ? "Повторное нажатие — подтверждение" : "Первое нажатие — предпросмотр");
  } else if (cell && visibleCell(cell)) {
    const onHero = hexKey(cell.position) === hexKey(hero.position);
    action.target = cell;
    if (cell.resource && (cell.resource.item_type !== "artifact" || !cell.barrel)) {
      action.kind = cell.resource.item_type === "artifact" ? "pickup" : "collect";
      action.label = `${action.kind === "pickup" ? "Подобрать" : "Собрать"}: ${cell.resource.type}`; action.cost.ap = 1;
      action.disabledReason = onHero ? "" : "Сначала переместитесь на клетку предмета";
      const node = cell.resource.professionNode;
      if (node) {
        action.icon = node.category; action.label = l(cell.resource.type);
        action.hint = l("Peaceful professional gathering");
        action.disabledReason = !onHero ? l("Stand on this object to gather it.") : !hero.professionUnlocked ? l("Gathering requires the Artisans' Guild at match start.") : node.class !== hero.characterType ? l("Required profession: {profession}").replace("{profession}", l(node.professionName)) : (hero.professionGatherCount ?? 0) >= (hero.professionGatherLimit ?? 10) ? l("Peaceful gathering limit reached for this match.") : "";
      }
    } else if (cell.barrel) {
      action.kind = "barrel"; action.label = "Открыть бочку"; action.cost.ap = 1;
      action.disabledReason = onHero ? "" : "Сначала переместитесь на клетку бочки";
    } else if (cell.isPortal && snapshot.mode !== "hex-test") {
      action.kind = "portal"; action.label = "Выйти через портал"; action.cost.mp = 2;
      const quest = snapshot.questArtifactId && (hero.inventory?.[`artifact_${snapshot.questArtifactId}`]?.item_count ?? 0) > 0;
      action.hint = quest ? "Выход с квестовым артефактом" : "Требуется квестовый артефакт или 50 золота; оплата после подтверждения";
      action.disabledReason = !onHero ? "Сначала переместитесь на клетку портала" : !quest && (hero.balance ?? 0) < 50 ? "Требуется квестовый артефакт или 50 золота" : "";
    } else if (cell.structureType && !cell.isUnderConstruction && (cell.structureOwnerUserId !== hero.userId || cell.structureType === "wall")) {
      action.kind = "structure"; action.label = `Атаковать: ${cell.structureType}`; action.cost.ap = 2;
      action.disabledReason = hexDistance(hero.position, cell.position) === 1 ? "" : "Для атаки сооружения встаньте на соседнюю клетку";
    }
  }
  if (action.kind === "none") return action;
  // Budget is checked before any preview or command, without changing selection.
  if (hero.ap < action.cost.ap && (!action.icon || !action.disabledReason)) action.disabledReason = action.icon ? l("Not enough AP: need {need}, available {available}").replace("{need}",String(action.cost.ap / 2)).replace("{available}",String(hero.ap / 2)) : budgetReason("ОД", action.cost.ap, hero.ap);
  else if (hero.mp < action.cost.mp) action.disabledReason = budgetReason("ОП", action.cost.mp, hero.mp);
  if (unavailable) action.disabledReason = action.icon ? l("Connection unavailable") : "Соединение недоступно";
  else if (snapshot.finished || snapshot.result || hero.health <= 0) action.disabledReason = action.icon ? l("Match finished.") : "Матч завершён";
  else if (snapshot.activeUser !== hero.userId) action.disabledReason = action.icon ? l("Wait for your turn.") : "Дождитесь своего хода";
  else if (pending) action.disabledReason = action.icon ? l("Working…") : "Выполняется действие…";
  action.enabled = !action.disabledReason;
  return action;
}
