# NACHALO: риски и проверка полной HEX-миграции

Основание: чтение исходников ревизии `9a462ad`, 18 октября 2026; [аудит](ARCHITECTURE_AUDIT.md), [дизайн](HEX_DESIGN.md), [план](MIGRATION_PLAN.md). Это план будущих проверок, кроме явно указанного раздела «Что проверено сейчас».

## 1. Реестр рисков

| Приоритет / риск | Доказательство в текущем коде | Последствие | Контроль / release gate |
|---|---|---|---|
| P0 — тайные данные на клиенте | `BuildMatchResponse` выдаёт fullMap/players; `Broadcast` фильтрует instance, не viewer | Подсветка/pathfinding/network раскрывают врагов/объекты | Recipient projection GET/JOIN/WS/HTTP/logs, paired-world тесты до включения HEX |
| P0 — доступ к чужому матчу/monster command | JOIN принимает instance от клиента; `UniversalAttackHandler` проверяет player auth внутри player ветки | Просмотр чужого мира / команда от monster | Проверять membership до подписки/GET, внешние attacker types allowlist, negative tests |
| P0 — mixed geometry | `x/y`, `matches.map`, start/portal arrays без версии | Square прочитана как HEX, потеря позиций | Versioned reader, explicit legacy, additive schema, unknown fail closed |
| P0 — race / частичный шаг | `lockPlayer` per-user; position и flags пишутся отдельно; таймер/смерть/финал разные входы | Double spend, unit overlap, движение после конца хода | Match coordinator + DB transaction/CAS, timer through same order, failpoints/race tests |
| P0 — implicit attack маршрута | `/move` до neighbor check переводит occupied destination в universal attack | Маршрут атакует без второго подтверждения | Отдельный route intent без перенаправления в attack; занятость останавливает |
| P0 — remote interactions / extraction | В `CollectResourceHandler`, `OpenBarrelHandler`, `FinishMatchHandler` нет проверки нужной позиции | Loot/победа вне клетки | CanInteract/на портале на сервере, auth/active/alive/geometry, direct API negative cases |
| P1 — атака через стены | `resolveAttackMode` проверяет Manhattan range, LOS нет | Renderer показывает запрет, API разрешает | Единый CanAttack/edge+cell LOS; melee/Range/Mystic/turret matrix |
| P1 — раскрытие через ошибки/cost | Preview на полном Redux; Guardian зависит от скрытых игроков | Enemy oracle без визуализации | Только knowledge graph; generic stop reasons; никаких hidden preview запросов |
| P1 — неэквивалентная цена пути | Boots первый шаг, аура pressure/exit damage, Titan | Недостаток энергии/смерть вопреки preview | Stateful shortest path или явно ограниченный контракт; точный server repricing |
| P1 — неверный push | sign(dx),sign(dy) в Ranger third-shot | Несоседний HEX, проход через edge wall | Шесть нормализованных world dirs, tie policy, collision/edge tests |
| P1 — restart теряет механику | `RestoreMatchStates` восстанавливает только turn fields | Повтор Boots/proc, потеря armor/pressure/stats, sequence reuse | Persist необходимых transient counters/deadline, restart tests, revision+epoch dedup |
| P1 — неверная карта/квест | Генератор random occupancy, нет min spawn distance; квест из бочки | Игрок заперт/не может добыть артефакт | Graph validator, доступные бочки/портал, seed sweep и bounded retries |
| P1 — поздние/дубли WS | HTTP+WS, snake/camel normalizers, combat sequence in memory | Старый snapshot возвращает игрока/мертвого монстра | Monotonic stateRevision, command/exchange dedup, reconnect snapshot, old event rejection |
| P1 — stale terrain | `LiveMatchScene3D.terrainRef` хранит первую карту | Открытая дверь/новая стена не совпадают с сервером | Versioned topology patches и scene cache invalidation |
| P1 — UI расходится с сервером | Две Manhattan click проверки и старые hooks/controls | Однокликовый обход/несогласованное движение | Единый interaction controller для обеих сцен и input |
| P1 — недостаточный rollback | Старый backend не знает HEX persisted state | Матчи зависают после отката | Rollback creation flag; compatible binary обслуживает existing HEX |
| P2 — visual geometry/picking | Row-major, box floors, masks4, Math.round(world) | Неверная цель, щели/проходы | Golden fixtures projection/picking/edge, Classic↔3D parity |
| P2 — 6 neighbors меняют баланс | Diamond radius2=13, HEX disk2=19 | Больше целей, иные chokepoints/ауры | Сохранить численные costs сначала, сравнить реальные match metrics |
| P2 — sprite/effect drift | Raw cell x/y используются как world/pixels | Projectile/death/push не на цели | Общая проекция snapshots и очередь confirmed segments |
| P2 — слабые GPU | Множество meshes/shadows/материалов/occlusion rays | Падение FPS/input responsiveness | Instancing/batching/culling, budget/quality fallback, low GPU tests |

P0 — блокирует публичное включение HEX. Наличие таблицы не означает, что перечисленные проблемы уже исправлены. Некоторые изменения (LOS и расположение взаимодействий) закрывают найденные недостатки square кода, поэтому их нужно описывать в release notes как изменение правил.

## 2. Сохранения, совместимость и пользовательские данные

Постоянные runtime данные: `matches`, `match_map_cells`, `match_players`, `match_monsters`, inventory/artifact/structure state. Старые карты могут оставаться в JSON `matches.map`, start/portal — JSON массивы; monster JSON содержит ID, которым нельзя подменять `db_instance_id`. История `match_stats`/`match_player_stats`, награды, character XP, inventory snapshots — отдельные носители, сохраняемые `FinalizeMatch`. Runtime cleanup в `DeleteMatch` — существующая логика завершения, не предлагаемая очистка для миграции.

Память процесса: `game.MatchStates` с боевыми maps/events/sequence, `artifactTurnMoves`, turn/reconnect timers. Сегодня рестарт не равен replay: `RestoreMatchStates` не восстанавливает весь этот набор. Не обещать сохранение боевых effects/статистики только на основании наличия PostgreSQL.

Клиентские остатки: exploration indices в sessionStorage (`fog-explored:...:WxH`), queue/snapshots в Redux, квестовые уведомления x/y, последние итоги матча, fixture JSON, отчёты scripts. Для HEX cache key обязан содержать geometry/version/mapRevision/instance/viewer-or-team. Старый key не переносится арифметически. Новая сцена не должна использовать прежний `terrainRef` после смены instance/revision.

Перед будущими миграциями: восстановимая резервная копия → rehearsal на отдельной копии → validation counts/constraints → совместимые readers → additive schema → creation flag. Проверять количество legacy/HEX матчей и координатных строк по версии, не делать массовый UPDATE x→q, y→r. History/intventory/accounts/Docker volumes не удалять. Не выполнять restart тесты на пользовательском матче без отдельного задания на тестовое окружение.

Матч не имеет доказанного короткого TTL: disconnect grace=3 минуты — не максимальная длительность игры. Поэтому rollout должен учитывать долгоживущие persisted matches. Legacy executor удаляется после подтверждения отсутствия таких матчей, а не после произвольно выбранной даты.

## 3. Unit-тесты: геометрия, правила и генератор

| Область | Входы / ожидаемый результат | Где реализовать в будущем |
|---|---|---|
| Axial/cube | Все 6 dirs; opposite; D symmetry/triangle; x+y+z=0; отрицательные q/r | Предлагаемый `internal/hexgrid/coord_test.go`, общий fixture и TS runner этапа 0 |
| Проекция/округление | Центры, случайные дробные координаты, отрицательные halves, edge/vertex ties, origin, signed zero; Go=TS | Те же common fixtures; отдельно inverse camera transform |
| Domain/rings | R=0,1,7,8,12,13; 6k и 1+3k(k+1); sparse holes; точки за границей | Geometry tests |
| Edge symmetry | A→B = B→A, opposite, canonical key, invalid non-neighbor edge, closed/open door | Будущий graph unit package |
| LOS | Прямая во всех направлениях, дальняя непараллельная линия, wall/rock, endpoint wall, вершины/ребро, A↔B symmetry, no corner shooting | Graph/CanSee tests; TS fixtures для overlay |
| Dijkstra/A* | Пустой граф path length=D; detour; unreachable; weighted longer-cheaper path; deterministic tie; A*=Dijkstra optimum | Предлагаемый navigation package/TS tests |
| Stateful cost | Boots first step already used/unused; Titan +2; overlapping guardian nonstack; aura entry/exit pressure and lethal exit | `handlers/combat_test.go`, `artifact_effects_test.go`, navigation tests |
| Input validation | Fraction/NaN/Inf в TS; JSON decimal/string/null/missing; huge integers/overflow; отрицательные допустимые; bounded path/loop/teleport | DTO/route handler tests |
| Combat | Ranger D2/Mystic D3, adjacency D1 all6, overlap invalid, energy6/8/artifact; team/self/dead/wrong-turn/hidden | `handlers/combat_test.go` |
| Class effects | Existing critical, armorBreak/reset/refund/push/fallback, shield/counter, fury/bloodFeast, drain/overburn/kill | Сохранить существующие tests, добавить HEX positions/edge fixtures |
| Push | Шесть направлений, tie determinism, occupied destination/rock/closed edge/boundary, monster/player flags, no aura forced-exit damage | `combat_test.go`, `monster_death_test.go` |
| Постройки | Place D1, не D2; floor/portal/spawn occupancy; wall structure отдельно от edge wall; scout sight | `handlers/turn_test.go`, building tests будущего этапа |
| Генерация | 1000 seed/profile, connected floors, graph-separated spawns, portal/quest-barrel accessibility, densities/limits | `game/hex_mapgen_test.go` (предлагается), существующий `mapgen_test.go` оставить legacy |
| Смерть/дроп | ID жертвы/монстра сохранён, tile освобождается, артефакт/экипировка появляются в правильном HEX, один reward | `monster_death_test.go`, `equipment_drops_test.go`, `game/results_test.go` |

Не заменять существующий `TestGenerateFullMap_BasicProperties` ожиданием HEX=15×15: оставить явно square-вариант, добавить отдельный HEX. Существующий `TestGenerateFullMap_AllObjectsReachPortal` проверяет объекты, но не все floor, не min spawn separation и не edge walls — нужны новые свойства.

## 4. Интеграция: backend и PostgreSQL

Только изолированная тестовая БД с явно назначенными данными. В репозитории integration helpers читают `GAME_DB_DSN` и вызывают `repository.RunMigrations`; запускать их против существующей БД пользователя запрещено в текущем задании. Не считать имя «test» гарантией read-only.

Обязательные сценарии:

1. Создать square из legacy JSON и normalized rows; сериализовать/перезапустить/read; geometry остаётся square. Создать HEX с negative q/r, edges и IDs; round-trip всех клеток, players, monsters, starts, portal, drops, structures.
2. Route step: position, energy, occupancy, first-step/aura counters, revision и outbox либо committed вместе, либо не меняются. Failpoint между каждой DB записью; клиент не получает событие о rolled-back шаге.
3. Два участника одновременно отправляют move/attack; ещё запрос у timer. Только активный turn token допускается. Старый turnToken с тем же TurnNumber отклоняется.
4. Два маршрута в одну клетку, swap, push во время маршрута, структурный block, death/grace expiration: конечная occupancy однозначна; второй actor получает безопасный отказ/остановку. Никакой неявной атаки.
5. End turn вручную и таймером одновременно: construction/turret/regen применяются ровно один раз. Таймер не откладывается на неограниченную длину маршрута.
6. HTTP timeout после commit, повтор commandId, повтор другого body с тем же ID, replay после рестарта: нет второго списания; outcome доступен, route не запускается повторно.
7. Reconnect snapshot после потери WS, out-of-order/duplicate patches и combat exchanges: revision монотонна, snapshot не откатывается старым HTTP; нет двойных анимаций/floaters/regen/procs.
8. Restart во время незавершённого route и после aura/armor/Boots: восстановлены committed position/state/deadline, сохранена геометрия, остаток отменён. Старый exchange ID не сталкивается с новым sequence.
9. Finish при quest missing / wrong cell / dead / wrong turn / wrong instance запрещён; допустимый финал один раз начисляет награды и XP, сохраняет history, удаляет только runtime по штатной логике. Повтор finish не дублирует награды.
10. GET/JOIN и mutations от постороннего пользователя не дают viewer state; запрос с `attacker_type=monster`, чужой attacker/target ID, скрытой целью или несоответствующим instance отвергается.

Будущие команды проверок: `go test ./game ./internal/... ./handlers`, `go test -race ./...` и integration suite только в изолированной БД. На Windows race detector может требовать поддерживаемый toolchain/C compiler; при отсутствии выполнять в существующем подходящем CI, не устанавливать инструменты самовольно. Каждый отчёт обязан перечислять skipped suites.

## 5. Проверка отсутствия утечек

Построить два server state, отличающиеся только скрытым монстром/игроком/ресурсом/закрытой дверью. При одинаковом knowledge state игрока сравнить нормализованные GET snapshot, JOIN snapshot, WS patches, HTTP outcomes preview/denial, Redux, action logs, minimap и overlay state. До события разрешённой разведки ответы должны быть одинаковыми; random IDs/время сравнивать с исключением лишь технических полей.

Проверять не только terrain: `tileCode=M/R/B`, `monster.id/db_instance_id`, `isPlayer`, cell count/holes, structure owner/health, combat snapshots, projectile source/target, death/drop координаты и quest announcements тоже могут выдать сведения. Передавать известное статическое terrain отдельно от актуальной скрытой динамики. Даже сообщение «занято монстром» на неизвестной клетке — утечка.

Серверная preview API не нужна при первом клике. Клиентский поиск не должен читать full internal grid, даже если его объекты не рисуются. Доступный graph явно строится из viewer state. Для explored клеток старое знание не даёт права показывать текущую occupancy/edge revision. Силуэт врага и depth-disabled route не обходят fog.

Исключения вроде разведсвитка или публичного объявления артефакта документируются отдельной политикой. Свиток раскрывает ровно разрешённую ось q/r; имя/ID/вторая ось/тип цели не добавляются неявно. Preview hidden-dependent energy — оценка/unknown, а не точное значение из невидимого Guardian.

## 6. E2E Classic и 3D

Нет доказанного общего Jest/Playwright runner в package.json: там dev/build/start/lint, наличие `@types/jest` не означает работающие Jest tests. Существующие browser scripts — сценарные инструменты, часть использует дополнительные browser-инструменты и реальные учётные данные; изучить prerequisites перед будущим запуском. Не добавлять зависимости в текущей работе.

| Сценарий | Classic | 3D | Наблюдаемая проверка |
|---|---|---|---|
| Первый/второй клик | Polygon/select target | Raycast actor/floor | Первый не даёт mutations; второй ровно одна команда, не два click+dblclick |
| Отмена/смена | Escape/RMB/другая клетка | Те же события + drag camera | Ни один отменённый path не отправлен |
| Target identity | Enemy ID/position | Sprite/mesh ID | Новый actor на том же HEX требует нового выбора |
| Стоимость/бюджет | Steps/energy/prefix | Те же значения/overlay | Доступный префикс соответствует server committed steps |
| Fog/stale | Unknown/explored/visible | Те же server view IDs | Скрытый target не clickable/attackable; новый view сбрасывает preview |
| Стены/LOS | Edge outline/block marker | Wall base/cutaway + outline | Запрет одинаков на UI и server; no vertex gaps |
| Анимация маршрута | PlayerLayer segments | Ranger world segments | Повороты по центрам, нет прямой через стены |
| Combat | Sprite hit/projectile/death | GLB animated/legacy + overlay | Release timing, 6 facings, death freeze, push, no duplicate effects |
| Переключение режима | Выбор/preview до переключения | После переключения | Один controller/WS, актуальный preview, нет повторной mutation |
| Камера/viewport | Resize/scale/центр | DPR/zoom/rotate/follow/picking | Сохраняется правильный HEX под курсором |
| Minimap | Фокус/пинг/известные клетки | Общий minimap | Клик не двигает игрока, camera footprint верен |
| Keyboard/touch | 6 directions/Enter/tap | Тот же controller | Нет двойного listener, ввод текста не двигает героя |
| Reconnect | Snapshot/preview reset | Persistent canvas/model fallback | Route не возобновлён, нет скрытых stale объектов |

Screenshot regression: одинаковые HEX fixtures при известной camera pose, несколько DPR/viewport, пять wall topology cases, граница карты/negative coordinates, видимый/неизвестный passage. Screenshot alone не доказывает корректность server rules.

## 7. Настоящие PvE/PvP и завершение матча

PvE smoke в изолированном окружении: создать настоящего игрока/матч через API, получить server view; разведать коридор, preview/подтвердить маршрут с остановкой, бой с монстром (melee и ranged), third-shot push/fallback, HP/energy changes, бочка/ресурс/квест, выход через портал, history и persistent inventory. Повторить для Guardian, Berserker, Ranger, Mystic и обоих renderer. Не заменять этот сценарий Graphics Lab.

PvP: минимум два независимых browser contexts с разными пользователями; затем 2×2 с союзной видимостью, 3×3/FFA и максимальный разрешённый профиль режима. Проверить порядок команды/игроков matchmaking, одновременные запросы на границе хода, общий HEX, запрет friendly fire, столкновения с монстром, ауры/контратаки/push, hidden opponent, disconnect grace, reconnect, смерть носителя артефакта, передачу/дроп, победу и одинаковый финал для всех клиентов. Публичные координаты исчезнувшего противника не остаются в log/Redux.

Отдельно: рестарт backend с незавершённым реальным тестовым матчем → продолжение того же экземпляра/геометрии/хода без повторной награды; уже завершённый матч → показ сохранённой статистики, route/attack отклоняются. Измерить длительность до первого контакта и до extraction для square baseline и HEX; это необходимо для утверждения размеров/аур/ranges.

## 8. Производительность

Здесь приведены оценки сложности и предлагаемые цели, не результаты измерения будущего HEX-кода. Пусть V — клетки графа, E<=3V внутренних неориентированных рёбер, P — видимые units, L — длина LOS.

| Операция | Оценка | Практическая мера |
|---|---|---|
| Генерация | Carve/validation O(V+E); MST зависит от числа комнат | Bounded retries, local PRNG, seed benchmark всех профилей |
| A*/Dijkstra | O((V+E)log V), stateful cost умножает число состояний | Пересчитывать на view/actor/cost revision, не на каждый RAF; cache reachable одного snapshot |
| FOV/LOS | Наивно O(P·V·L) в домене; radius кандидаты O(R²) | Ограничить радиус, early block, cache static edges, recompute при движении/edge change |
| HEX под курсором | Inverse+round O(1), domain lookup в среднем O(1) | Throttle pointer к RAF; для floors plane picking вместо raycast всех meshes |
| Path overlay | O(path length) на смену selection | Один batched line/instance buffer, никаких scene rebuild на hover |
| Пол/стены | Геометрия O(V+E), draw calls зависят от batching | Shared HEX geometry, InstancedMesh/batches по материалу, canonical edges один раз |
| Occlusion | Сейчас ~5P лучей × пересечение wall meshes, до 30 Hz | Ограничить видимые targets/walls, spatial culling, без all-map raycast каждый frame |
| Синхронизация | Full snapshot O(V+units), patches по изменению | Viewer diffs, ordered revision, не слать весь мир после шага |
| Анимация | O(visible actors + active effects) | Не пересоздавать GLB/terrain/materials при route/WS; dispose stale resources |

Не увеличивать tile/wall draw calls в 6/4 раза механически: canonical edges и batching экономят больше, чем форма клетки. Полупрозрачные стены вызывают overdraw и sorting artifacts; сравнить cutaway с transparency. Существующий renderer cap DPR<=2 не гарантирует приемлемую скорость на слабом GPU. Предложить low-quality preset (DPR/shadows/light/decor limits), без урезания правил/видимости.

Предварительные performance gates для согласования: preview на малой карте p95<16ms, на максимальном профиле <50ms на выбранном слабом CPU; server route validation/step p95<50ms без сетевой задержки на целевой нагрузке; main-thread long tasks >50ms не возникают на каждом hover; 3D ≥30 FPS на выбранном слабом GPU при обычном бою, frame p95<33ms; отсутствие монотонного роста памяти после 20 mode switches/reconnect. До выбора эталонных устройств эти цифры — цели, не обещание.

Измерять generation p50/p95/max и fail count на seed sweep, число visits/expanded states/LOS checks, WS bytes/player/step, DB queries/commit latency, renderer.info draw calls/triangles/memory, frame times/GC и GPU overdraw. Нужны раздельные результаты для малой и самой большой карты, 1/2/4/максимума игроков, Classic/3D, animated/legacy Ranger. HTML build/tsc не измеряют эти свойства.

## 9. Что проверено сейчас

- Начальный `git status --short` был пуст; HEAD=`9a462ad`. Рабочие изменения задания — только четыре Markdown-файла `docs/hex-migration/`.
- Прочитаны реальные mapgen/handlers/repository/contracts/Redux/Classic/3D/input/effects и имеющиеся тестовые сценарии. Выполнен поиск прямых `±1`, N/E/S/W, row-major, Manhattan/Euclidean/Chebyshev формул, tile literals, `Math.round` world позиций и старых JSON координат. Реестр с местами и заменами находится в ARCHITECTURE_AUDIT.
- Подсчитаны cell codes трёх существующих fixtures: playable-shape counts 132/136/384, всего225/225/576. Это статическая база, не выборка runtime матчей.
- Независимым Python вычислением без записи игровых файлов проверены 1681 центр projection/inverse, шесть равнодлинных neighbors и формулы rings/disks радиусов1–10. Дополнительная проверка rounding/path invariants фиксируется после финального запуска ниже.
- Выполнен локальный TypeScript checker: `node_modules/.bin/tsc.cmd --noEmit --incremental false` — успешно. Incremental cache не изменялся. Это baseline типизации текущего square кода, не проверка будущей HEX-реализации.
- Go отсутствует в текущем PATH (`Get-Command go` не нашёл executable). Go unit/race/integration tests **не выполнены**; toolchain не устанавливался.
- DB, Docker, миграции, сетевые игровые команды и live PvE/PvP не запускались. Integration helpers способны вызвать миграции; browser scripts способны изменить матчи. Existing mixer scripts пишут validation artifacts, поэтому также не запускались в этой документационной задаче.

Будущий отчёт о готовности должен явно разделять unit, integration, E2E fixtures, реальные тестовые матчи и performance; ни один пункт не заменяет остальные.
