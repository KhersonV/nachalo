# Этап 2: промежуточный результат

Этап **не завершён**. Реализованы транзакционные пути обычного квадратного шага, подбора ресурса, ряда нелетальных PvP/PvE атак и соответствующих автоатак, а также клиентский приём версионированного patch. Смерть, дроп, Ranger push, quest artifacts и Guardian aura по-прежнему обходят общий commit. Ниже указаны точные границы.

## Карта изменяющих состояние команд

`Да` в колонке «Tx» означает **единый commit всей указанной команды**, а не наличие локальной транзакции для одной из её записей. В колонке тестов указаны только реально выполненные проверки нового механизма.

| Команда и обработчик | Tx/lock матча | `commandId` | Revisions | Проверка / остаточный риск |
| --- | --- | --- | --- | --- |
| Обычный квадратный шаг: `MoveOrAttackHandler` → `ApplySquareMove`, без Guardian aura | Да / advisory lock, затем `matches FOR UPDATE` | Да | state + map | `TestSquareMoveTransactionIntegration`: параллельные шаги, retry, conflict, rollback, шаг + подбор. Boots marker остаётся в памяти. |
| Нелетальная автоатака поддерживаемой PvP/PvE ветки: `MoveOrAttackHandler` → `tryAtomicNonlethalPvP` / `tryAtomicNonlethalPvE` | Да / один внешний advisory lock, затем `matches FOR UPDATE` | Да | state; map только при изменении HP монстра в клетке | `TestAtomicCombatBranchesPostgres`: реальный `/move` по занятой клетке и replay. Replay autoattack проверяется до обычного шага. |
| Прямые нелетальные PvP всех классов без quest artifacts, кроме Ranger third shot/push: `UniversalAttackHandler` → `tryAtomicNonlethalPvP` | Да / advisory + row lock | Да | state; map без изменений | `TestAtomicCombatBranchesPostgres`: Guardian shield/counter, Mystic drain/overburn, Berserker lifesteal/followup, Ranger crit/armor break, конкуренция, rollback/replay. Классовые ветки имеют дополнительные условия выживания до RNG. |
| Прямые нелетальные PvE всех классов без quest artifacts, кроме Ranger third shot/push: `UniversalAttackHandler` → `tryAtomicNonlethalPvE` | Да / advisory + row lock | Да | state; map только при изменении JSON клетки | `TestAtomicCombatBranchesPostgres`: Guardian counter/response, Mystic monster defense, Berserker lifesteal/reduced counter/followup, Ranger crit/armor break, rollback карты и replay. |
| Смертельные PvP/PvE, Ranger third shot/push, quest artifacts, удар по строению и шаг под Guardian aura | **Нет / advisory lock только сериализует вход** | Поле принимается, но ledger не используется | Нет | `universalAttackLocked` и move helpers делают независимые записи и могут разослать WS до конца операции. |
| Конец хода: `EndTurnHandler`; таймер: `doEndTurn` | **Нет / advisory lock только сериализует вход** | Нет | Нет | DB active/turn проверяются после lock; таймер сравнивает сохранённый номер хода. Стройка, турели, эффекты, turn state и regen пишутся по частям. |
| Подбор ресурса/артефакта: `CollectResourceHandler` → `ApplyResourceCollection` | Да / advisory lock, затем `matches FOR UPDATE` | Да | state + map | Integration: повторный подбор, conflict, отказ inventory, шаг + подбор. Старое правило удалённого подбора сохранено. |
| Бочка: `OpenBarrelHandler` → `HandleOpenBarrel` | **Нет** | Нет | Нет | Могут измениться клетка, inventory, HP и финал отдельно. |
| Постройка: `PlaceBlueprintHandler`; ходовые эффекты строений: `progressConstructionByTurn`, `progressStructuresEffectsByTurn` | **Нет** | Нет | Нет | Старые тесты правил; нет единого commit. |
| Свиток: `UseScrollHandler` | **Нет** | Нет | Нет | Использование предмета и разведка без ledger. |
| Использование consumable: `UseInventoryHandler` | **Нет** | Нет | Нет | Inventory/energy/health и cooldown памяти без ledger. Прямой маршрут `AddInventoryHandler` удалён из router, так как позволял менять match inventory в обход игрового пути; функция ещё существует, но HTTP не подключён. |
| Портал/эвакуация: `FinishMatchHandler` → `service.FinalizeMatch` | **Нет** | Нет | Нет | Повтор финала и частичные rewards не покрыты новым механизмом. |
| Смерть и disconnect grace: `handlePlayerDeath`, `handleMonsterDeath`, `scheduleReconnectGrace` | **Нет** | Нет | Нет | Мутируют матч из combat/timer вне координатора. |
| Создание матча: `CreateMatchHandler`; вспомогательные команды persisted artifacts: `DeleteArtifactHandler`, `TransferArtifactHandler`, `UpdateArtifactHandler` | **Нет** | Нет | Нет | Создание и служебные прямые API не перенесены. Artifacts API меняет persisted user data; перед защитой требуется определить полномочия/назначение маршрутов. |

`repository.BeginMatchCommand` остаётся единственным транзакционным компонентом: он блокирует строку `matches`, проверяет ledger и payload hash. `MatchCommand.Commit` обновляет revisions и ledger в той же транзакции. `MatchCommand.QueueEvent` накапливает WS-события; новые ветки боя отправляют их только после успешного `Commit`. `ReplayMatchCommand` позволяет `/move` распознать повтор автоатаки до попытки обычного шага. `AddInventoryItemTx` остаётся доступным для будущего переноса дропа. Локальный `playerLocks` всё ещё используется в legacy путях. Все строки таблицы с «Нет» блокируют завершение этапа 2.

`repository.AcquireMatchSerialization` держит PostgreSQL session advisory lock на время HTTP шага/атаки/подбора и HTTP/таймерной смены хода. Порядок в этих обработчиках: локальный player lock (если есть), затем advisory lock; `universalAttackLocked` вызывается из `/move` без повторного захвата. Таймер запоминает номер хода при создании, затем после advisory lock сверяет этот номер, `active_user_id` и `turn_number` с DB, чтобы не завершить тот же ход ещё раз после ожидания lock. Этот механизм координирует **только вошедшие в него** операции и не превращает отдельные SQL-записи legacy combat/turn в атомарную транзакцию. Бочки, consumables и остальные оставленные на следующий проход команды пока обходят advisory lock.

### Статус веток боя и завершения хода

| Ветка | Сериализация с шагом | Атомарный commit / revisions | Причина остатка |
| --- | --- | --- | --- |
| Нелетальные PvP/PvE без quest artifacts; Ranger только до third shot (`UniversalAttackHandler` → `tryAtomicNonlethalPvP` / `tryAtomicNonlethalPvE`) | Да, advisory + row lock | Да, state +1; map +1 только если изменилась клетка монстра | Один commit HP, энергии, монстра и клетки. Эффекты памяти и WS после commit. Реальная PostgreSQL проверка для каждого класса ниже. |
| Смертельные, artifact и Ranger third-shot атаки (`UniversalAttackHandler` → `universalAttackLocked`) | Да, advisory lock | Нет | `universalAttackLocked` вызывает `Combat.UpdatePlayer`, `UpdateMonsterHealth`, `SaveMap`, inventory/drop и WS через независимые операции. |
| Автоатака поддерживаемой нелетальной ветки (`MoveOrAttackHandler` → `tryAtomicNonlethalPvP/PvE`) | Да, тот же внешний lock без повторного захвата | Да | PostgreSQL тест HTTP `/move` по монстру и replay; остальные автоатаки остаются legacy. |
| Контратака | Внутри внешней атаки — да | Только новые нелетальные PvP/PvE ветки; `doCounterattackWithEnergy` — нет | Новый путь рассчитывает её внутри `MatchCommand`; legacy helper по-прежнему делает отдельные записи и может вызвать смерть. |
| Смерть и дроп (`handlePlayerDeath`, `handleMonsterDeath`, equipment drop helpers) | При вызове из атаки — да | Нет | Финализация, инвентарь, клетки и статистика сохраняются раздельно; некоторые другие источники смерти не входят в lock. |
| Guardian aura при выходе (`MoveOrAttackHandler`, `resolveGuardianAuraExitDamage`) | Да, внешний lock | Нет | Старый шаг сохраняет позицию и энергию до расчёта/записи HP и смерти. |
| HTTP конец хода (`EndTurnHandler`) | Да, advisory lock | Нет | Стройка, турели, artifact effect, turn row и regen обновляются по частям. |
| Конец хода по таймеру (`doEndTurn`) | Да, advisory lock и проверка active/turn DB | Нет | Использует те же нетранзакционные эффекты; устаревший таймер отсекается, но падение посередине команды оставит частичные изменения. |

### Транзакционный фундамент боя в этом проходе

`repository.MatchCommand` служит контекстом с `sql.Tx`, `InstanceID`, `CommandID`, ledger, revisions и очередью событий. `match_combat_tx.go` содержит tx-aware `MatchCombatPlayerTx`, `MatchCombatMonsterTx`, `QuestArtifactTx`, `SetCombatPlayerVitalsTx`, `SetMonsterHealthTx`, `SetMonsterHealthAndCellTx`. Последний метод сохраняет HP монстра и JSON клетки в том же `sql.Tx` и сообщает, изменился ли JSON карты. Вложенных транзакций в новых ветках нет. Существующие `LoadMapCellForUpdateTx`, `SaveMapCellTx`, `AddInventoryItemTx` остаются доступными для будущего переноса смерти и дропа.

`tryAtomicNonlethalPvP` и `tryAtomicNonlethalPvE` проверяют очередь, здоровье, энергию, цель и дистанцию после блокировки строки `matches`. Они используют старые формулы `applyDamage`, `resolveAttackMode`, `tryApplyRangerCriticalShot` и те же константы классовых эффектов. Результат удара, контратаки, энергии, HP и клетки монстра сохраняется одним commit. Успех даёт `state_revision +1`; `map_revision +1` только при изменении JSON клетки монстра. Повтор `commandId` возвращает ответ ledger; другой payload получает 409. `MATCH_PATCH` содержит `COMBAT_EXCHANGE` и `UPDATE_PLAYER`, совместимые legacy копии помечены `commandId`. Рассылка и изменения боевой статистики/временных эффектов в памяти выполняются после commit. Порядок WebSocket-доставки между backend экземплярами не гарантирован; revision gap вызывает resync.

Новые ветки покрывают только случай, когда оба участника выживают, без quest artifact и без третьего дальнего выстрела Ranger. При неподходящем случае транзакция откатывается **до записей**, затем вызывается старый `universalAttackLocked`; его `commandId` не регистрируется. Ranger third shot может толкнуть цель и поменять карту; смерть запускает удаление, дроп, turn order и иногда финализацию. Эти цепочки пока не присоединены к `sql.Tx`, поэтому остаются неатомарными. Guardian aura при шаге также остаётся в `MoveOrAttackHandler` legacy path: `resolveGuardianAuraExitDamage` меняет давление в памяти до сохранения HP, а смерть может запустить `handlePlayerDeath`.

`universalAttackLocked`, `saveTargetHealth`, `doCounterattackWithEnergy`, `handlePlayerDeath`, `handleMonsterDeath`, старые `tryApplyBerserkerBloodFeast`, `tryApplyMysticEnergyDrain`, Ranger push/refund, equipment drop и finalize всё ещё обращаются к `Combat`/`repository.DB` вне общей транзакции. Они могут отправлять WS до окончания операции. HTTP/timer конец хода и остальные команды из таблицы тоже остаются неатомарными. Новый путь рассчитывает соответствующие нелетальные эффекты внутри `MatchCommand`; старые helpers не перенесены целиком.

| Ветка | Атомарность | PostgreSQL integration-тест |
| --- | --- | --- |
| PvP Guardian: shield, обычная контратака | Только нелетально, без quest artifacts | Да: shield/block, HP/energy, counter, rollback/replay |
| PvP Mystic: Energy Drain, Arcane Overburn, pure damage | Только нелетально, без quest artifacts | Да: burn, bonus HP damage, энергия и отсутствие контратаки при нехватке энергии |
| PvP Berserker: Blood Feast и followup | Только нелетально, без quest artifacts | Да: heal, counter, followup |
| PvP Ranger: crit и первые Armor Break stacks | Только нелетально, без quest artifacts; third shot/push — legacy | Да: crit и первый stack; third shot — нет |
| PvE Guardian: counter и Guardian response | Только нелетально, без quest artifact | Да: HP строки/клетки, response, map revision |
| PvE Mystic: снижение защиты монстра | Только нелетально, без quest artifact | Да: ranged hit без контратаки |
| PvE Berserker: Blood Feast, reduced monster counter, followup | Только нелетально, без quest artifact | Да: heal, reduced counter, followup |
| PvE Ranger: crit и первые Armor Break stacks | Только нелетально, без quest artifact; third shot/push — legacy | Да: crit и первый stack; third shot — нет |
| Смерть игрока | Нет: `handlePlayerDeath` и finalize вне tx | Нет |
| Смерть монстра и дроп | Нет: `handleMonsterDeath`, карта и `maybeGrantMonsterEquipmentDrop` вне tx | Нет |
| Автоатака `MoveOrAttackHandler` | Только перечисленные нелетальные ветки | Да: HTTP `/move` по монстру и replay; прочие ветки — нет |
| Guardian aura при шаге | Нет: legacy move + memory pressure + HP/death | Нет |

## Контракт нового пути

Клиент отправляет обязательный `commandId` в JSON `/game/{instance_id}/player/{id}/move` и `/game/collectResource` (до 128 символов). Клиент отправляет `commandId` и в прямой `/attack`; для новых нелетальных веток он обязателен, а старые клиенты и прочие ветки пока продолжают legacy обработку. Сервер связывает `(instance_id, command_id)` с хешем типа команды и её параметров для нового пути. Повтор с тем же payload возвращает сохранённую ревизию без новой записи и без повторной рассылки. Повтор с другим payload получает HTTP 409. Занятые клетки с поддерживаемой нелетальной автоатакой записываются в ledger и распознаются при повторе до попытки обычного шага. Guardian aura, смерть, дроп, Ranger third shot/push и прочие неперенесённые атаки остаются legacy: их `commandId` принимается, но **не записывается в ledger**. Журнал имеет внешний ключ `ON DELETE CASCADE`, поэтому удаление завершённого матча очищает историю; если матчи сохраняются долго, понадобится отдельная retention policy с учётом допустимого окна retry. Не удалять записи для активного матча: это вновь разрешит старый `commandId`.

Один успешный обычный шаг или подбор ресурса увеличивает `state_revision` и `map_revision` на 1. При шаге меняются `is_player` флаги двух клеток; при подборе меняется содержимое клетки. Ошибка откатывает записи команды, revisions и ledger. `MATCH_PATCH` публикуется после commit с `schemaVersion: 1`, `instanceId`, `stateRevision`, `mapRevision`, `commandId`, `changes: [{type, payload}]`. Для шага в `changes` лежат совместимые `MOVE_PLAYER` и `UPDATE_PLAYER`; для подбора — `RESOURCE_COLLECTED`. После patch сервер посылает копии этих сообщений для старых клиентов; новый клиент игнорирует копии по `commandId`.

Клиент хранит последний применённый revision. Дубликат patch игнорируется, разрыв запускает HTTP snapshot; snapshot с меньшей ревизией не заменяет более новое состояние. При reconnect клиент посылает `JOIN_MATCH` и запрашивает свежий HTTP snapshot. Redux хранит revisions snapshot и patch. Legacy события без revision продолжают применяться старым способом, поэтому восстановление **не является надёжным для всех команд**. Также серверное `BuildMatchResponse` читает матч/игроков/карту разными запросами без единого repeatable-read snapshot.

## PostgreSQL и развёртывание

`RunMigrations` создаёт базовые таблицы при старте, но не применяет вручную подготовленный Stage 1 SQL. Stage 2 использует `matches.state_revision/map_revision` из Stage 1 и новую таблицу [stage2_commands.sql](stage2_commands.sql). Миграцию Stage 2 следует применить **после Stage 1** к выбранной БД до включения нового backend. К основной пользовательской БД миграции не применялись. Новый клиент отправляет `commandId`; старые HTTP-клиенты `/move` и `/collectResource` должны добавить это поле. Если таблица не создана, обычный шаг и подбор вернут ошибку БД. Текущие square правила и форма координат остаются прежними; HEX runtime и генерация не включались.

## Проверки

- `go test ./...` — успешно. PostgreSQL integration проверки запускаются только при `STAGE2_TEST_DSN`; обычный запуск их пропускает.
- `TestAtomicCombatBranchesPostgres` на отдельно поднятой временной PostgreSQL — успешно: нелетальные Guardian/Mystic/Berserker/Ranger PvP и PvE эффекты из таблицы выше; HP, энергия и JSON клетки монстра; duplicate `commandId`, payload conflict 409, отказ ledger и карты с полным rollback и без WS; две конкурентные атаки, атака параллельно с шагом; реальный HTTP `/move` по занятой клетке и replay. Hook WS проверяет, что patch видит сохранённую revision. Новое подключение к PostgreSQL читает revisions и ledger после завершения команд. Смерть/дроп, Ranger third shot и aura не проверены как атомарные.
- `TestSquareMoveTransactionIntegration` на отдельно поднятом временном PostgreSQL — успешно: два конкурентных шага дают один commit и одно списание; повтор `commandId` не списывает повторно; иной payload получает конфликт; отказ ledger полностью откатывает шаг. Подбор выдаёт предмет один раз; отказ inventory откатывает клетку и revisions. Параллельные шаг и подбор получают две последовательные revisions. State-only commit оставляет mapRevision неизменной; новая PostgreSQL-сессия читает сохранённые revisions.
- В том же integration-тесте два независимых PostgreSQL-соединения подтвердили, что второй advisory lock того же матча ждёт освобождения первого. Это проверка сериализации входа, не rollback боя/конца хода.
- `TestStaleTurnTimerPostgres` на отдельной схеме временной PostgreSQL подтвердил, что таймер, назначенный для прежнего номера хода, не меняет новый ход того же игрока после получения advisory lock. Параллельный успешный HTTP/timer end-turn с полным набором эффектов не проверен.
- Последовательность штатного `RunMigrations`, `stage1_geometry.sql`, `stage2_commands.sql` применена без ошибки на другой отдельной временной PostgreSQL; три ожидаемые таблицы присутствуют. `RunMigrations` напечатал существующее предупреждение backfill `match_player_stats`, но завершился успешно.
- `node scripts/test_hex_geometry.mjs`, `node scripts/test_geometry_contract.mjs`, `node scripts/test_revision_gate.mjs`, `npx tsc --noEmit`, `npm run build` — успешно.
- Клиентский unit для revision gate проверяет duplicate, gap и продолжение после нового snapshot. Browser E2E reconnect и потеря WS, реальный матч PvE/PvP, проверка порядка публикации при двух Go-процессах и отказ публикации после commit **не проверены**.
- Интеграционные сценарии «атака + конец хода», «смерть/дроп при повторе», «Guardian aura», «Ranger third shot/push» и «HTTP конец хода + таймер» **не выполнены как тесты атомарности**, потому что соответствующие обработчики ещё не используют `MatchCommand`. Параллельные атака + шаг и две атаки проверены только для нелетальной Guardian PvP ветки. Повтор эвакуации также не проверен.

## Оставшиеся риски этапа 2

1. Нужно перевести атаки, конец хода и таймер, бочки, постройки, портал/финал, смерть/disconnect и прочие mutation-пути на общий координатор и транзакции. Legacy combat рассылает события между записями.
2. `markArtifactMoveUsed` и Guardian/turn-local эффекты живут в памяти. Для нескольких процессов и восстановления после рестарта их требуется хранить или вычислять из транзакционного состояния. Одновременные первые шаги с Boots могут получить неверную стоимость между процессами; текущий новый путь нельзя считать многопроцессной гарантией всех правил.
3. Новый patch отправляется после commit, но межпроцессный порядок публикации и доставка не гарантированы. В текущем WS-коде `clients` — память одного процесса; второй backend не доставит событие сокетам первого. Нужны DB outbox и общий fanout либо single-owner routing. Клиентский gap detection уменьшает ущерб, но не заменяет этот механизм.
4. Для успешного шага или подбора HTTP-ответ и WS собираются после commit. Если процесс упадёт между commit и broadcast, событие потеряется; клиент восстановится при reconnect или следующем разрыве revision, но текущая открытая сессия без дальнейших событий не обновится. Нужна durable outbox.
5. Прямой `/attack` теперь запрещает HTTP `attacker_type=monster`; внутренний вызов боёвки этим не затронут. Остальные проверки полномочий и ограничения видимости требуют отдельной работы по плану.
