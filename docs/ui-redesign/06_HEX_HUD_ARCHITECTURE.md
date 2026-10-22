# Stage I.1 — архитектура HEX HUD

## Текущая цепочка

```text
/game → GameWrapper (Square 409 → GET personal HEX snapshot)
      → HexGameController (snapshot hook + UI state + commands + layout)
      → HexMap (SVG/camera/selection/indicators)
      → PlayerHUD / ActionLog / Inventory / HexMatchResults
WS JOIN_MATCH → MATCH_UPDATE or MATCH_PATCH → useHexMatchSnapshot → snapshot → panels/map
HUD or map intent → controller POST → server response → refresh → personal snapshot
```

`useHexMatchSnapshot` — единственный authoritative **client copy** серверного персонального snapshot. Его `revision` ref отклоняет устаревший GET/UPDATE. `HexGameController` держит выбранную клетку/цель, route/attack preview, placement, busy, ошибки, локальный log, `pending*` commandId и дедупликацию эффектов. `HexMap` получает snapshot целиком, рассчитывает SVG геометрию и камеру (`useHexCamera`), отдаёт клики по axial q/r/actor. `Inventory` в HEX режиме вызывает колбэки контроллера; `HexMatchResults` читает personal `snapshot.result`. Серверные команды/валидаторы находятся в `beckend/gameservice/handlers/hex_match.go`, `hex_combat.go`, `hex_pvp.go`, `repository/hex_move.go`, `hex_fog.go`, `hex_match.go`, `hex_turn.go`; timer и lightning обслуживаются `handlers/hex_turn_timer.go`.

## Узкие места

1. `HexGameController.tsx` одновременно владеет состоянием, последовательностью команд и всей разметкой. Перестановка панелей в нём легко меняет `busy`, `commandId`, refresh или порядок действия.
2. `HexMap` получает весь snapshot, хотя ему нужны filtered map/actor данные, renderer selection и индикаторы. Это расширяет будущую границу 3D и повышает риск утечки персональных данных в scene props.
3. `PlayerHUD` считает и подписывает Reflex по общей утилите; это информация, а не подтверждение применённого эффекта. Активные эффекты доступны в результате конкретного удара, но не представлены долговременным списком в snapshot.
4. `ActionLog` заполняется локальными результатами действий этого клиента. После reload/reconnect он пуст; WS события других игроков дают visual notices и refresh, но не полный журнал. Нельзя обещать исторический replay без API.
5. `useHexMatchSnapshot` сообщает `syncError/unavailable`, но не публикует фазу соединения. UI Stage I.2 должен показать честное состояние подключения на основе существующего socket lifecycle.
6. Текущие `route/target/selectedCell` локальны и могут устареть при `stateRevision`/Fog. `route` сбрасывается, но цель и клетку нужно перепроверять по новому filtered snapshot без раскрытия скрытого.
7. CSS shell накладывает две scrollable панели поверх поля; на мобильной ширине правый HUD занимает нижнюю часть карты. Переделка расположения нужна, но геометрия SVG не должна определять положение HUD.

## Предлагаемая граница

Оставить `useHexMatchSnapshot` и командные функции в одном controller hook/контроллере. Не создавать второй store, WS клиент или HTTP координатор. Выделить чистую проекцию `toHexHudView(snapshot, ui, now)` без side effects: она форматирует только доступные поля и вычисляет **презентационные** состояния (`isMyTurn`, секунды для отображения, selected visible target, доступность локальных кнопок по текущему preview). Команда всё равно всегда проходит серверную проверку. В проекции не вычислять путь, LoS, урон, цену, cooldown или Fog.

```ts
type HexFieldView = {
  cells: HexSnapshot['cells']; edges: HexSnapshot['edges']; actors: HexSnapshot['actors'];
  playerPosition?: HexCoord; selectedCell?: HexCoord | null;
  selectedTarget?: { id: number; kind: 'player' | 'monster' } | null;
  route?: HexRoute | null; indicators: HexCombatIndicator[];
};
type HexFieldIntent =
  | { type: 'selectCell'; position: HexCoord }
  | { type: 'selectActor'; actorId: string };
type HexHudView = {
  mode?: string; turn: { number?: number; activeUser?: number; deadline?: string; isMine: boolean };
  hero?: HexSnapshot['player']; target?: HexActor; preview?: HexAttackPreview;
  cell?: HexCell; inventory: NonNullable<HexPlayer['inventory']>;
  quest?: HexQuestArtifact; result?: HexPlayerResult;
  connection: 'connecting' | 'live' | 'reconnecting' | 'unavailable';
};
```

Это **эскиз интерфейсов для Stage I.2**, не новый API. В реальном коде ссылки на типы импортируются из `HexGameTypes.ts`, а callback `onCellSelect/onActorSelect` адаптирует id к существующему контроллеру. Renderer получает только `HexFieldView` и callbacks, владеет камерой/hover/SVG анимацией, но не токеном, fetch, правилами действия, наградами или HUD layout. HUD получает `HexHudView`, notices/log и intent callbacks (`attack`, `useItem`, `endTurn`, `extract`, `refresh`, `cancelPlacement`); он не читает Canvas/Three.js или пиксельные координаты. Индикаторы, привязанные к q/r, остаются в renderer; текстовый combat log — в HUD. При переходе к 3D заменяется реализация renderer adapter, контракты axial q/r и командный controller сохраняются.

## Композиция Stage I.2

`HexBattleShell` размещает верхнюю строку (mode/ход/таймер/connection), слева героя (HP, Energy, class, range и только достоверные эффекты), центр с renderer, справа filtered target и клеточный контекст, снизу атаку, контекст, инвентарь/quick items и End Turn. `ActionLog`, `Inventory`, `HexMatchResults` сохраняются как существующие сценарные компоненты, пока им не понадобится тонкая визуальная оболочка. Quick items — представление `snapshot.player.inventory` через существующий `useItem`, без отдельного набора или второй копии inventory. Ability bar не должна показывать активные кнопки, пока backend не предоставит такие действия; нынешние классовые passives можно пояснить информационно. На мобильной ширине панели переходят в компактные раскрываемые области и не перекрывают кнопки поля.

## Безопасная миграция

1. Зафиксировать поведение текущих действий и источников (инвентаризация `05_…`).
2. Ввести чистый view model/типы для чтения того же snapshot, покрыть тестами переходы turn, death/result, filtered target и deadline. Не дублировать authoritative snapshot.
3. Выносить сначала пассивные панели из JSX контроллера; сравнивать данные и состояния на каждом шаге. `PlayerHUD`/`ActionLog` можно обернуть без переноса команд.
4. Вынести controls через callbacks, оставив тела `selectCell`, `attackTarget`, `useItem`, `endTurn`, `extract`, `post`, `command`, `refresh` в прежнем владельце. Не менять порядок POST → результат → refresh, retries или `commandId`.
5. Добавить узкий adapter props для `HexMap` без изменения SVG/камеры. Лишь после стабильности интерфейса можно подменять renderer 3D.
6. Отдельно проверить reconnect, старые revisions, Fog и личный результат. Ошибки command reject не превращать в оптимистическое изменение HP/Energy; результат сервера и новый snapshot остаются истиной.

Не переносить Square Redux, SSE matchmaking, sessionStorage результата Square и механику HEX в общую сущность. Можно переиспользовать визуальные блоки, сохраняя раздельные источники.
