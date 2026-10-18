# NACHALO: последовательный план полной миграции

План реализации после отдельного назначения работ. В этом задании создана только документация. Текущее устройство и пути проверены в [ARCHITECTURE_AUDIT.md](ARCHITECTURE_AUDIT.md); будущий контракт — в [HEX_DESIGN.md](HEX_DESIGN.md). Новые пути в этом плане явно обозначены как предлагаемые; они не описывают существующие файлы.

## Принципы выпуска

Окончательное состояние — HEX во всех новых матчах, Classic, 3D и серверных правилах. Временная поддержка square нужна только для безопасного завершения/восстановления старых матчей. Не запускать HEX-клиент против unversioned square API. Включение создания HEX отделить от установки кода и аддитивной схемы.

Каждый этап — отдельный проверяемый change set. Не менять формулы классов, экономику, длительность хода (сейчас 60 секунд), награды или группы одновременно с геометрией без явной причины. Новые серверные проверки LOS/пространства — запланированное изменение поведения и должны иметь собственные регрессионные сценарии. Зависимости не устанавливать автоматически; выбор будущего test harness обсуждается отдельно.

## Этап 0 — самый первый безопасный и независимый шаг

**Создать изолированную HEX-математику и одинаковые Go/TypeScript fixtures без подключения к игре.**

Предлагаемые новые файлы: `beckend/gameservice/internal/hexgrid/coord.go`, `coord_test.go`, `src/utils/hexGrid.ts`, `docs/hex-migration/fixtures/geometry-v1.json`; тестовый TS runner — предлагаемый `scripts/test_hex_geometry.mjs`, использующий уже установленный TypeScript/Node по образцу существующих mixer tests. Fixture входит в пакет будущего этапа, сейчас не создан.

Результат: axial/cube conversion, шесть neighbors, distance, projection/inverse, cube rounding с одинаковыми negative/tie semantics, ring/disk/domain predicate, canonical edge key. Стены/бой/API пока не подключать. Уточнить лимиты integer domain и отсутствие -0 в wire.

Проверки: общие expected fixtures для Go/TS; все шесть шагов distance=1 и одинаковой world длины; opposite identity; negative coords; origin; round-trip centres/near-edge/vertices; ring=6k/disk=1+3k(k+1); invalid/out-of-domain. При наличии Go выполнить package test, Node runner — без записи игровых артефактов. Готовность: одинаковые результаты двух языков; существующие square пути не импортируют модуль; diff не затрагивает runtime, БД, GLB или Docker.

Этот этап можно полностью реализовать и проверить независимо от выбора профилей генерации, серверного route endpoint и изменения рендера.

## Этап 1 — версии данных и чтение старых матчей

Зависит от 0. Существующие файлы: `beckend/gameservice/models/match.go`, `models/player.go`, `game/mapgen.go`, `repository/db.go`, `repository/match_map_cells.go`, `repository/matches.go`, `repository/match_monsters.go`, `handlers/match.go`; `src/types/GameTypes.ts`, `types/ApiTypes.ts`, `types/combat.ts`, `features/game/GameWrapper.tsx`, `hooks/useGameSocket.ts`, `features/game/createWsHandlers.ts`, `store/slices/gameSlice.ts`.

Результат: discriminated square-v1/hex-v1 DTO, schema version, geometry metadata, map/state revision, явный typed position patch; legacy decoder. Новая геометрия пока не создаётся. Предлагаемая аддитивная БД: metadata матча, nullable q/r для всех позиций и клеток, edge table, версия JSON start/portal; согласовать один канонический набор полей для HEX и unique constraints. Legacy x/y оставить до завершения square. DB schema changes — отдельная будущая контролируемая операция, не запуск приложения с неожиданными миграциями.

Проверки: копия legacy `matches.map`, нормализованный match_map_cells, смешанные версии, unknown version rejection, round-trip SQL/JSON всех coordinate-bearing сущностей и combat events; (0,0) не sentinel. Убедиться, что legacy backfill не интерпретирует q/r как x/y и не срабатывает для HEX. Готовность: старый square матч читается одинаково; неполная HEX-строка не активируется; HEX creation выключено; restore версии работает.

## Этап 2 — сериализация команд и надёжное восстановление

Зависит от 1. Файлы: `handlers/locks.go`, `combat.go`, `turn.go`, `ws_handler.go`, `finish_match_handler.go`, `barrel_http.go`, `resource_collection.go`, `blueprint_building.go`, `scrolls.go`, `artifact_effects.go`; `game/state.go`, `game/match.go`, `repository/db.go`, `repository/matches.go`, `repository/match_map_cells.go`, `repository/match_monsters.go`, `service/match_service.go`.

Результат: одна очередь/координатор mutations матча, turn token/deadline/revision; commandId idempotency; атомарное обновление позиции/занятости/энергии; publish после commit с упорядоченным diff. Нельзя просто вложить match lock в существующие player locks: определить порядок и убрать nested callback deadlocks. Timeout, disconnect death и финал используют тот же координатор. Если нужен rollout на несколько backend instances, выбрать DB CAS/row locks или single owner; process mutex недостаточен.

Для HEX сохранить необходимые turn-local effects/counters (Boots usage, aura pressure, armor break, sequence/dedup); восстановить deadline и не повторять regen/procs. Исполнение route после рестарта не возобновлять. Повтор финала/награды защитить idempotency отдельно от маршрута.

Проверки: быстрые повторы одного пользователя; два игрока одновременно; route-step vs turn timeout; push vs death; disconnect vs finish; rollback при отказе DB; reconnect после commit до HTTP response; результат повторного commandId. Готовность: одно списание, одна occupancy, ни одна поздняя команда не меняет завершённый матч, retry возвращает тот же результат. Square smoke tests сохранены.

## Этап 3 — серверная видимость и фильтрация всех выходов

Зависит от 1–2. Файлы: `handlers/match.go` (`BuildMatchResponse`, `GetMatchHandler`), `ws_handler.go` (`JOIN_MATCH`, `Broadcast`), `combat.go`, `barrel.go`, `resource_collection.go`, `scrolls.go`, `finish_match_handler.go`; `features/game/GameWrapper.tsx`, `createWsHandlers.ts`, `store/slices/gameSlice.ts`, `hooks/useGameSocket.ts`, `utils/fogOfWar.ts`, `utils/realMapAdapter.ts`.

Результат: membership check GET/JOIN; viewer/team projection, FOV и explored memory; hidden-object removals; все mutation responses и WS/log/quest/drop events проходят recipient policy. Предлагаемый новый Go-модуль visibility следует разместить рядом с будущими HEX predicates, его путь выбрать в этом change set. Ограничить внешнее `attacker_type` игроком. Сохранить явно разрешённую разведку свитками, не передавая весь target object.

Для временного square использовать отдельную square visibility policy; не ломать legacy decoder. Новую строгую HEX-видимость не включать частично только в 3D.

Проверки: два внутренних мира с разными невидимыми монстрами дают одинаковый viewer snapshot, preview и ошибки до разрешённого раскрытия; patch удаления при выходе из FOV; shared group и FFA=0; чужой instance GET/JOIN; hidden combat snapshot/log/HTTP response; scroll disclosure ровно одной оси. Готовность: тайная информация отсутствует в network payload/Redux, а не только DOM. Этот этап — блокирующий gate перед публичным HEX preview.

## Этап 4 — граф HEX и воспроизводимый генератор

Зависит от 0–3. Файлы: `game/mapgen.go`, `game/mapgen_test.go`, `handlers/match.go`, `repository/match_map_cells.go`, `repository/match_monsters.go`; предлагаемые новые `game/hex_mapgen.go`, `game/hex_mapgen_test.go`. Legacy generator оставить для старых fixtures/compatibility до финального отключения новых square матчей.

Результат: finite axial domain, floors/rock, canonical edges, комнаты/коридоры/петли/ниши, локальный seeded PRNG, generatorVersion/config; profile 1–2 игроков и отдельные утверждённые multiplayer profiles. Сохранённая карта — авторитетный материализованный результат, seed — воспроизводимость initial generation.

Проверки: 1000 seed/profile, connected playable graph, start/portal/object occupancy uniqueness, MST/corridor widths, limits retries, graph spawn separation, fairness, доступная бочка квеста; десять seed повторить для identical byte-normalized generation. Сравнить 130–140 floor для малых карт с graph distance/темпом square, не только их количеством. Готовность: валидатор не принимает запертый objective; failed seed воспроизводим; экспериментальные HEX матчи создаются только в тестовом окружении.

## Этап 5 — HEX-правила движения, боя, построек и эвакуации

Зависит от 4. Файлы: `handlers/combat.go`, `combat_test.go`, `turn.go`, `turn_test.go`, `blueprint_building.go`, `scrolls.go`, `resource_collection.go`, `barrel_http.go`, `barrel.go`, `finish_match_handler.go`; `game/match.go`, `game/state.go`; `repository/utils.go`, `match_monsters.go`, `matches.go`.

Результат: общие CanTraverse/CanAttack/CanSee/CanInteract/StepCost; HEX distance и edge LOS во всех диапазонах. Удалённый сбор/бочка/финал запрещены сервером. Определить on-cell interaction policy (рекомендуется тот же HEX), extraction — живой active actor на портале с quest artifact. Турели используют HEX range/LOS; Guardian aura отдельно сохраняет утверждённую wall policy. Ranger push — ровно один сосед, корректные fallback/refund. Сохранить formulas/proc order/energy costs классов и current counter semantics.

Монстры остаются со своими существующими реакциями/контратаками; AI navigation не добавлять под видом миграции. Если AI будет отдельной задачей, он использует те же graph/LOS predicates и серверный поиск пути, deterministic target selection, активируется по отдельному acceptance scope.

Проверки: шесть симметричных направлений, melee через edge wall, Ranger D2 и Mystic D3, corner/vertex LOS ties, blocked push, collision/cell ID, guardian aura/exit, все critical/armor/drain/fury/shield tests; turret no friendly fire; resource/barrel remote denial; portal/auth/dead/wrong-turn denial. Готовность: PvE/PvP server rules независимы от renderer, square branches не читают axial.

## Этап 6 — серверная команда маршрута

Зависит от 5. Файлы: `cmd/main.go`, `handlers/combat.go`, `handlers/turn.go`, `handlers/locks.go`, `repository/matches.go`, `repository/match_map_cells.go`; предлагаемые `handlers/route.go`, `handlers/route_test.go`.

Результат: endpoint из HEX_DESIGN, bounded path, commandId/turnToken/revision/origin/maxEnergyCost; точная server repricing по шагам; atomic step commits; stop-on-reveal/change/deadline/damage/death/block. Route никогда не превращается в атаку/сбор/эвакуацию, не переносится на другой ход и не возобновляется после reconnect. Проверить deadline между шагами независимо от занятости очереди.

Проверки: изменившаяся стена/occupancy; повтор/timeout HTTP; длина/петли/teleport/overflow; энергия на границе; Boots первый шаг; Titan; aura pressure и летальный выход; stopped prefix; rollback одного шага; finish посередине. Готовность: outcome содержит только committed segments и допустимые viewer данные; никакой клиентской автоматической цепочки `/move`.

## Этап 7 — единый preview/confirm UX и Classic HEX

Зависит от 6. Файлы: `GameController.tsx`, `hooks/usePlayerActions.ts`, `hooks/useGameKeyboard.ts`, `Controls.tsx`, `MapWithCamera.tsx`, `Map.tsx`, `MapCell.tsx`, `MiniMap.tsx`, `ObjectiveTracker.tsx`; `utils/fogOfWar.ts`, `buildPlayerMap.ts`, `getPlayerInCell.ts`, `hooks/usePlayerInCell.ts`, `styles/Map.module.css`, `MiniMap.module.css`, `Controls.module.css`, `GameController.module.css`; предлагаемый `hooks/useTacticalSelection.ts`.

Результат: один interaction controller; Dijkstra reachable, A* preview на разрешённом snapshot; state-aware costs; два отдельных выбора, понятный префикс/остаток, доступные враги/линия атаки, причины отказа без утечек. HEX polygons/layout/picking, shared q/r transforms для спрайтов и effects, minimap focus/ping, objective distance. Queue подтверждённых segments, отдельно display и authoritative positions. Шесть направлений с одним keyboard listener; touch, отмена, focus/input guards.

Проверки: первый клик → ноль mutations; второй тот же target → одна; смена цели/actor ID, turn/view changes отменяют; right-click/Escape; pointer drag; mobile tap; prefix beyond energy; hidden cells; PlayerLayer не режет стену; minimap не отправляет движение. Готовность: одинаковая машина работает без 3D, все старые immediate-click обходы удалены для HEX.

## Этап 8 — 3D HEX и читаемость

Зависит от 7. Файлы: `LiveMatchScene3D.tsx`, `NachaloScene3D.tsx`, `wallOcclusion.ts`, `columnPlacement.ts`, `catacombsAtmosphere.ts`, `AnimatedRanger.ts`, `CombatOverlay3D.ts`, `hooks/useCombatPresentationPlayback.ts`, `utils/realMapAdapter.ts`, `src/app/graphics-lab/page.tsx`; предложить отдельные новые HEX fixtures, не переименовывая square как HEX.

Результат: HEX floor и edge meshes; единые world transforms для камеры, объектов, света/декора/анимаций/эффектов; sparse viewer updates вместо первой полной terrainRef; picking по ID/inverse; outlines, edge boundaries, route, marker героя. Потом cutaway/силуэты/zoom/rotation отдельными небольшими изменениями. GLB и Blender не требуют изменения; существующие animated/legacy модели остаются.

Проверки: world/pixel parity с Classic, шесть направлений без смены скорости, all object anchors/projectile/hit/death/push, occlusion только visible targets, floors/edges совпадают с server validator; camera rotation/zoom/picking, режимы модели, загрузка/reconnect/defeat. Готовность: две сцены исполняют одни команды и рисуют одинаковые состояния overlay, 3D не требует скрытых server cells.

## Этап 9 — настоящие PvE/PvP, performance и controlled rollout

Зависит от 8 и утверждения игровых параметров. Файлы: `scripts/check_ranger_walk_pve.mjs`, `check_ranger_live_browser.mjs`, `check_ranger_live_combat.mjs`, `check_ranger_combat_reconnect.mjs`, `test_ranger_lab.mjs`, `test_ranger_combat.mjs`, backend test files; `beckend/matchmaking/Match_Making_test.go`; документация режима/выпуска в этом каталоге.

Результат: сценарии из RISKS_AND_TESTS в изолированной БД/на тестовых аккаунтах, metrics report и release gate; адаптация четырехнаправленных тестов к versioned geometry. Настоящий PvP включает независимые browser contexts/пользователей, а не fixture с двумя спрайтами. Сначала rollout на ограниченный профиль HEX, затем команды/FFA и максимальные разрешённые режимы. Зафиксировать release version/checksum клиента, протокол и профили.

Готовность: regression green, server race/integration pass, network secrecy checks pass, manual 3D usability pass, GPU budgets измерены; незавершённые square матчи продолжают работать/восстанавливаться. При проблеме запретить создание новых HEX и сохранить уже созданные для compatible backend; не откатывать на старый binary, не понимающий новые записи.

## Этап 10 — завершение перехода

Зависит от 9. Файлы: `handlers/match.go`, `game/mapgen.go`, все временные square/HEX dispatch points в backend/frontend, `src/types`, versioned fixtures/scripts и release docs.

Результат: новые матчи создаются только HEX во всех разрешённых режимах. Перед удалением runtime square ветвей подтвердить read-only запросом отсутствие активных legacy matches, старых jobs и необходимого rollback retention. Историю, профили, экипировку, inventory snapshots сохранять. Если square матчи долгоживущие, оставить минимальный versioned legacy executor до их естественного окончания; не назначать скрытое принудительное завершение.

Проверки: полный поиск четырёх направлений/Manhattan/row-major/hardcoded 15/old event координат; каждый остаток имеет обоснованное назначение (legacy fixture, world metric, UI grid). Документация не утверждает, что любые Math.abs/BoxGeometry запрещены: некоторые остаются художественными/численными операциями. Готовность: ни один активный runtime HEX путь не использует square adjacency или square coordinates.

## Безопасное хранение и переход данных

1. До будущих DB изменений — резервная копия и проверка восстановления на копии; в этом задании ничего не выполнялось.
2. Аддитивные поля/таблицы и совместимый reader сначала; existing rows explicitly square-v1, без пересчёта x/y в q/r.
3. Проверка backfill по версии: старый `matches.map` переносится как square. Неполная нормализованная карта не должна считаться валидной лишь по COUNT>0; потребуется проверка completeness/revision.
4. Не конвертировать активную square карту арифметически в HEX: меняются neighbors/LOS/пути/баланс. Дать завершиться старому матчу. Обсуждаемая альтернатива — отдельная добровольная конверсия на копии с полной валидацией; она не входит в базовый план.
5. Сохранять геометрию/revision в snapshot/events/cache keys и истории там, где сохраняются координаты. История итогов не является replay. Старые exported fixtures/action logs/session cache остаются legacy и не загружаются в HEX.
6. Rollback отключает creation flag; данные HEX сохраняются для поддерживающей их версии. Очистка volumes/истории/профилей не предусмотрена.

## Решения для согласования с владельцем

| Решение | Предложение | Что блокирует |
|---|---|---|
| Ориентация/масштаб | Axial pointy, a=1/√3 | Утверждение geometry fixtures |
| Профили размера и fairness | Малый R8, floor130–140; 4 players R13; остальные по budget | Окончательную генерацию и matchmaking rollout |
| Экономика | Сохранить shared energy, без нового AP/step cap | Reachable UX и баланс |
| Видимость и магия | Server HEX LOS, консервативные ties; aura through-wall policy отдельно | CanAttack/FOV |
| Маршрут при открытии fog | Остановка и повторный выбор | Route executor/UX |
| Сбор и бочки | Отдельное подтверждение на своей клетке | Прекращение move→auto collect/open |
| Опасное движение | Safe reachable исключает известную смерть, явная отдельная команда при разрешении | State-aware path cost |
| Разведка/глобальные события | Одно q/r значение свитка; скрытые координаты в объявлениях не передавать | Recipient projection |
| Клавиатура/камера | Шесть клавиш, camera-relative labels; zoom/rotate после core HEX | Usability acceptance |
| Активные legacy матчи | Естественное завершение с versioned executor | Финальное удаление square runtime |

Согласование этих параметров не требуется для выполнения текущего аудита и независимого этапа 0. Приведённые предложения остаются явными допущениями, а не принятыми владельцем решениями.

## Задачи для Codex

Каждый пункт ниже — самостоятельное последующее поручение; не выполнять их автоматически в рамках аудита.

1. **Изолированная математика (этап 0).** Создать перечисленные новые Go/TS модули и общие fixtures, не импортировать их в runtime. Результат: шесть направлений/distance/projection/rounding/domain/edges/rings. Проверки: одинаковые negative/tie результаты, Go package tests и Node runner; игровой код/БД не меняются.
2. **Versioned data contract (этап 1).** Добавить discriminated DTO и аддитивный schema plan в перечисленных models/repository/frontend файлах, HEX creation оставить выключенным. Результат: legacy square читается явно, неизвестные версии отвергаются. Проверки: SQL/JSON round-trip на изолированной копии, backfill/version gates, origin=(0,0).
3. **Сериализация и recovery (этап 2).** Включить все mutations/timer/death/finalize в один match coordinator, обеспечить atomic step и idempotency. Результат: устойчивые revision/turn token/command outcomes. Проверки: конкурентные команды, failpoints DB, restart/HTTP timeout, отсутствие double spend/reward.
4. **Server visibility (этап 3).** Заменить выдачу полного клиентского мира recipient projection, закрыть GET/JOIN membership и внешние monster attacks. Результат: hidden entities отсутствуют в network/Redux/logs. Проверки: paired-world secrecy, team/FFA, mutations/WS/reconnect/scroll scope.
5. **Seeded HEX catacombs (этап 4).** Реализовать finite axial graph, комнаты/коридоры/edges/spawns/portal/loot по утверждённым профилям. Результат: материализованная воспроизводимая карта и validator. Проверки: 1000 seed/profile, connectivity/fairness/quest-barrel, bounded retry.
6. **HEX game rules (этап 5).** Перевести movement/combat/structures/turrets/scrolls/interactions/extraction на общие predicates с LOS. Результат: истинное соседство и server-side пространственная проверка. Проверки: шесть направлений, все class effects, forbidden wall attacks, remote interaction denial, PvE/PvP handlers.
7. **Route endpoint (этап 6).** Добавить отдельную команду bounded path с per-step commit/stop policy и commandId. Результат: только подтверждённые steps без implicit attack/loot/next turn. Проверки: hidden block, energy modifiers, deadline, damage/death, repeated ID/reconnect и rollback.
8. **Classic и общий двухкликовый UX (этап 7).** Подключить selection controller, reachable/route/attack previews, HEX Classic/minimap/спрайты и 6-key input. Результат: первый клик без mutations, второй отправляет ровно одну команду. Проверки: mouse/touch/cancel/stale/режимы/hidden/route prefix, confirmed animation segments.
9. **Three.js HEX (этап 8).** Перевести floors/walls/picking/camera anchors/декор/Ranger/effects на общую проекцию, затем readability overlays/cutaway. Результат: parity с Classic без изменения GLB/Blender. Проверки: все 6 направлений, occlusion secrecy, projectile/death/push/reconnect, model fallback, GPU smoke.
10. **Регрессия и выпуск (этап 9).** Адаптировать указанные tests/scripts, провести реальные PvE/PvP и performance на изолированных данных. Результат: воспроизводимый report и gated rollout. Проверки: unit/integration/race/E2E/network secrecy/low GPU, legacy continuation и безопасный rollback.
11. **Завершение миграции (этап 10).** Отключить создание square во всех режимах; убрать runtime legacy только после подтверждения отсутствия активных legacy матчей и выполнения retention policy. Результат: все новые игры HEX, история/инвентарь сохранены. Проверки: весь matrix, restore, полный residual-pattern review без потери пользовательских данных.
