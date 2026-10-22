# UI Stage A — реестр функций

Аудит кода на 21.10.2026. **IMPLEMENTED** означает наличие связки UI → обработчик → данные в коде, не подтверждает работу в браузере. **PARTIAL** — видимый незавершённый участок или сценарий с неполной связкой; **PLACEHOLDER** — недоступное действие/декорация; **UNCLEAR** — поведение нельзя установить статически. `+D` означает повторение логики или источника состояния, не просто повторное отображение значения. Ручную проверку Stage A не проводили: браузер, сервер и БД не запускались. Существующие тесты в `beckend/gameservice` и `beckend/matchmaking` служат кодовым свидетельством, а не доказательством UX.

Сокращения: `G` = gameservice (`beckend/gameservice/cmd/main.go`), `M` = matchmaking (`beckend/matchmaking/Match_Making.go`), `A` = auth (`beckend/auth/Auth.go`); все API ниже относительно соответствующего сервиса. `Auth` = `AuthContext`, `P` = профиль/герои в PostgreSQL, `MP` = `match_players`/матч, `R` = локальный React state, `S` = Redux `gameSlice`, `WS` = WebSocket, `SSE` = EventSource. Упоминание backend означает найденный маршрут и обработчик; последствия без запуска остаются предположением по коду.

## Аккаунт и навигация

| ID | Функция, место | Статус | Компонент; API/обработчик | Данные → изменение состояния; зависимости/ограничение |
|---|---|---|---|---|
| F01 | Регистрация, `/register` | IMPLEMENTED | `RegisterPage`; A `POST /auth/register` | Форма → аккаунт; валидация фронта и auth backend. |
| F02 | Вход и показ пароля, `/login` | IMPLEMENTED | `LoginPage`; A `POST /auth/login` | Email/пароль → `Auth`; JWT и `localStorage`. |
| F03 | Восстановление/проверка сессии и выход | IMPLEMENTED | `AuthContext`, `RequireAuth`; A `GET /auth/profile` | `localStorage` → `Auth`; сроки JWT, маршрутизация, повторная проверка. |
| F04 | Шесть вкладок лобби | IMPLEMENTED | `LobbyHeader`; маршруты Next.js | URL → активная вкладка; Profile, History, Shop, Base, Inventory(`/equipment`), Modes. Отдельной Battle нет. |
| F05 | Переход в матч после подбора | IMPLEMENTED +D | `GlobalMatchListener`, `ModeSelectionPage`, `MatchReadyModal`; M `currentMatch`, `stream` | SSE/опрос и локальные таймеры → `/game`; два контроллера ожидания, нельзя удалять один без проверки фонового перехода. |

## Profile и социальные функции

| ID | Функция, место | Статус | Компонент; API/обработчик | Данные → изменение состояния; зависимости/ограничение |
|---|---|---|---|---|
| F06 | Личный профиль: имя, аватар, уровень, XP, баланс, статус | IMPLEMENTED | `ProfilePage`; G `GET /game/profile` | `P` → `R`; статус присутствия зависит от WS/HTTP сервера. |
| F07 | Редактирование имени и аватара | IMPLEMENTED | `ProfilePage.saveProfile`; G `PATCH /game/profile` | Поля формы → `P`, затем локальное обновление. |
| F08 | Поиск игроков по имени | IMPLEMENTED | `ProfilePage.searchPlayers`; G `GET /game/players/search?q=` | Строка поиска → список `R`; пустое и ошибочное состояние. |
| F09 | Публичный профиль игрока | IMPLEMENTED | `ProfilePage.loadPublicProfileById`; G `GET /game/profile/{id}` | ID из поиска/друга/party → просмотр; внешний режим скрывает `LobbyHeader`. |
| F10 | Список друзей и присутствие | IMPLEMENTED +D | `ProfilePage`, `ModeSelectionPage`; G `GET /game/friends` | Два независимых запроса/локальных кэша; party использует только список для приглашения. |
| F11 | Добавить друга / отправить заявку | IMPLEMENTED | `ProfilePage.addFriend`; G `POST /game/friends/add` | Статус связи → обновление списков и публичного профиля. |
| F12 | Удалить друга | IMPLEMENTED | `ProfilePage.removeFriend`; G `DELETE /game/friends/{id}` | Подтверждение и обновление `R`; доступность определяется relation. |
| F13 | Входящие заявки: список, принять, отклонить | IMPLEMENTED | `ProfilePage`; G `GET /game/friends/requests/incoming`, `POST .../{id}/accept` и `/reject` | `P` → списки/связь; после мутации повторная загрузка. |
| F14 | Исходящие заявки: список и отмена | IMPLEMENTED | `ProfilePage`; G `GET /game/friends/requests/outgoing`, `DELETE .../{id}` | `P` → `R`; зависит от статуса связи. |
| F15 | Ресурсы, баланс, прогресс героя и Base-сводка в профиле | IMPLEMENTED +D | `ProfilePage`; G `GET /game/profile` | Значения отображаются рядом с разными сводками Base/Shop; единый контракт данных для будущего `ResourceSummary` не выделен. |
| F16 | Общая статистика и статистика по классам в Profile | IMPLEMENTED | `ProfilePage` (Progress/Heroes); G `GET /game/profile`, `GET /game/profile/{id}` | `activeProgress` показывает матчи, победы, урон, XP и `classes`; серверный DTO из Stage 9. |

## History и результаты

| ID | Функция, место | Статус | Компонент; API/обработчик | Данные → изменение состояния; зависимости/ограничение |
|---|---|---|---|---|
| F17 | Список завершённых матчей | IMPLEMENTED | `MatchHistoryPage`; G `GET /game/matches/history` | JWT → список, загрузка/ошибка/пусто; показывает режим, персонажа, участников, статус. |
| F18 | Открыть детали матча | IMPLEMENTED | `MatchHistoryPage`, `MatchHistoryDetailsPage`; G `GET /game/matches/history/{id}` | ID → личная статистика, победитель, участники и награды; доступ проверяет backend. |
| F19 | Отображение итоговых наград и статистики | IMPLEMENTED +D | `MatchHistoryDetailsPage`, `HexMatchResults`, `/game/stats`; история и `hex_player_results` | Три формы одного результата; форматирование и DTO различаются, сведение требует сценариев Square/HEX. |
| F20 | Общая статистика и статистика по классам в истории | PARTIAL | `MatchHistoryPage`/`MatchHistoryDetailsPage`; G history API | Исторические записи и личная детализация есть; отдельный UI-фильтр по классу/агрегация не обнаружены. Backend Stage 9 имеет статистические расчёты. |
| F21 | Результат HEX: победа/поражение, добыча, экипировка, XP/золото | IMPLEMENTED | `HexMatchResults`; HEX snapshot `result` | `hex_player_results` → экран результата; причина молнии, повторная игра и возврат в Lobby. |
| F22 | Результат Square и восстановление после смерти | IMPLEMENTED +D | `/game/stats`, `GameResultShell`, `useGameSocket`; G `GET /game/match/{id}/my-stats`, WS | `sessionStorage` + запрос/WS → результат; отдельный путь от HEX. |
| F23 | Повторная игра из результата | IMPLEMENTED | `HexMatchResults`, `/game/stats`; переход `/mode?repeatMode=` | Предвыбор режима; повторная очередь всё равно проходит проверки Modes. |

## Shop

| ID | Функция, место | Статус | Компонент; API/обработчик | Данные → изменение состояния; зависимости/ограничение |
|---|---|---|---|---|
| F24 | Каталог расходников, свитков и чертежей | IMPLEMENTED | `LobbyShopPage`; G `GET /game/shop/items` | Каталог/цены → карточки; Forge ограничивает чертежи. |
| F25 | Переключение Buy/Sell и категорий | IMPLEMENTED | `LobbyShopPage`; локальное `R` | Только фильтр представления; не серверная операция. |
| F26 | Количество 1–99, ±1, +5, Max | IMPLEMENTED | `LobbyShopPage`; локальное `R` | Цена × количество, баланс из `GET /game/player/{id}`; сервер повторно проверяет покупку. |
| F27 | Покупка расходников/свитков/чертежей | IMPLEMENTED | `LobbyShopPage.handleBuyShopItem`; G `POST /game/shop/buy` | Товар, количество, баланс → инвентарь и баланс; повторный fetch части данных после действия. |
| F28 | Каталог зелёных комплектов и цен | PLACEHOLDER | `LobbyShopPage.GREEN_SETS`; API покупки не вызывается | Кнопка принудительно `disabled`, «Coming soon»; показанная цена не означает покупаемость. |
| F29 | Просмотр продаваемой экипировки | IMPLEMENTED | `LobbyShopPage`; G `GET /game/equipment` | Неэкипированные предметы → список; карточка пересекается с Inventory. |
| F30 | Продажа экипировки | IMPLEMENTED | `LobbyShopPage.handleSellEquipment`; G `POST /game/equipment/sell` | Confirm, instanceId → баланс/список; экипированный предмет запрещён backend. |
| F31 | Продажа расходников/свитков/ресурсов | UNCLEAR | `LobbyShopPage` sell показывает экипировку; соответствующего UI/API в проверенных маршрутах нет | Не обещать продажу иных типов в Stage B. |

## Base

| ID | Функция, место | Статус | Компонент; API/обработчик | Данные → изменение состояния; зависимости/ограничение |
|---|---|---|---|---|
| F32 | Состояние базы, уровни и стоимость построек | IMPLEMENTED | `LobbyBasePage`; G `GET /game/base/state` | Ресурсы и canBuild с сервера → карточки; цены не вычислять в UI. |
| F33 | Построить Forge | IMPLEMENTED | `LobbyBasePage.handleBuildForge`; G `POST /game/base/forge/build` | Ресурсы → уровень постройки; backend списывает. |
| F34 | Построить Library | IMPLEMENTED | `LobbyBasePage.handleBuildLibrary`; G `POST /game/base/library/build` | Аналогично; см. ограничение исследований ниже. |
| F35 | Построить Tavern | IMPLEMENTED | `LobbyBasePage.handleBuildTavern`; G `POST /game/base/tavern/build` | Уровень/ресурсы → открытие управления героями. |
| F36 | Рецепты Forge как просмотр | PARTIAL | `LobbyBasePage`, G `GET /game/base/state` содержит `recipes` | Рецепты в данных есть; отдельного UI крафта/обработчика для использования рецепта не найдено. |
| F37 | Исследования Library | PLACEHOLDER | Описание `LobbyBasePage`; отдельного действия и API исследований не найдено | Нельзя проектировать как работающую покупку улучшений. |
| F38 | Просмотр героев Tavern | IMPLEMENTED +D | `LobbyBasePage`, `ModeSelectionPage`; G `GET /game/heroes` | Два независимых запроса; Tavern содержит управление, Modes показывает активного. |
| F39 | Нанять героя | IMPLEMENTED | `LobbyBasePage.handleHireHero`; G `POST /game/heroes/{heroClassId}/hire` | Цена/доступность от сервера → список/золото; условие Tavern. |
| F40 | Сделать героя активным | IMPLEMENTED +D | `LobbyBasePage.handleSetActiveHero`; G `POST /game/heroes/active` | Активный персонаж → `P`; Modes/Inventory отдельно перечитывают. |
| F41 | Просмотр склада в Base | IMPLEMENTED +D | `StorageLoadout` без выбора; G `GET /game/storage` | Чтение сохранённого loadout; тот же компонент в Modes с редактированием. |

## Inventory / Equipment

| ID | Функция, место | Статус | Компонент; API/обработчик | Данные → изменение состояния; зависимости/ограничение |
|---|---|---|---|---|
| F42 | Выбор героя для просмотра экипировки | IMPLEMENTED | `EquipmentPage`; G `GET /game/equipment` | Локальный `selectedCharacterId`; **не** меняет активного героя аккаунта. |
| F43 | Слоты и надетые предметы | IMPLEMENTED | `EquippedSlotCard`; G `GET /game/equipment` | Состояние по characterId/slot, двуручное оружие блокирует off-hand. |
| F44 | Экипировать предмет | IMPLEMENTED | `InventoryCard`, `EquipmentPage.handleEquip`; G `POST /game/equipment/equip` | Предмет/герой/слот → обновлённая экипировка; сервер проверяет класс/слот. |
| F45 | Снять предмет | IMPLEMENTED | `EquippedSlotCard`, `handleUnequip`; G `POST /game/equipment/unequip` | Слот/герой → склад; продажа требует предварительного снятия. |
| F46 | Характеристики: база против эффективных | IMPLEMENTED +D | `StatsComparison`, `PlayerHUD`, Profile; G `GET /game/equipment` | Производные серверные характеристики; UI нормализует DTO локально. |
| F47 | Сетовые бонусы | IMPLEMENTED | `ActiveSetBonuses`; G `GET /game/equipment` | По выбранному герою; контракт `activeSetBonusesByCharacter`. |
| F48 | Фильтр/карточка доступной экипировки | IMPLEMENTED | `EquipmentPage.InventoryCard`; G `GET /game/equipment` | `R` для выбранного героя, rarity/slot/class; отдельная карточка от Shop. |
| F49 | Dev выдача классового комплекта | IMPLEMENTED | `EquipmentPage.handleGrantClassSet`; G `POST /game/equipment/dev/grant-class-set` | Показ только при `NEXT_PUBLIC_EQUIPMENT_DEV_GRANT_ENABLED=true`; не включать в обычный UX. |
| F50 | Выброс предмета из склада | PARTIAL | G `POST /game/equipment/discard` есть, явного действия в `EquipmentPage` не найдено | Backend возможность существует, пользовательский сценарий не экспонирован. |
| F51 | Редактирование loadout для поиска | IMPLEMENTED +D | `StorageLoadout` в Modes; G `GET/PUT /game/storage` | Локальный выбор → серверный склад; queue требует сохранения. Base показывает тот же набор без выбора. |

## Modes / Lobby

| ID | Функция, место | Статус | Компонент; API/обработчик | Данные → изменение состояния; зависимости/ограничение |
|---|---|---|---|---|
| F52 | Выбор PVE, 1x1, 1x2, 2x2, 3x3, 5x5 | IMPLEMENTED | `ModeSelectionPage`; M `GET /matchmaking/status` | Локальный режим/размер очереди; доступность зависит от party/team size. |
| F53 | Просмотр активного героя | IMPLEMENTED +D | `ModeSelectionPage`; G `GET /game/heroes` | Показывает активного и ссылку в Base, не меняет его здесь. |
| F54 | Просмотр party и участников | IMPLEMENTED | `ModeSelectionPage`; M `GET /matchmaking/party` | Состав/лидер/очередь → `R`; ограничения группы на сервере. |
| F55 | Друзья для приглашения в party | IMPLEMENTED +D | `ModeSelectionPage`; G `GET /game/friends` | Отдельный от Profile fetch и карточки друзей. |
| F56 | Пригласить друга | IMPLEMENTED | `ModeSelectionPage.handleAddToParty`; M `POST /matchmaking/party/invite` | Лидер, ID друга, лимит 5, состояние приглашений. |
| F57 | Входящие приглашения: принять/отклонить | IMPLEMENTED | `ModeSelectionPage`; M `GET /party/invites`, `POST /party/invite/accept` и `/reject` | Состояние группы/приглашений; текущий пользователь. |
| F58 | Удалить участника / выйти / распустить группу | IMPLEMENTED | `ModeSelectionPage`; M `POST /party/remove`, `/leave`, `/disband` | Роли лидера и статус поиска; после мутации обновление party. |
| F59 | Запустить поиск | IMPLEMENTED | `ModeSelectionPage.handleStart`; M `POST /matchmaking/join` | Сохранённый loadout, JWT, режим, party leader → очередь; сервер формирует матч. |
| F60 | Отменить поиск | IMPLEMENTED | `ModeSelectionPage.handleCancel`; M `POST /matchmaking/cancel` | Очередь → `R`; при ошибке состояние возвращается. |
| F61 | Статус очереди / текущий матч / polling | IMPLEMENTED +D | `ModeSelectionPage`, `GlobalMatchListener`; M `status`, `inQueue`, `currentMatch`, `stream` | Несколько таймеров/подписок; возможен конкурентный показ модалки/редиректа. |
| F62 | Модалка готовности, отсчёт, начать сейчас, закрыть | IMPLEMENTED +D | `MatchReadyModal`, оба контроллера | `instanceId`/redirectAt, localStorage suppression → `/game`; сохранение закрытия важно. |

## HEX HUD и матч

| ID | Функция, место | Статус | Компонент; API/обработчик | Данные → изменение состояния; зависимости/ограничение |
|---|---|---|---|---|
| F63 | Загрузка HEX / отдельный HUD | IMPLEMENTED | `GameWrapper`, `HexGameController`; G `GET /game/hex/{id}` | 409 из Square endpoint → HEX snapshot; `useHexMatchSnapshot` поддерживает WS и refresh. |
| F64 | Поле, туман, выбор клетки | IMPLEMENTED | `HexMap`, `HexGameController`; HEX snapshot | Фильтр видимости на backend; координаты q/r и выбор в UI, нельзя строить правила видимости в React. |
| F65 | Предпросмотр маршрута и ход | IMPLEMENTED | `HexGameController`; G `POST /route`, `/move` | Энергия/позиция/turn + commandId → новый snapshot; повторный клик подтверждает. |
| F66 | Выбор цели и предпросмотр атаки | IMPLEMENTED | `HexGameController`; G `POST /attack-preview` | Цель monster/player, дистанция/стоимость → локальный preview. |
| F67 | Атака монстра/игрока | IMPLEMENTED | `HexGameController`; G `POST /attack` | `commandId`, HP/энергия, боевые события; сервер считает урон и смерть. |
| F68 | Атака сооружения | IMPLEMENTED | `HexGameController`; G `POST /attack-structure` | Позиция/энергия → результат/patch; только построенные цели. |
| F69 | Подбор ресурса, открытие бочки | IMPLEMENTED | `HexGameController`; G `POST /collect`, `/open-barrel` | Текущая клетка/инвентарь → snapshot; сервер определяет содержимое. |
| F70 | Расходники и свитки матча | IMPLEMENTED | `Inventory` в HEX, `HexGameController`; G `POST /use-item` | Inventory snapshot, class/turn ограничения; сервер выполняет эффект. |
| F71 | Чертёж и размещение сооружения | IMPLEMENTED | `Inventory` + `HexGameController`; G `POST /place-blueprint` | Режим размещения, q/r, предмет → карта; нельзя связывать с Base recipe UI. |
| F72 | Передать ход, таймер, автоматический timeout | IMPLEMENTED | `HexGameController`, `useHexMatchSnapshot`; G `POST /end-turn`, `hex_turn_timer.go` | `hex_turn_deadline` в БД; UI только отображает, не продлевает. |
| F73 | Платный/артефактный выход через портал | IMPLEMENTED | `HexGameController`; G `POST /extract` | Позиция, артефакт/50 gold → индивидуальный результат; не смешивать с закрытием вкладки. |
| F74 | Отключение/возврат/молния | IMPLEMENTED | `useHexMatchSnapshot`, `HexGameController`; G `/ws`, `hex_presence.go` | Время сервера/`hex_disconnected_at`, WS heartbeat → уведомление/результат; требуется миграция Stage 10. |
| F75 | Боевые индикаторы и журнал событий | IMPLEMENTED +D | `HexGameController`, `ActionLog`, `hexCombatIndicators`; WS `MATCH_PATCH`/`MATCH_HEX_VISUAL` | Клиентская презентация по серверным фактам; Square имеет отдельные overlay/playback. |
| F76 | Способности героя в HEX как отдельная панель | PARTIAL | `HexGameController`, серверная боевая логика классов | Влияние класса есть в attack/item; отдельной панели «способности» и отдельного HEX action API нет. |
| F77 | Результат погибшего до конца матча | IMPLEMENTED | `HexMatchResults`, HEX snapshot `result` | Stage 9.1: результат участника отдельно от `finished`; выжившие продолжают. |

## Square, системные состояния и уведомления

| ID | Функция, место | Статус | Компонент; API/обработчик | Данные → изменение состояния; зависимости/ограничение |
|---|---|---|---|---|
| F78 | Square HUD, карта, выбор клетки, HP/энергия, журнал | IMPLEMENTED | `GameController`, `MapWithCamera`, `PlayerHUD`, `ActionLog`; G `GET /game/match`, WS | `S` + серверная карта/ход; детали действий F86–F94 ниже. |
| F79 | Square 3D-представление | IMPLEMENTED | `LiveMatchScene3D` в `GameController` | Есть переключатель 3D/Classic и error boundary; визуальный паритет без браузера не подтверждён. |
| F80 | Уведомления о ходе, квесте, уроне, результате | IMPLEMENTED +D | `createWsHandlers`, `useGameSocket`, `GameController`, `useHexMatchSnapshot` | WS/Redux/local React/sessionStorage; нужна единая модель событий презентации без потери протокола. |
| F81 | Глобальное уведомление о найденном матче | IMPLEMENTED +D | `GlobalMatchListener`, `MatchReadyModal`; M SSE + currentMatch | Работает вне Modes, использует localStorage suppression. |
| F82 | Загрузка, ошибки, пустые состояния | PARTIAL | Каждая страница реализует локально; G/M/A API | Набор состояний есть, формы и поведение разнородны; общих UI-контрактов нет. |
| F83 | Обновление snapshot/переподключение | IMPLEMENTED +D | `useGameSocket`, `useHexMatchSnapshot`, `GlobalMatchListener`; WS/SSE/HTTP | Разные стратегии по runtime; нельзя объединять без контроля ревизий и личного тумана. |
| F84 | HEX лаборатория и preview | IMPLEMENTED | `/hex-lab`, G `/game/hex-preview`, `/game/hex-test` | Флаги окружения; не включать в основные шесть разделов и обычный сценарий. |
| F85 | Graphics lab | IMPLEMENTED | `/graphics-lab` | Демонстрационные fixtures; не пользовательская вкладка Lobby. |

## Детализация действий Square и общения в матче

| ID | Функция, место | Статус | Компонент; API/обработчик | Данные → изменение состояния; зависимости/ограничение |
|---|---|---|---|---|
| F86 | Перемещение Square | IMPLEMENTED | `GameController`, `usePlayerActions.movePlayer`; G `POST /game/{instance_id}/player/{id}/move` | Cartesian x/y, энергия, ход; `commandId`, WS/Redux подтверждают. |
| F87 | Атака игрока/монстра Square | IMPLEMENTED | `GameController`, `usePlayerActions.fightPlayer` и `fightMonster`; G `POST /game/attack` | Цель и серверный бой; combat presentation через WS, клиент не считает урон. |
| F88 | Сбор ресурса Square | IMPLEMENTED | `GameController`, `usePlayerActions`; G `POST /game/collectResource` | Клетка/инвентарь → серверное обновление карты и игрока. |
| F89 | Открытие бочки Square | IMPLEMENTED | `GameController`, `usePlayerActions.openBarrel`; G `POST /game/openBarrel` | Клетка → предмет/урон/событие от сервера. |
| F90 | Расходник/свиток из рюкзака Square | IMPLEMENTED | `Inventory`; G `POST /game/player/{id}/inventory/use` | Предмет, instanceId, токен → эффект/обновление игрока; отдельный HEX путь. В G также зарегистрирован `/game/match/{id}/use-scroll`, но этот вызов в `Inventory` не найден. |
| F91 | Размещение чертежа Square | IMPLEMENTED | `Inventory`, `GameController`, `usePlayerActions`; G `POST /game/blueprint/place` | Режим строительства, клетка, предмет → карта; валидность серверная. |
| F92 | Завершение хода Square | IMPLEMENTED | `GameController`; G `POST /game/endTurn` | Активный игрок/энергия/ход → WS/Redux. |
| F93 | Выход через портал Square | IMPLEMENTED | `GameController`; G `POST /game/finishMatch` | Позиция/условия выхода → результат; не эквивалент закрытию вкладки. |
| F94 | Профиль игрока и заявка в друзья прямо в матче | IMPLEMENTED +D | `GameController`; G `GET /game/profile/{id}`, `POST /game/friends/add` | Контекстный просмотр/заявка; логика дублирует Profile, но доступ в матче нужен. |

## Проверенные различия и пробелы

- Кнопка покупки зелёных комплектов действительно отключена в `LobbyShopPage` (F28). Описания исследований Library и рецептов Forge не подтверждают полноценный интерфейс исследования/крафта (F36–F37).
- В Inventory выбор героя для просмотра экипировки не меняет героя аккаунта; это делает Tavern в Base (F40/F42).
- В Modes loadout сохраняется перед очередью. В Base тот же `StorageLoadout` отображается без редактирования (F41/F51).
- Profile уже показывает общую статистику и статистику по классам. История содержит список и личную детализацию, но отдельные фильтры/агрегаторы по классу на странице History не обнаружены; продажа ресурсов также не найдена.
- Статусы отражают только статический анализ кода. Ручное подтверждение матрицы сценариев A–D требуется в Stage B и финальном этапе.
