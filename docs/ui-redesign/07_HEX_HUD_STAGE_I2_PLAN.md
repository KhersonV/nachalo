# Stage I.2 — план реализации HEX Battle HUD

Основа: `05_HEX_HUD_INVENTORY.md` и `06_HEX_HUD_ARCHITECTURE.md`. Stage I.2 меняет представление и границы компонентов, сохраняя server snapshot, команды, порядок событий и текущий `HexMap` как работающий renderer.

## Последовательность

| Шаг | Работы и предполагаемые файлы | Выходной контроль |
| --- | --- | --- |
| 1. Зафиксировать поведение | Целевые тесты для существующих `src/components/HexGameController.tsx`, `src/hooks/useHexMatchSnapshot.ts`, `src/utils/hexCombatIndicators.ts` и fixtures схемы 2 | Проверены повторный `commandId`, устаревшая ревизия, route/preview, death/result. Не писать тесты, повторяющие JSX. |
| 2. Чистая проекция | Новый `src/features/hexHud/hexHudView.ts` и типы рядом; selector из `HexSnapshot` + временного UI state; тесты filtered target, turn, deadline и пустых полей | Ни fetch, ни новый store, ни боевые формулы; проекция не показывает данные отсутствующего actor/cell. |
| 3. Пассивные панели | `HexBattleShell`, `HexTopBar`, `HexHeroPanel`, `HexTargetPanel` в `src/features/hexHud/`; стили `HexBattleShell.module.css`; интеграция в `HexGameController.tsx` | Hero HP/Energy, range, class, target preview, mode/ход/таймер и ошибки совпадают с действующим HUD. Connection status берётся из того же hook. |
| 4. Действия и вторичные панели | `HexActionBar`/`HexTurnControls`, обёртки `ActionLog` и `Inventory`; правки только props/layout; кнопки передают callbacks старого контроллера | Route/move, attack, structure, collect/barrel, use-item, blueprint, extract, end-turn, Refresh работают без нового обработчика команд. Quick items используют тот же inventory. |
| 5. Граница поля и адаптивность | Узкий adapter для `HexMap` props, CSS shell и responsive states; `HexMap.tsx`/`HexMap.module.css` трогать только для передачи данных/подсветки, не переписывать SVG и `useHexCamera` | Панели не привязаны к `RADIUS`, viewBox или пикселям; карта остаётся просторной, на mobile действия доступны. Контракт годится для будущей 3D сцены. |
| 6. Регрессия и ручная приёмка | TypeScript/Next typegen при необходимости, целевые FE tests, `git diff --check`; ручной чеклист ниже. Go tests только если когда-либо менялся Go (для Stage I.2 не планируется) | Все сценарии ниже подтверждены; нет новых API/DB/правил. |

## Компонентные границы

- `HexBattleShell`: раскладка и responsive accessibility, без игрового состояния.
- `HexTopBar`/`HexConnectionStatus`: номер хода, active player, локальное отображение серверного deadline, mode и состояние той же WS подписки.
- `HexHeroPanel`: `snapshot.player` и классовый портрет при наличии; только HP и Energy как ресурсы HEX. Эффекты — только подтверждённые данные, не вычисляемый cooldown.
- `HexTargetPanel`: filtered actor, `attackPreview`, last attack, выбранная структура/клетка, clear intent; ничего о скрытом противнике.
- `HexActionBar`: базовая атака из server preview, существующие контекстные действия, портальное подтверждение, inventory/quick items и End Turn. Возможные пассивные способности — справочная информация, не кнопки действий.
- `ActionLog`, `Inventory`, `HexMatchResults`: переиспользуются; их внутренняя Square ветка не меняется.
- `HexMap`: только визуальная карта, q/r callbacks, route/target highlight и visual indicators.

Точный набор новых файлов допускает упрощение после первого PR; избегать десятков мелких компонентов. В `HexGameController.tsx` остаются владельцы selection/pending command, orchestration POST/refresh и error handling. `src/types/HexGameTypes.ts` менять лишь при реальном существующем поле server DTO, не ради выдуманной ability модели. `src/hooks/useHexMatchSnapshot.ts` можно расширить только статусом соединения; reconnect/revision code остаётся единым.

## Функции, которые нельзя потерять

`GameWrapper` 409 → HEX, `hex-test` путь, q/r выбор, два клика для route/move, attack-preview перед атакой, monster/PvP targetKind и запрет союзной цели; structure attack, collect, barrel, расходник и scroll, blueprint placement/cancel, портал (artifact/paid + confirmation), End Turn `expectedTurn`, отдельный Refresh, язык EN/RU/DE, debug revisions, Quest artifact, combat indicators/deduplication, ActionLog, timeout/presence/lightning/exit notices, disconnect countdown, персональный result до `finished`, repeat mode/возврат. При смерти, эвакуации или недоступности матча управление блокируется, но результат/ошибка остаются доступны. Не изменять `commandId`, сериализацию через `actionInFlight`, POST → refresh, WS `MATCH_UPDATE`/`MATCH_PATCH`/`MATCH_HEX_VISUAL`, HTTP fallback, серверный Fog.

## Сценарии проверки

1. `PVE`, `1x1`, `1x2`, `2x2`, `3x3`, `5x5`: panel по фактическим `actors/teamId`, без предположения о числе игроков; проверить монстра, видимого врага, союзника и скрытую цель.
2. Guardian, Ranger, Berserker, Mystic: реальные HP/Energy/range, серверный preview и эффекты в ответе; отсутствие ложных active ability/cooldown кнопок. Guardian aura меняет стоимость маршрута только через серверный preview.
3. Клик клетки → route → подтверждение, недостаток Energy, rejected stale move; target preview → attack/counterattack/death; structure attack; collect/barrel; item cooldown reject; blueprint placement и cancel; paid/artifact extract; End Turn и timeout.
4. WS отключение/возврат: status, HTTP refresh после JOIN, монотонный `stateRevision`, визуальные notices без повтора по `commandId`, ошибка sync и недоступный матч. Проверить смену видимости при reconnect и очистку устаревшего target/route.
5. Умирает один игрок, другой продолжает: первый видит личный `HexMatchResults` до общего `finished`, второй сохраняет управление; reload обоих сохраняет серверный результат. Награды из `result`, а не из локального лога.
6. 1440×900 и узкий mobile viewport: крупная карта, доступные controls, читаемые панели, keyboard focus, scroll инвентаря/лога без потери End Turn; эти пункты требуют ручной проверки в Stage I.2, не браузера в I.1.

## Риски и защита

| Риск | Защита |
| --- | --- |
| Второй источник состояния или optimistic HP/loot | Одна копия `useHexMatchSnapshot`; view model чистый и пересоздаётся из snapshot. |
| Двойная команда/иная очередность | Сохранить `pending*` refs, `commandId`, `actionInFlight`, `expectedTurn` и тела вызовов; проверять rejected/retry. |
| Утечка Fog в target/quest/log | Проецировать только персональный filtered snapshot и подтверждённые visual notices; удалять устаревший выбор. |
| Ложные abilities/quick slots | Не выводить активные способности без server contract; quick items читают текущий inventory и вызывают имеющийся `useItem`. |
| Таймер и disconnect как локальное правило | Отображать `turnDeadline/disconnectedPlayers.deadline`, не выполнять end-turn/lightning на клиенте. |
| Разрыв Square/HEX shared компонентов | Проверять HEX props `Inventory` и `PlayerHUD` отдельно, Square branch оставить без изменений. |
| 3D сцена требует переписать HUD | Контракт только axial q/r, filtered data и callbacks, никакого viewBox/DOM node в HUD. |

## Готовность Stage I.2

Старт безопасен после принятия этого перечня и создания тестовых фикстур для ключевых переходов; блокирующих пробелов в текущих HEX командах для **визуального** редизайна не найдено. Приёмка Stage I.2: F63–F77 сохранены, нет активных несуществующих способностей, HUD не читает renderer internals, renderer можно заменить по adapter контракту, серверный snapshot единственный источник gameplay, Fog/revisions/reconnect/result не регрессировали, целевые проверки и ручные сценарии пройдены. Полноценный серверный combat replay/история эффектов и новые ability API находятся вне этого этапа.
