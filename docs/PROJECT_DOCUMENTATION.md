# Project Documentation

This document describes the current project as found in the repository. It is intentionally practical: file paths, real routes, real tables, and concrete debugging flows are preferred over abstract architecture text.

## Analysis Inventory

This section is the implementation inventory used to build the documentation.

### Found Services

| Service | Path | Runtime | Responsibility |
| --- | --- | --- | --- |
| Frontend | `src/`, `package.json` | Next.js, React, Redux Toolkit | Browser UI, auth pages, lobby, base/tavern, matchmaking UI, match screen, map rendering, HUD, WebSocket state updates. |
| Auth service | `beckend/auth/Auth.go` | Go HTTP service | Registration, login, JWT issuing, auth profile. |
| Game service | `beckend/gameservice/` | Go HTTP/WebSocket service | Player profiles, base buildings, heroes, match state, map, combat, inventory, stats, WebSocket broadcasting. |
| Matchmaking service | `beckend/matchmaking/` | Go HTTP/SSE service | Queue, parties, match creation requests to game service, match-ready SSE. |
| PostgreSQL | `docker-compose.yml`, `docker/postgres/` | Postgres 16 | Auth DB and game DB. |
| Nginx | `deploy/nginx/nginx.conf` | nginx:1.27-alpine | Reverse proxy for frontend, REST APIs, health endpoints, and WebSocket upgrade. |

### Found Important Folders

| Folder | Purpose |
| --- | --- |
| `src/app` | Next.js app routes and layout wrappers. |
| `src/components` | Main UI components for auth, lobby, shop, base/tavern, match screen, map, HUD, inventory, action log. |
| `src/store` | Redux store and slices for match state and combat presentation queue. |
| `src/hooks` | WebSocket, combat animation playback, player actions, keyboard/camera helpers. |
| `src/features/game` | WebSocket event handler mapping into Redux and action log updates. |
| `src/utils` | Service URL resolution, Reflex helpers, action log formatting/filtering, normalization utilities. |
| `beckend/auth` | Auth service code. Note: folder name is spelled `beckend` in the repository. |
| `beckend/gameservice/handlers` | Game REST and WebSocket handlers. |
| `beckend/gameservice/repository` | PostgreSQL connection, schema creation, query helpers, hero/base/progression persistence. |
| `beckend/gameservice/game` | Runtime match state, combat state, map/resource models, result calculation. |
| `beckend/gameservice/service` | Match finalization service. |
| `beckend/matchmaking` | Queue, party, SSE, and match creation service. |
| `docker` | Dockerfiles and PostgreSQL bootstrap/init scripts. |
| `deploy/nginx` | Nginx reverse proxy config. |
| `public` | Static images for classes, resources, tiles, forge/library items, UI assets. |

### Found Main Routes

Auth:

- `GET /healthz`
- `POST /auth/register`
- `POST /auth/login`
- `GET /auth/profile`

Game service:

- `GET /healthz`
- `GET /ws`
- `POST /create/player`
- `GET /game/player/{id}`
- `POST /game/player/{id}/gain_experience`
- `GET /game/profile`
- `PATCH /game/profile`
- `GET /game/profile/{id}`
- `GET /game/base/state`
- `POST /game/base/forge/build`
- `POST /game/base/library/build`
- `POST /game/base/tavern/build`
- `GET /game/heroes`
- `POST /game/heroes/{heroClassId}/hire`
- `POST /game/heroes/active`
- `POST /game/createMatch`
- `GET /game/match?instance_id=...`
- `GET /game/match/{instance_id}/my-stats`
- `POST /game/match/{instance_id}/use-scroll`
- `GET /game/matches/history`
- `GET /game/matches/history/{instance_id}`
- `POST /game/finishMatch`
- `POST /game/{instance_id}/player/{id}/move`
- `POST /game/attack`
- `POST /game/endTurn`
- `POST /game/collectResource`
- `POST /game/openBarrel`
- `GET /api/resources`
- `GET /api/monsters`
- `GET /game/shop/items`
- `POST /game/shop/buy`
- `POST /game/player/{id}/inventory/add`
- `POST /game/player/{id}/inventory/use`
- `POST /game/blueprint/place`
- Friend/profile routes under `/game/friends*` and `/game/players/search`
- Artifact routes under `/game/user/{id}/artifacts`, `/game/artifact/{id}`, `/game/artifact/transfer`

Matchmaking:

- `GET /healthz`
- `POST /matchmaking/join`
- `POST /matchmaking/cancel`
- `GET /matchmaking/status?mode=...`
- `GET /matchmaking/currentMatch?player_id=...`
- `GET /matchmaking/stream?player_id=...`
- `GET /matchmaking/match?player_id=...`
- `GET /matchmaking/inQueue?player_id=...`
- `GET /matchmaking/party?player_id=...`
- `GET /matchmaking/party/invites?player_id=...`
- `POST /matchmaking/party/invite`
- `POST /matchmaking/party/invite/accept`
- `POST /matchmaking/party/invite/reject`
- `POST /matchmaking/party/remove`
- `POST /matchmaking/party/leave`
- `POST /matchmaking/party/disband`
- `DELETE /matchmaking/player/{playerID}`

### Found Main Tables

Auth DB:

- `users`

Game DB:

- `monsters`
- `resources`
- `artifacts`
- `player_profiles`
- `player_characters`
- `matches`
- `match_players`
- `match_map_cells`
- `match_monsters`
- `inventory_items`
- `persisted_artifacts`
- `match_stats`
- `match_player_stats`
- `player_friends`
- `player_friend_requests`
- `player_base_buildings`
- `match_player_structures`

Legacy or removed compatibility area:

- Current code uses `player_characters` and `player_profiles.selected_character_id`.
- Current tests indicate old `player_heroes` and `selected_hero_class_id` should not exist in the current schema.
- `Needs verification`: if production data exists from the old schema, the exact one-time migration path is not fully clear from current code.

### Found Main Flows

- Registration: Browser -> Auth `/auth/register` -> Auth DB `users` -> Game `/create/player` -> Game DB `player_profiles` + starter `player_characters`.
- Login: Browser -> Auth `/auth/login` -> JWT -> localStorage -> `/auth/profile` validation on app load.
- Lobby: Browser -> `/game/profile`, `/game/base/state`, `/game/heroes`, `/matchmaking/*`.
- Matchmaking: Browser -> `/matchmaking/join` -> in-memory queue -> Matchmaking POSTs `/game/createMatch` -> SSE/polling notifies browser.
- Match start: Game creates `matches`, `match_map_cells`, `match_monsters`, `match_players` snapshots from selected `player_characters`.
- WebSocket: Browser opens `/ws?token=...&instanceId=...`, sends `JOIN_MATCH`, receives `MATCH_UPDATE` and live events.
- Combat: Browser POSTs `/game/attack` or movement endpoint -> game service validates turn/auth -> computes combat -> updates DB/runtime state -> broadcasts `COMBAT_EXCHANGE` and state updates.
- Match end: Portal finish or last survivor -> `service.FinalizeMatch` -> persistent rewards/XP/stats -> deletes active match rows and writes history tables.

## 1. Project Overview

The project is a browser multiplayer tactical game. Players register a character class, prepare in a lobby/base, queue for PvE or PvP modes, enter a generated map, collect resources/artifacts, fight monsters or other players, and finish the match by surviving or leaving through the portal with the quest artifact.

The main gameplay loop visible in code is:

```text
Register/Login
  -> Profile and lobby
  -> Prepare base/shop/tavern/heroes
  -> Join matchmaking
  -> Match created
  -> WebSocket match updates
  -> Move, collect, open barrels, build structures, fight
  -> Find quest artifact and portal or eliminate opponents
  -> Finalize match
  -> Persist stats, rewards, character XP
```

Implemented systems found in the code:

- Auth with email/password and JWT.
- Player profiles with name, avatar, balance, resources, and selected character.
- Class system: Guardian, Berserker, Ranger, Mystic.
- Base buildings: Forge, Library, Tavern.
- Tavern hero hiring and active hero selection.
- Matchmaking queues for `PVE`, `1x1`, `1x2`, `2x2`, `3x3`, `5x5`.
- Party invite flow for matchmaking.
- Generated match maps with resources, barrels, monsters, portal, and quest artifact.
- WebSocket match state broadcasting.
- Turn-based movement, attacks, end turn, and energy regen.
- Combat effects: Reflex procs, Guardian Shield Block, Berserker Blood Feast, Ranger Critical Shot, Mystic Arcane Overburn.
- Ranger Armor Break with push/fallback/reset behavior.
- Shop items, consumables, scrolls, and blueprints.
- Match history and player stats.
- Friend requests and player search.

Visible in-progress or incomplete areas:

- `/equipment` exists as a frontend route, but appears to be a placeholder.
- Some library/base unlock descriptions are returned, but not all described bonuses are clearly implemented in the inspected flow. `Needs verification`.
- Matchmaking SSE receives a `token` query parameter from the frontend, but server code appears to use only `player_id`. `Needs verification` for production auth expectations.

## 2. High-Level Architecture

```text
Browser
  -> Nginx :80
    -> Frontend :3000
    -> Auth service :8000
    -> Game service :8001
    -> Matchmaking service :8002
    -> PostgreSQL :5432
```

In Docker Compose, only Nginx is published to the host on port `80`, and Adminer is published on `8080`. Auth, game service, matchmaking, frontend, and PostgreSQL communicate through the Docker network `app_net`.

Manual/local service ports from README and service defaults:

| Component | Port |
| --- | --- |
| Frontend dev server | `3000` |
| Auth service | `8000` |
| Game service | `8001` |
| Matchmaking service | `8002` |
| Nginx | `80` |
| Adminer | `8080` |
| PostgreSQL | `5432` inside Docker network |

Request routing through Nginx:

- `/` -> frontend.
- `/auth/` -> auth service.
- `/game/` -> game service, except exact frontend pages `/game`, `/game/`, `/game/stats`, `/game/stats/`.
- `/matchmaking/` -> matchmaking service.
- `/ws` -> game service WebSocket with upgrade headers.
- `/api/resources`, `/api/monsters`, `/create/player` -> game service.
- `/healthz/auth`, `/healthz/game`, `/healthz/matchmaking` -> service health checks.

## 3. Repository Structure

```text
.
|-- README.md
|-- package.json
|-- docker-compose.yml
|-- example.env
|-- src/
|-- public/
|-- beckend/
|   |-- auth/
|   |-- gameservice/
|   `-- matchmaking/
|-- docker/
|   |-- auth.Dockerfile
|   |-- frontend.Dockerfile
|   |-- gameservice.Dockerfile
|   |-- matchmaking.Dockerfile
|   `-- postgres/
|-- deploy/
|   `-- nginx/
`-- docs/
```

Important root files:

- `README.md`: current project summary and run instructions.
- `docker-compose.yml`: full local stack.
- `example.env`: environment variable template.
- `package.json`: frontend scripts and dependencies.
- `next.config.ts`, `tsconfig.json`: frontend build config.

Important frontend folders:

- `src/app`: Next.js routes such as `/login`, `/register`, `/mode`, `/base`, `/shop`, `/profile`, `/game`, `/game/stats`, `/matches`.
- `src/components`: page-level and match UI components.
- `src/store/slices/gameSlice.ts`: authoritative frontend match state.
- `src/store/slices/combatPresentationSlice.ts`: queued combat animation state.
- `src/hooks/useGameSocket.ts`: WebSocket lifecycle.
- `src/features/game/createWsHandlers.ts`: server events -> Redux actions.
- `src/hooks/useCombatPresentationPlayback.ts`: animation/effect playback for `COMBAT_EXCHANGE`.
- `src/utils/actionLog.ts`: action log formatting and current-player relevance filtering.
- `src/utils/reflex.ts`: frontend Reflex chance and label helpers.
- `src/utils/serviceUrls.ts`: resolves API/WebSocket base URLs from `NEXT_PUBLIC_*`.

Important backend folders:

- `beckend/auth/Auth.go`: entire auth service.
- `beckend/gameservice/cmd/main.go`: game service route registration and startup.
- `beckend/gameservice/handlers`: REST/WebSocket handlers.
- `beckend/gameservice/repository`: DB schema and persistence.
- `beckend/gameservice/game`: runtime match state and result logic.
- `beckend/gameservice/internal/game/balance/formulas.go`: backend Reflex formula.
- `beckend/gameservice/service`: match finalization.
- `beckend/matchmaking/Match_Making.go`: queue/party/matchmaking API.
- `beckend/matchmaking/sse.go`: match-ready SSE.

Important Docker/deploy files:

- `docker/postgres/bootstrap/ensure-databases.sh`: ensures auth and game DBs.
- `docker/postgres/init/01-init.sql`: initial DB SQL.
- `docker/postgres/init/02-game-seed.sql`: seed/static data.
- `deploy/nginx/nginx.conf`: reverse proxy and WebSocket config.

## 4. Local Development Setup

Docker Compose is the easiest path because it matches the Nginx routing used by the frontend build.

```bash
cp example.env .env
docker compose up --build
```

Detached mode:

```bash
docker compose up -d --build
```

Open:

- Frontend through Nginx: `http://localhost`
- Adminer: `http://localhost:8080`

Health checks through Nginx:

```bash
curl -s http://localhost/healthz/auth
curl -s http://localhost/healthz/game
curl -s http://localhost/healthz/matchmaking
```

Stop containers:

```bash
docker compose down
```

Stop and remove database volume:

```bash
docker compose down -v
```

Frontend build:

```bash
npm run build
```

Backend tests:

```bash
cd beckend/gameservice
go test ./...
```

Manual development from README:

```bash
npm install
npm run dev
```

```bash
cd beckend/auth
go mod tidy
go run Auth.go
```

```bash
cd beckend/gameservice
go mod tidy
go run ./cmd
```

```bash
cd beckend/matchmaking
go mod tidy
go run Match_Making.go
```

When running services manually, set DSNs and URLs to host-local values instead of Docker service names.

## 5. Environment Variables

| Variable | Used by | Required | Example | Notes |
| --- | --- | --- | --- | --- |
| `JWT_SECRET_KEY` | Auth, game service | Yes | `replace_with_a_secure_random_string_here` | Must match in auth and game service. Use a strong secret in production. |
| `AUTH_DB_DSN` | Auth | Yes | `postgres://admin:admin@postgres:5432/admin?sslmode=disable` | Auth database for `users`. |
| `GAME_DB_DSN` | Game service, matchmaking | Yes | `postgres://admin:admin@postgres:5432/game_db?sslmode=disable` | Game DB for profiles, matches, stats, matchmaking profile lookups. |
| `GAME_SERVICE_URL` | Auth, matchmaking | Yes in Docker | `http://gameservice:8001` | Auth calls `/create/player`; matchmaking calls `/game/createMatch`. |
| `FRONTEND_ORIGIN` | Auth | Optional but recommended | `http://localhost,http://127.0.0.1` | Comma-separated allowed origins. Auth also allows localhost and Cloud Run patterns. |
| `NEXT_PUBLIC_API_BASE` | Frontend | Optional | empty string | Empty means same-origin through Nginx. |
| `NEXT_PUBLIC_WS_URL` | Frontend | Optional | `/ws` | Relative `/ws` is converted to current origin with `ws://` or `wss://`. |
| `NEXT_PUBLIC_AUTH_BASE` | Frontend | Optional | empty string | Empty means same-origin `/auth/*`. |
| `NEXT_PUBLIC_MATCHMAKING_BASE` | Frontend | Optional | empty string | Empty means same-origin `/matchmaking/*`. |
| `NEXT_PUBLIC_PREP_SECONDS` | Frontend | Optional | `15` | Match-ready modal auto-redirect delay, clamped by frontend to 5-120 seconds. |
| `PORT` | Go services | Optional | `8000`, `8001`, `8002` | Set by Docker Compose per service. |

Production notes:

- Keep `NEXT_PUBLIC_*` as relative paths when serving through Nginx.
- Use `https`/`wss` URLs when frontend and APIs are on different origins.
- Do not use gameservice CORS `*` as-is for sensitive production deployments without review.
- Use a persistent Postgres volume or managed database.

## 6. Backend Documentation

### Auth Service

Path: `beckend/auth/Auth.go`

Responsibilities:

- Create auth users in `users`.
- Hash passwords with bcrypt.
- Issue JWTs with `user_id` and 6-hour expiration.
- Validate JWT for `/auth/profile`.
- Call game service `/create/player` after registration.

Main routes:

- `GET /healthz`
- `POST /auth/register`
- `POST /auth/login`
- `GET /auth/profile`

JWT flow:

1. Login validates email/password.
2. Auth signs JWT with `JWT_SECRET_KEY`.
3. Frontend stores user and token in localStorage.
4. Frontend sends `Authorization: Bearer <token>` to protected game routes.
5. WebSocket sends the token as query param: `/ws?token=<token>&instanceId=<id>`.

Registration flow:

```text
POST /auth/register
  -> insert users row
  -> POST GAME_SERVICE_URL/create/player
  -> create game profile and starter character
```

`Needs verification`: registration is not transactional across auth DB and game DB. If auth user creation succeeds but game profile creation fails, auth logs the game-service error but does not roll back the auth user.

Database tables used:

- `users`

Important files:

- `beckend/auth/Auth.go`
- `beckend/auth/go.mod`
- `docker/auth.Dockerfile`

### Game Service

Path: `beckend/gameservice/`

Responsibilities:

- Own game database schema and migrations.
- Own player profiles, characters, base buildings, heroes, inventory.
- Create matches and snapshot selected characters into `match_players`.
- Own authoritative turn state and combat.
- Broadcast match events over WebSocket.
- Finalize matches and write stats/history.

Startup:

- `cmd/main.go` reads `GAME_DB_DSN`, `JWT_SECRET_KEY`, `PORT`.
- `repository.ConnectDB()`.
- `repository.RunMigrations()`.
- `repository.EnsureSchemaReady()`.
- Register REST and WebSocket routes.

Important handlers:

- `handlers/players.go`: player profile creation and old player APIs.
- `handlers/profile.go`: authenticated profile/friends APIs.
- `handlers/base_buildings.go`: Forge, Library, Tavern.
- `handlers/heroes.go`: hero catalog, hire, active selection.
- `handlers/match.go`: match creation and match response.
- `handlers/combat.go`: movement and attacks.
- `handlers/turn.go`: end turn, construction progress, turret effects.
- `handlers/ws_handler.go`: WebSocket clients and broadcasts.
- `handlers/resource_collection.go`: resource pickup.
- `handlers/barrel_http.go`, `handlers/barrel.go`: barrels and quest artifact drop rules.
- `handlers/shop.go`: shop items and purchases.
- `handlers/inventory.go`: use food/water/scrolls.
- `handlers/blueprint_building.go`: in-match structure placement.
- `handlers/finish_match_handler.go`: portal finish.
- `handlers/match_history_handler.go`: completed match history.

Game service is authoritative for combat. Frontend displays backend-provided state and explicit combat effects.

### Matchmaking Service

Path: `beckend/matchmaking/`

Responsibilities:

- Maintain in-memory queues per mode.
- Maintain in-memory party state and party invites.
- Create match plans when enough queued players exist.
- POST match creation requests to game service.
- Notify clients via SSE and fallback polling.
- Remove player-to-match mapping when game service tells it a player/match ended.

Modes and required players:

| Mode | Required players | Team count | Team size limit |
| --- | ---: | ---: | ---: |
| `PVE` | 1 | 1 | 1 |
| `1x1` | 2 | 2 | 1 |
| `1x2` | 3 | 2 | 1 |
| `2x2` | 4 | 2 | 2 |
| `3x3` | 6 | 2 | 3 |
| `5x5` | 10 | 2 | 5 |

Important files:

- `beckend/matchmaking/Match_Making.go`
- `beckend/matchmaking/sse.go`
- `beckend/matchmaking/go.mod`
- `docker/matchmaking.Dockerfile`

`Needs verification`: matchmaking HTTP and SSE routes appear to trust `player_id` and do not validate JWT in the inspected code. The frontend passes `token` to SSE, but `sse.go` does not use it.

## 7. Frontend Documentation

Frontend stack:

- Next.js app router.
- React components under `src/components`.
- Redux Toolkit store under `src/store`.
- Auth context under `src/contexts/AuthContext.tsx`.
- Service URL helper under `src/utils/serviceUrls.ts`.

Routing:

| Route | Component | Purpose |
| --- | --- | --- |
| `/` | `src/app/page.tsx` | Landing/start page. |
| `/login` | `LoginPage` | Login form. |
| `/register` | `RegisterPage` | Register account and pick starting class. |
| `/mode` | `ModeSelectionPage` | Lobby, queue, parties, active hero summary. |
| `/base` | `LobbyBasePage` | Base buildings, Tavern, heroes, active hero. |
| `/shop` | `LobbyShopPage` | Buy food, water, blueprints, scrolls. |
| `/profile` | `ProfilePage` | Profile, stats, friends, public profile lookup. |
| `/equipment` | Placeholder page | Equipment system not implemented in inspected code. |
| `/game?instance_id=...` | `GameWrapper` + `GameController` | Active match screen. |
| `/game/stats` | stats page | Post-match stats display. |
| `/matches` | match history page | Completed matches list. |
| `/matches/[instanceId]` | match detail page | Completed match detail. |

Auth flow:

- `LoginPage` calls `POST /auth/login`.
- `RegisterPage` calls `POST /auth/register`.
- `AuthContext` stores the returned user/token in `localStorage`.
- On app load, `AuthContext` checks JWT expiration and calls `GET /auth/profile`.
- `RequireAuth` redirects unauthenticated users to `/login`.

API clients:

- `src/utils/serviceUrls.ts` exports `AUTH_BASE`, `API_BASE`, `MATCHMAKING_BASE`, `WS_URL`.
- Empty `NEXT_PUBLIC_*` values mean same-origin, which works with Nginx.
- `WS_URL` defaults to `/ws` and is converted to `ws://host/ws` or `wss://host/ws`.

State management:

- `src/store/index.ts` combines:
  - `game` slice.
  - `combatPresentation` slice.
- `gameSlice` stores match grid, players, active user, turn number, quest artifact info, and action log.
- `combatPresentationSlice` queues `COMBAT_EXCHANGE` events for presentation playback.

Important components and hooks:

| File | What it does | Data/events |
| --- | --- | --- |
| `src/components/GlobalMatchListener.tsx` | Global SSE/polling listener after login. | `/matchmaking/stream`, `/matchmaking/currentMatch`, opens match-ready modal. |
| `src/components/ModeSelectionPage.tsx` | Queue mode selection, party invites, active hero preview. | `/matchmaking/*`, `/game/friends`, `/game/heroes`. |
| `src/components/LobbyBasePage.tsx` | Base buildings and Tavern UI. | `/game/base/state`, `/game/heroes`, build/hire/active hero endpoints. |
| `src/components/LobbyShopPage.tsx` | Preparation shop. | `/game/shop/items`, `/game/shop/buy`, `/game/base/state`. |
| `src/components/ProfilePage.tsx` | Profile, edit profile, friends, search, public profile. | `/game/profile`, `/game/profile/{id}`, `/game/friends*`. |
| `src/features/game/GameWrapper.tsx` | Loads resources/monsters/match and starts WebSocket. | `/api/resources`, `/api/monsters`, `/game/match`. |
| `src/components/GameController.tsx` | Main match screen and controls. | Player actions, end turn, finish match, profile modal. |
| `src/hooks/useGameSocket.ts` | WebSocket lifecycle and reconnect. | Sends `JOIN_MATCH`, receives server events. |
| `src/features/game/createWsHandlers.ts` | Converts WS events to Redux updates and action log entries. | Handles `MATCH_UPDATE`, `COMBAT_EXCHANGE`, `UPDATE_PLAYER`, etc. |
| `src/hooks/usePlayerActions.ts` | Click actions for move, attack, collect, barrel, blueprint. | Calls movement, attack, resource, barrel, blueprint endpoints. |
| `src/hooks/useCombatPresentationPlayback.ts` | Plays combat exchange animation/effects. | Uses explicit backend `effects`. |
| `src/hooks/useCombatFloaters.ts` | Legacy HP-diff floaters. | Suppressed during active `COMBAT_EXCHANGE` to avoid duplicate damage floaters. |
| `src/utils/actionLog.ts` | Formats and filters action log entries. | Filters relevance to current player. |
| `src/components/PlayerHUD.tsx` | Shows current player stats. | Displays DB/API `agility` as UI `Reflex`. |

`Needs verification`: `/game/page.tsx` appears to wrap the game route in a Redux Provider while the root layout already uses `ReduxProvider`. It may be harmless but should be checked before refactoring.

## 8. API Documentation

All protected game endpoints use:

```http
Authorization: Bearer <jwt>
```

### Auth API

#### `GET /healthz`

Purpose: Check auth service DB connectivity and `users` table.

Auth required: No.

Response: JSON health status or error.

Used by frontend: Not directly; used by Docker/Nginx/debugging.

#### `POST /auth/register`

Purpose: Create auth user and game profile.

Auth required: No.

Request body:

```json
{
  "email": "user@example.com",
  "password": "secret123",
  "name": "Player",
  "image": "/guardian/guardian.webp",
  "characterType": "guardian"
}
```

Response: Text success message with status `201`.

Important notes: Calls game service `/create/player` after inserting `users`.

Used by frontend: `src/components/RegisterPage.tsx`.

#### `POST /auth/login`

Purpose: Login and receive JWT.

Auth required: No.

Request body:

```json
{
  "email": "user@example.com",
  "password": "secret123"
}
```

Response:

```json
{
  "id": 1,
  "email": "user@example.com",
  "name": "Player",
  "created_at": "...",
  "token": "..."
}
```

Used by frontend: `src/components/LoginPage.tsx`, `src/contexts/AuthContext.tsx`.

#### `GET /auth/profile`

Purpose: Validate JWT and return auth user.

Auth required: Yes.

Response:

```json
{
  "id": 1,
  "email": "user@example.com",
  "name": "Player",
  "created_at": "..."
}
```

Used by frontend: `src/contexts/AuthContext.tsx`.

### Game API

#### `POST /create/player`

Purpose: Create game profile and starter character for a newly registered auth user.

Auth required: No. Called by auth service.

Request body:

```json
{
  "user_id": 1,
  "name": "Player",
  "image": "/guardian/guardian.webp",
  "character_type": "guardian"
}
```

Response: Player profile JSON.

Used by frontend: Not directly.

#### `GET /game/profile`

Purpose: Current authenticated player profile, progress, resources, and base summary.

Auth required: Yes.

Response:

```json
{
  "status": "ok",
  "data": {
    "player": {
      "userId": 1,
      "name": "Player",
      "characterType": "guardian",
      "level": 1,
      "experience": 0,
      "maxExperience": 100,
      "balance": 0,
      "attack": 9,
      "defense": 8,
      "mobility": 2,
      "agility": 5,
      "sightRange": 2,
      "isRanged": false,
      "attackRange": 1
    },
    "progress": {},
    "resources": {},
    "base": {}
  }
}
```

Used by frontend: `ProfilePage`, `AuthContext` indirectly for logged-in app state.

#### `PATCH /game/profile`

Purpose: Update profile name/avatar.

Auth required: Yes.

Request body:

```json
{
  "name": "New name",
  "image": "/guardian/guardian.webp"
}
```

Response: Same shape as `GET /game/profile`.

Used by frontend: `ProfilePage`.

#### `GET /game/profile/{id}`

Purpose: View another player's public profile and friendship relation.

Auth required: Yes.

Request body: None.

Response: `status`, `data.player`, `data.progress`, `isFriend`, `friendRelation`.

Used by frontend: `ProfilePage`, profile modal in `GameController`.

#### `GET /game/base/state`

Purpose: Return Forge, Library, Tavern state, costs, owned resources, and unlockables.

Auth required: Yes.

Response:

```json
{
  "forgeLevel": 0,
  "built": false,
  "costs": { "wood": 40, "stone": 30, "iron": 20 },
  "resources": { "wood": 0, "stone": 0, "iron": 0 },
  "canBuild": false,
  "recipes": [],
  "forge": {},
  "library": {},
  "tavern": {}
}
```

Used by frontend: `LobbyBasePage`, `LobbyShopPage`, `ProfilePage`.

#### `POST /game/base/forge/build`

Purpose: Build Forge.

Auth required: Yes.

Request body: Empty.

Response: Updated base state.

Important notes: Costs `40 wood`, `30 stone`, `20 iron`.

Used by frontend: `LobbyBasePage`.

#### `POST /game/base/library/build`

Purpose: Build Library.

Auth required: Yes.

Request body: Empty.

Response: Updated base state.

Important notes: Costs `30 wood`, `45 stone`, `25 iron`.

Used by frontend: `LobbyBasePage`.

#### `POST /game/base/tavern/build`

Purpose: Build Tavern.

Auth required: Yes.

Request body: Empty.

Response: Updated base state.

Important notes: Costs `10 wood`, `10 stone`, `10 iron`.

Used by frontend: `LobbyBasePage`.

#### `GET /game/heroes`

Purpose: Return hero catalog, owned heroes, hire/lock state, active hero.

Auth required: Yes.

Response:

```json
{
  "activeHeroClassId": "guardian",
  "tavernBuilt": false,
  "gold": 0,
  "heroes": [
    {
      "id": "ranger",
      "displayName": "Ranger",
      "owned": false,
      "characterId": 0,
      "active": false,
      "locked": true,
      "unlockPrice": 2500,
      "requiresTavern": true,
      "canHire": false,
      "lockReason": "tavern_required",
      "enabled": true,
      "sortOrder": 30
    }
  ]
}
```

Used by frontend: `LobbyBasePage`, `ModeSelectionPage`.

#### `POST /game/heroes/{heroClassId}/hire`

Purpose: Hire a new hero into `player_characters`.

Auth required: Yes.

Request body: Empty.

Response: Updated heroes state.

Important notes:

- `heroClassId` is one of `guardian`, `berserker`, `ranger`, `mystic`.
- Price is `2500` gold.
- Tavern is required.
- Existing hero returns conflict.

Used by frontend: `LobbyBasePage`.

#### `POST /game/heroes/active`

Purpose: Select the active owned hero.

Auth required: Yes.

Request body:

```json
{
  "heroClassId": "ranger"
}
```

Response: Updated heroes state.

Important notes: Updates `player_profiles.selected_character_id`.

Used by frontend: `LobbyBasePage`.

#### `POST /game/createMatch`

Purpose: Create a match from matchmaking plan.

Auth required: No. Called by matchmaking service.

Request body:

```json
{
  "instance_id": "uuid",
  "mode": "1x1",
  "player_ids": [1, 2],
  "group_ids": [1, 2],
  "teams": [
    { "group_id": 1, "player_ids": [1] },
    { "group_id": 2, "player_ids": [2] }
  ],
  "turn_order": [1, 2],
  "teams_count": 2,
  "total_players": 2
}
```

Response: Match response with map, players, active user, turn number, quest/portal metadata.

Important notes: Snapshots selected `player_characters` into `match_players`.

Used by frontend: Not directly.

#### `GET /game/match?instance_id=...`

Purpose: Load current match state.

Auth required: Yes.

Request body: None.

Response: `instance_id`, `mode`, map dimensions, grid/cells, players, active user, turn number, portal/quest fields.

Used by frontend: `GameWrapper`, `useGameSocket`.

#### `GET /game/match/{instance_id}/my-stats`

Purpose: Load stored stats for current user after match finalization.

Auth required: Yes.

Response:

```json
{
  "status": "ok",
  "stats": {
    "instanceId": "...",
    "winnerType": "user",
    "winnerId": 1,
    "player": {}
  }
}
```

Used by frontend: Stats flow.

#### `POST /game/match/{instance_id}/use-scroll`

Purpose: Use a purchased scroll and reveal one coordinate.

Auth required: Yes.

Request body:

```json
{
  "scroll_type": "scroll_portal_x"
}
```

Response: Axis/value result.

Important notes: `UseInventoryHandler` also supports scroll use through `/game/player/{id}/inventory/use`; this route is a specific scroll API.

Used by frontend: `Needs verification`; inventory code currently calls `/game/player/{id}/inventory/use`.

#### `POST /game/{instance_id}/player/{id}/move`

Purpose: Move one tile or attack an occupied adjacent/target cell through the move handler.

Auth required: Yes. Token user must match `{id}`.

Request body:

```json
{
  "new_pos_x": 3,
  "new_pos_y": 4
}
```

Response: Movement result or attack result.

Important notes: Only active player can move. Movement consumes energy based on mobility.

Used by frontend: `usePlayerActions`.

#### `POST /game/attack`

Purpose: Universal attack endpoint for player/monster targets.

Auth required: Yes for player attacker.

Request body:

```json
{
  "instance_id": "...",
  "attacker_type": "player",
  "attacker_id": 1,
  "target_type": "monster",
  "target_id": 12
}
```

Response:

```json
{
  "damage_to_target": 7,
  "new_target_hp": 20,
  "counter_damage": 0,
  "new_attacker_hp": 130,
  "attack_mode": "melee"
}
```

Important notes: The authoritative combat details are broadcast through `COMBAT_EXCHANGE`.

Used by frontend: `usePlayerActions`.

#### `POST /game/endTurn`

Purpose: End active player's turn.

Auth required: Yes. Token user must match body `user_id`.

Request body:

```json
{
  "user_id": 1,
  "instance_id": "..."
}
```

Response:

```json
{
  "active_user": 2,
  "energy": 100,
  "turn_number": 3
}
```

Important notes: Also progresses construction, turret effects, combat state duration, energy regen, and WebSocket `SET_ACTIVE_USER`.

Used by frontend: `GameController`.

#### `POST /game/collectResource`

Purpose: Collect a resource/artifact from a map cell.

Auth required: Yes. Token user must match body `user_id`.

Request body:

```json
{
  "instance_id": "...",
  "user_id": 1,
  "cell_x": 5,
  "cell_y": 6
}
```

Response: `message`, `updatedCell`, `updatedPlayer`.

Important notes: Broadcasts `RESOURCE_COLLECTED`; quest artifact pickup also broadcasts `QUEST_ARTIFACT_FOUND`.

Used by frontend: `usePlayerActions`.

#### `POST /game/openBarrel`

Purpose: Open a barrel cell.

Auth required: Yes. Token user must match body `user_id`.

Request body:

```json
{
  "instance_id": "...",
  "user_id": 1,
  "cell_x": 5,
  "cell_y": 6
}
```

Response: `updatedCell`, `updatedPlayer`, optional `matchEnded`.

Important notes: Barrel may damage player, drop resource, drop artifact, or force quest artifact on last barrel.

Used by frontend: `usePlayerActions`.

#### `POST /game/finishMatch`

Purpose: Finish match through portal after carrying quest artifact.

Auth required: Yes.

Request body:

```json
{
  "instanceId": "..."
}
```

Response:

```json
{
  "status": "ok",
  "stats": {}
}
```

Important notes: Broadcasts `PLAYER_LEFT_PORTAL` and `MATCH_ENDED`, then finalizes stats/rewards and active match cleanup.

Used by frontend: `GameController`.

#### `GET /game/shop/items`

Purpose: List food, water, blueprints, and scrolls.

Auth required: No.

Response:

```json
{
  "items": [
    {
      "id": 1,
      "type": "food",
      "category": "resource",
      "name": "Food",
      "price": 12,
      "inventoryKey": "",
      "requiresForge": false
    }
  ]
}
```

Used by frontend: `LobbyShopPage`.

#### `POST /game/shop/buy`

Purpose: Buy preparation items into persistent profile inventory.

Auth required: Yes.

Request body:

```json
{
  "player_id": 1,
  "item_type": "food",
  "count": 1
}
```

Response: Updated player, bought item, count, total cost.

Important notes:

- Food price: `12`.
- Water price: `8`.
- Blueprints require Forge.
- Scrolls require Library.
- Count max is `99`.

Used by frontend: `LobbyShopPage`.

#### `POST /game/player/{id}/inventory/use`

Purpose: Use food, water, or scroll during a match.

Auth required: Yes. Token user must match `{id}`.

Request body:

```json
{
  "instance_id": "...",
  "item_type": "resource",
  "item_id": 1,
  "count": 1
}
```

Response: Updated match player, or `{ player, scroll_result }` for scrolls.

Important notes:

- Only active player can use.
- Food can be used once per 2 turns.
- Water max is 5 uses over 2 turns.
- Broadcasts `UPDATE_PLAYER`.

Used by frontend: `Inventory`.

#### `POST /game/blueprint/place`

Purpose: Start construction of an in-match structure.

Auth required: Yes.

Request body:

```json
{
  "instance_id": "...",
  "user_id": 1,
  "cell_x": 4,
  "cell_y": 5,
  "blueprint_key": "blueprint_turret"
}
```

Response: `updatedCell`, `updatedPlayer`.

Important notes:

- Only active player can place.
- Target cell must be adjacent and empty ordinary tile.
- Construction takes 2 turns.
- Broadcasts `UPDATE_CELL` and `UPDATE_PLAYER`.

Used by frontend: `usePlayerActions`, `Inventory`, `GameController`.

### Matchmaking API

#### `GET /healthz`

Purpose: Check matchmaking service and game schema readiness.

Auth required: No.

Response: JSON health status.

Used by frontend: Not directly.

#### `POST /matchmaking/join`

Purpose: Join matchmaking queue, or return existing active match if still valid.

Auth required: No in inspected server code.

Request body:

```json
{
  "player_id": 1,
  "mode": "1x1",
  "rating": 1000
}
```

Response: Text `"You have joined the queue"` or `MatchInfo` JSON.

Important notes: Only party leader can start queue. Party size cannot exceed team size.

Used by frontend: `ModeSelectionPage`.

#### `POST /matchmaking/cancel`

Purpose: Remove player/party from queue.

Auth required: No in inspected server code.

Request body:

```json
{
  "player_id": 1,
  "mode": "1x1"
}
```

Response: Text success or `404`.

Used by frontend: `ModeSelectionPage`.

#### `GET /matchmaking/status?mode=...`

Purpose: Read queue status for a mode.

Auth required: No.

Response:

```json
{
  "queue": [],
  "totalPlayers": 0
}
```

Used by frontend: `ModeSelectionPage`.

#### `GET /matchmaking/currentMatch?player_id=...`

Purpose: Check if player has a current match.

Auth required: No in inspected server code.

Response: `{"status":"waiting"}` or `MatchInfo`.

Used by frontend: `GlobalMatchListener`, `ModeSelectionPage`, `useGameSocket` fallback flow.

#### `GET /matchmaking/stream?player_id=...`

Purpose: SSE stream for match-ready notification.

Auth required: No in inspected server code.

Response: `text/event-stream` with `data: <MatchInfo JSON>`.

Important notes: Frontend sends `token`, but server does not read it in `sse.go`. `Needs verification`.

Used by frontend: `GlobalMatchListener`.

#### `GET /matchmaking/match?player_id=...`

Purpose: Compatibility route delegating to the current match handler.

Auth required: No.

Response: Same as current match handler.

Important notes: The route name is generic, but the current implementation delegates to `currentMatchHandler`, so it expects `player_id`. `Not fully clear from current code` whether `instance_id` was intended here earlier.

Used by frontend: Not clearly used in current inspected frontend.

#### `GET /matchmaking/inQueue?player_id=...`

Purpose: Check if player is already queued.

Auth required: No.

Response:

```json
{
  "inQueue": true,
  "mode": "1x1",
  "leaderId": 1,
  "partySize": 1
}
```

Used by frontend: `ModeSelectionPage`.

#### `GET /matchmaking/party?player_id=...`

Purpose: Read party state.

Auth required: No.

Response: Party state from `buildPartyStateForPlayer`.

Used by frontend: `ModeSelectionPage`.

#### `GET /matchmaking/party/invites?player_id=...`

Purpose: Read pending party invites.

Auth required: No.

Response:

```json
{
  "status": "ok",
  "invites": []
}
```

Used by frontend: `ModeSelectionPage`.

#### `POST /matchmaking/party/invite`

Purpose: Leader invites a friend to party.

Auth required: No in inspected server code.

Request body:

```json
{
  "leader_id": 1,
  "member_id": 2
}
```

Response: Party state.

Important notes: Requires friendship, neither player in match/queue, max party size 5.

Used by frontend: `ModeSelectionPage`.

#### `POST /matchmaking/party/invite/accept`

Purpose: Accept party invite.

Auth required: No in inspected server code.

Request body:

```json
{
  "player_id": 2
}
```

Response: Party state.

Used by frontend: `ModeSelectionPage`.

#### `POST /matchmaking/party/invite/reject`

Purpose: Reject party invite.

Auth required: No in inspected server code.

Request body:

```json
{
  "player_id": 2
}
```

Response: `{"status":"ok"}`.

Used by frontend: `ModeSelectionPage`.

#### `POST /matchmaking/party/remove`

Purpose: Leader removes member.

Auth required: No in inspected server code.

Request body:

```json
{
  "leader_id": 1,
  "member_id": 2
}
```

Response: Party state.

Used by frontend: `ModeSelectionPage`.

#### `POST /matchmaking/party/leave`

Purpose: Player leaves party.

Auth required: No in inspected server code.

Request body:

```json
{
  "player_id": 2
}
```

Response: Party state.

Used by frontend: `ModeSelectionPage`.

#### `POST /matchmaking/party/disband`

Purpose: Leader disbands party.

Auth required: No in inspected server code.

Request body:

```json
{
  "leader_id": 1
}
```

Response: Party state.

Used by frontend: `ModeSelectionPage`.

## 9. WebSocket Documentation

WebSocket endpoint:

```text
GET /ws?token=<jwt>&instanceId=<instance_id>
```

Connection flow:

1. Frontend builds URL in `src/hooks/useGameSocket.ts`.
2. Token is passed as query param.
3. Game service validates JWT with `JWT_SECRET_KEY`.
4. One active WebSocket is kept per user; a new connection closes the old one.
5. Client sends `JOIN_MATCH`.
6. Server returns `MATCH_UPDATE` or `MATCH_ENDED`.

Client message:

```json
{
  "type": "JOIN_MATCH",
  "instanceId": "..."
}
```

Important server behavior:

- Broadcasts are scoped by `instanceId`.
- On disconnect while alive in a match, server broadcasts `PLAYER_DISCONNECTED` with `graceMs`.
- Reconnect within grace broadcasts `PLAYER_RECONNECTED`.
- Grace period is 3 minutes.
- If grace expires, the player is defeated/disconnected by server death handling.

### WebSocket Events

| Event | Sender | Receiver | Payload shape | UI updates |
| --- | --- | --- | --- | --- |
| `JOIN_MATCH` | Client | Server | `{ type, instanceId }` | Registers connection to match. |
| `MATCH_UPDATE` | Server | Client | Full match response | `setMatchData`; initializes map/players. |
| `COMBAT_EXCHANGE` | Server | Client | `{ instanceId, exchangeId, attackerId, attackerType, targetId, targetType, attackStyle, steps, effects }` | Enqueue presentation, apply HP/death state, action log. |
| `MOVE_PLAYER` | Server | Client | `{ instanceId, userId/playerId, position, updatedCell? }` | Move player in Redux/map. |
| `UPDATE_PLAYER` | Server | Client | `{ instanceId, player }` | Replace/update player state. |
| `UPDATE_CELL` | Server | Client | `{ instanceId, updatedCell }` | Update one grid cell. |
| `SET_ACTIVE_USER` | Server | Client | `{ instanceId, active_user, energy, turnNumber }` | Turn indicator, active player, logs. |
| `TURN_PASSED` | Server | Client | Legacy turn payload | Frontend normalizes to active-user flow. |
| `PLAYER_DEFEATED` | Server | Client | `{ instanceId, userId, updatedCell?, matchEnded? }` | Remove/mark player, redirect defeated current user flow. |
| `MATCH_ENDED` | Server | Client | `{ instanceId, winnerType, winnerId, stats }` | Store stats, navigate to stats/mode. |
| `UPDATE_INVENTORY` | Server | Client | Inventory/player payload | Update player inventory. |
| `RESOURCE_COLLECTED` | Server | Client | `{ instanceId, updatedCell, updatedPlayer }` | Update cell/player and action log. |
| `QUEST_ARTIFACT_FOUND` | Server | Client | `{ instanceId, playerName, x, y }` | Quest notification and log. |
| `PLAYER_LEFT_PORTAL` | Server | Client | `{ instanceId, playerName, x, y }` | Action log. |
| `BARREL_DAMAGE` | Server | Client | `{ instanceId, userId, amount, hp }` | HP update and action log. |
| `BARREL_RESOURCE` | Server | Client | Barrel/resource payload | Resource log/update. |
| `BARREL_ARTIFACT` | Server | Client | Barrel/artifact payload | Artifact log/update. |
| `PLAYER_DISCONNECTED` | Server | Client | `{ instanceId, userId, graceMs }` | Disconnect panel/status. |
| `PLAYER_RECONNECTED` | Server | Client | `{ instanceId, userId }` | Clear disconnect status. |

`COMBAT_EXCHANGE` processing:

```text
Server handles /game/attack
  -> updates DB/runtime state
  -> records steps/effects
  -> Broadcast COMBAT_EXCHANGE
  -> Broadcast UPDATE_PLAYER/UPDATE_CELL as needed

Frontend receives COMBAT_EXCHANGE
  -> createWsHandlers builds action log entries
  -> combatPresentationSlice queues exchange
  -> gameSlice applies authoritative HP/death state
  -> useCombatPresentationPlayback plays motions, floaters, effect text
```

Presentation rule: do not infer `BLOCK`, `CRIT`, `LIFESTEAL`, or `OVERBURN` from HP numbers. The backend sends explicit `effects`, and the frontend displays those.

## 10. Combat System Documentation

Main file: `beckend/gameservice/handlers/combat.go`

Frontend presentation files:

- `src/features/game/createWsHandlers.ts`
- `src/store/slices/gameSlice.ts`
- `src/store/slices/combatPresentationSlice.ts`
- `src/hooks/useCombatPresentationPlayback.ts`
- `src/hooks/useCombatFloaters.ts`

### Universal Attack Flow

```text
POST /game/attack
  -> validate attacker/target and JWT
  -> require attacker is active user
  -> resolve attacker and target stats
  -> resolve attack mode by distance/range
  -> check energy and friendly fire
  -> spend attack energy
  -> calculate primary damage
  -> apply class mechanics and Reflex effects
  -> apply Ranger Armor Break mechanics if applicable
  -> handle counterattack if applicable
  -> handle Berserker follow-up if applicable
  -> save health, energy, stats, deaths
  -> broadcast COMBAT_EXCHANGE
  -> broadcast state updates
```

Energy costs:

- Melee attack: `6`.
- Ranged attack: `8`.
- Standard counterattack: `2`.
- Guardian counterattack: `0`.

Movement cost by mobility:

| Mobility | Move energy cost |
| ---: | ---: |
| `<= 2` | `4` |
| `3..5` | `3` |
| `6..8` | `2` |
| `> 8` | `1` |

Guardian zone control:

- Range: `2`.
- Move penalty: `+1`.
- Aura exit damage cap: `5`.

### Attack Mode

- Melee if target distance is `<= 1`.
- Ranged if attacker `is_ranged` and target distance is within `attack_range`.
- Otherwise attack is rejected.
- Friendly fire is rejected for players with the same non-zero `group_id`.

### Damage Calculation

Base:

```text
effectiveDefense = target.defense - armorBreakStacks * 2
effectiveDefense = max(effectiveDefense, 0)
damage = attacker.attack - effectiveDefense
damage = max(damage, 0)
```

Berserker wounded-target bonus applies to primary damage when the target is already wounded:

- HP ratio `< 25%`: `+35%`
- HP ratio `< 50%`: `+20%`
- HP ratio `< 75%`: `+10%`

### Defense and Blocking

Guardian Shield Block:

- Can trigger when the target is a Guardian player.
- Uses Reflex proc chance.
- On success, primary damage becomes `0`.
- Backend emits effect kind `block`.
- A block prevents Ranger Armor Break stack application and Mystic drain/overburn on that blocked hit.

### Counterattack

Counterattack can happen when:

- Attack was melee and target survived.
- Or Guardian blocked a ranged attack while adjacent.

Counterattack:

- Uses defender attack against attacker defense.
- Costs `2` energy, except Guardian counterattack is free.
- Can damage or kill attacker.
- Is represented as a `counter` step in `COMBAT_EXCHANGE`.

### Death Handling

Player death:

- Records defeat in match state.
- Handles quest artifact transfer/drop.
- Removes player from turn order.
- Clears map cell player flag.
- Calls matchmaking unbind.
- Broadcasts `PLAYER_DEFEATED`.
- If match is over, finalizes stats and broadcasts `MATCH_ENDED`.

Monster death:

- Deletes monster from `match_monsters`.
- Clears monster from map cell.
- Broadcasts `UPDATE_CELL`.

### Damage Events and Stats

Runtime match state records:

- Damage events.
- Kill events.
- Defeated players.

Final stats are calculated in `beckend/gameservice/game/results.go` and persisted by `service.FinalizeMatch`.

XP visible in code:

- Monster damage: `damage / 5` gives `+1 XP` units.
- Player damage: `damage / 5` gives `+2 XP` units.

Rewards visible in code:

- Survived base coins: `20`.
- Player kills: `+20`.
- Monster kills: `+8`.
- Damage total bonus: `damageTotal / 50`.

### COMBAT_EXCHANGE

Payload:

```json
{
  "instanceId": "...",
  "exchangeId": "...",
  "attackerId": 1,
  "attackerType": "player",
  "targetId": 2,
  "targetType": "player",
  "attackStyle": "ranged",
  "steps": [],
  "effects": []
}
```

Step kinds found:

- `hit`
- `counter`
- `followup`
- `bonus`
- `auraExit`
- `death`

Effect kinds found:

- `armorBreak`
- `push`
- `energyDrain`
- `block`
- `lifesteal`
- `crit`
- `arcaneOverburn`
- `pureDamage`

Frontend state update:

- `gameSlice.applyCombatExchangeState` applies authoritative HP/death from steps.
- `combatPresentationSlice` queues exchange for animation.
- `useCombatPresentationPlayback` shows projectiles/lunges/hit flashes/floaters.
- `useCombatFloaters` suppresses legacy HP diff floaters for actors already handled by the exchange to prevent double damage floaters.

## 11. Class System Documentation

Stats come from `beckend/gameservice/repository/hero_stats.go`, `beckend/gameservice/handlers/players.go`, and `src/constants/characterArchetypes.ts`.

| Class | HP | Energy | Regen | Attack | Defense | Mobility | Reflex/agility | Sight | Type | Range |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | ---: |
| Guardian | 130 | 90 | 10 | 9 | 8 | 2 | 5 | 2 | Melee | 1 |
| Berserker | 100 | 100 | 10 | 14 | 3 | 3 | 5 | 2 | Melee | 1 |
| Ranger | 92 | 105 | 11 | 11 | 4 | 4 | 5 | 2 | Ranged | 2 |
| Mystic | 95 | 125 | 13 | 10 | 4 | 3 | 5 | 2 | Ranged | 3 |

### Guardian

Role: Frontline control/tank.

Attack type: Melee.

Energy behavior:

- Melee attack costs `6`.
- Movement cost is high because mobility is `2`.
- Counterattack is free.

Special mechanics:

- Guardian Shield Block via Reflex.
- Zone control movement penalty around enemy Guardian.
- Aura exit damage cap `5`.

Known limitations:

- Exact UI explanation of zone control is not fully clear from current UI code. `Needs verification`.

### Berserker

Role: High melee pressure.

Attack type: Melee.

Energy behavior:

- Melee attack costs `6`.
- Counterattack costs `2`.

Special mechanics:

- Bonus primary damage against wounded targets.
- Berserker Blood Feast via Reflex.
- Follow-up/fury after counterattack conditions, using half of primary hit damage as flat damage.

Known limitations:

- `berserkerFollowUpLimitPerTurn` is set to `0`, and code comments indicate `0 = unlimited follow-ups`. Balance should be tested after changes.

### Ranger

Role: Mobile ranged pressure and armor control.

Attack type: Ranged.

Energy behavior:

- Ranged attack costs `8`.
- Counterattack costs `2` when in melee.
- Successful third Armor Break push grants movement-cost-sized energy back to Ranger.

Special mechanics:

- Ranger Critical Shot via Reflex.
- Ranged hits apply Armor Break stacks.
- Third connected ranged hit at max stacks triggers push or fallback full attack and resets stacks.

Known limitations:

- Fallback full attack intentionally does not crit. Keep this covered by tests.

### Mystic

Role: Ranged tempo/energy control.

Attack type: Ranged.

Energy behavior:

- Ranged attack costs `8`.
- Energy drain restores some Mystic energy.
- Arcane Overburn restores more Mystic energy.

Special mechanics:

- Mystic drains target energy on successful hit.
- Arcane Overburn via Reflex increases burn/restoration.
- If target lacks energy, missing burn becomes pure HP damage.

Known limitations:

- Pure damage and block interaction should stay explicit in tests because it is easy to regress.

## 12. Reflex System Documentation

Important naming convention:

- Backend DB/API field is `agility`.
- UI displays the same value as `Reflex`.
- Do not rename database columns or API fields from `agility` to `reflex` unless there is a planned migration and compatibility layer.

Backend helper:

- `beckend/gameservice/internal/game/balance/formulas.go`

Frontend helper:

- `src/utils/reflex.ts`

Formula:

```go
chance := (30 * reflex) / (reflex + 10)
if chance > 25 {
    chance = 25
}
```

Examples:

| Reflex | Proc chance |
| ---: | ---: |
| 5 | 10% |
| 10 | 15% |
| 20 | 20% |
| 50 | 25% |

Class-specific Reflex effects:

| Class | Effect | Behavior |
| --- | --- | --- |
| Guardian | Shield Block | Chance to block incoming primary hit and emit `block`. |
| Berserker | Blood Feast | Chance to heal for part of actual primary damage dealt. |
| Ranger | Critical Shot | Chance to increase primary ranged hit damage to 142%. |
| Mystic | Arcane Overburn | Chance to increase energy burn/restoration. |

Frontend display:

- Registration class cards show Reflex and proc chance.
- `PlayerHUD` displays Reflex and chance.
- Profile displays class Reflex effect name/description.

## 13. Ranger Armor Break Documentation

Main file: `beckend/gameservice/handlers/combat.go`

Constants:

- Defense penalty per stack: `2`.
- Duration: `2` turns.
- Max stacks: `2`.

Rules:

- Only Ranger ranged hits apply Armor Break.
- Stack 1 applies `-2 defense`.
- Stack 2 applies another `-2 defense`.
- A third connected ranged hit while target already has 2 stacks triggers the third-hit effect and resets stacks.
- Third-hit effect tries to push the target one cell away from the attacker.
- If push succeeds:
  - target position is updated;
  - cell updates are broadcast;
  - Ranger gains energy equal to base movement cost, capped by max energy;
  - Armor Break resets.
- If push fails:
  - server applies fallback full attack damage;
  - fallback damage is emitted as a `bonus` step;
  - Armor Break resets.
- A hit that deals `0` damage because of defense still counts as a connected hit.
- Guardian Shield Block prevents the hit from counting as connected, so it prevents stack application.
- Fallback full attack does not crit.

Backend effects:

- Stack application emits `armorBreak`.
- Third-hit push emits `push`.
- Failed push may include `bonusDamage`.

Tests requested/visible:

```bash
cd beckend/gameservice
go test ./handlers -run 'TestRangerArmorBreak'
```

## 14. Base / Tavern / Heroes Documentation

### Base Buildings

Table: `player_base_buildings`

Columns:

- `user_id`
- `forge_level`
- `library_level`
- `tavern_level`
- `created_at`
- `updated_at`

Costs:

| Building | Wood | Stone | Iron |
| --- | ---: | ---: | ---: |
| Forge | 40 | 30 | 20 |
| Library | 30 | 45 | 25 |
| Tavern | 10 | 10 | 10 |

Resources are read from `player_profiles.inventory` keys like `resource_<id>`.

### Tavern

Tavern unlocks hero hiring.

Endpoint:

```text
POST /game/base/tavern/build
```

After Tavern is built, `/game/heroes` returns hireable heroes when the player has enough gold.

### Hero Catalog

Catalog is in `beckend/gameservice/handlers/heroes.go`.

Classes:

- `guardian`
- `berserker`
- `ranger`
- `mystic`

All catalog entries in inspected code:

- Enabled.
- Hireable.
- Require Tavern.
- Cost `2500` gold.

### Hire Hero Flow

```text
GET /game/heroes
  -> user sees locked/hireable state

POST /game/base/tavern/build
  -> tavern_level becomes 1

POST /game/heroes/{heroClassId}/hire
  -> check Tavern
  -> check balance >= 2500
  -> insert player_characters row with source='hire'
  -> deduct balance
  -> return updated hero state

POST /game/heroes/active
  -> check owned hero
  -> update player_profiles.selected_character_id
```

### Selected Active Hero

Current active hero is stored by `player_profiles.selected_character_id`.

`GetSelectedCharacterForUser()` resolves the selected character for match creation. `CreateMatchPlayerCopy()` snapshots that character into `match_players`.

### `player_characters`

`player_characters` is the current per-hero progression table. It stores:

- Hero class.
- Level/exp/max exp.
- Energy/HP/combat stats.
- Image.
- Source (`starter`, `hire`, etc.).

### Legacy Compatibility

Current code uses:

- `player_characters`
- `player_profiles.selected_character_id`
- `match_players.character_id`

Legacy names mentioned in tests/tasks:

- `selected_hero_class_id`
- `player_heroes`

Current tests suggest old structures should not exist in the new schema. `Needs verification`: production migration from any old `player_heroes` data is not fully clear from current code.

### Match Snapshot

At match creation:

```text
player_profiles.selected_character_id
  -> player_characters.id
  -> match_players.character_id
  -> match_players character_type/stats/exp snapshot
```

At finalization:

```text
match_player_stats and match_stats saved
character exp awarded through match_players.character_id
```

If `match_players.character_id` is missing, finalization logs an error and character XP may not be awarded. Keep this column required.

## 15. Database Documentation

Schema is created by Go code in `beckend/gameservice/repository/db.go` and related repository files. Seed/static data is also provided in `docker/postgres/init/02-game-seed.sql`.

### `users`

Purpose: Auth users.

Defined in: `beckend/auth/Auth.go`

Important columns:

- `id`
- `email`
- `password_hash`
- `name`
- `created_at`

Relationships:

- Game DB references user IDs conceptually, but auth and game tables are in separate DBs.

### `player_profiles`

Purpose: Persistent player profile and shared inventory/balance.

Important columns:

- `user_id`
- `name`
- `image`
- `balance`
- `inventory`
- `selected_character_id`
- `created_at`
- `updated_at`

Relationships:

- `selected_character_id` points to `player_characters.id`.
- Referenced by `player_characters`, `match_players`, friend tables, artifacts.

### `player_characters`

Purpose: Owned heroes and per-character progression.

Important columns:

- `id`
- `user_id`
- `hero_class_id`
- `image`
- `level`
- `exp`
- `max_exp`
- `max_energy`
- `max_health`
- `attack`
- `defense`
- `mobility`
- `agility`
- `sight_range`
- `is_ranged`
- `attack_range`
- `source`

Relationships:

- Unique `(user_id, hero_class_id)`.
- Used by `match_players.character_id`.

Migration notes:

- Current schema rejects empty hero class and `adventurer`.
- Replaces old `player_heroes` concept.

### `player_base_buildings`

Purpose: Persistent base building levels.

Important columns:

- `user_id`
- `forge_level`
- `library_level`
- `tavern_level`
- `created_at`
- `updated_at`

Relationships:

- `user_id` references `player_profiles`.

### `matches`

Purpose: Active match metadata.

Important columns:

- `instance_id`
- `mode`
- `teams_count`
- `total_players`
- `active_user_id`
- `winner_id`
- `winner_group_id`
- `turn_order`
- `turn_number`
- `start_positions`
- `portal_position`
- `map_width`
- `map_height`
- `map`
- `quest_artifact_id`

Relationships:

- Parent of `match_players`, `match_map_cells`, `match_monsters`.

### `match_players`

Purpose: Per-match snapshot of a selected character.

Important columns:

- `instance_id`
- `user_id`
- `character_id`
- `name`
- `character_type`
- `position`
- `inventory`
- `level`
- `energy`
- `max_energy`
- `health`
- `max_health`
- `experience`
- `max_experience`
- `attack`
- `defense`
- `mobility`
- `agility`
- `sight_range`
- `is_ranged`
- `attack_range`
- `balance`
- `image`
- `group_id`

Relationships:

- Composite primary key `(instance_id, user_id)`.
- `character_id` links the match snapshot back to `player_characters`.

### `match_map_cells`

Purpose: Persistent map cells for active matches.

Important columns:

- `instance_id`
- `cell_id`
- `x`
- `y`
- `tile_code`
- `resource`
- `barbel`
- `monster`
- `is_portal`
- `is_player`
- `structure_type`
- `structure_owner_user_id`
- `structure_health`
- `structure_defense`
- `structure_attack`
- `structure_energy`
- `is_under_construction`
- `construction_turns_left`

Relationships:

- Belongs to `matches`.

### `match_monsters`

Purpose: Runtime monster instances.

Important columns:

- `instance_id`
- `monster_instance_id`
- `monster_ref_id`
- `pos_x`
- `pos_y`
- `health`
- `max_health`
- `attack`
- `defense`
- `speed`
- `maneuverability`
- `vision`
- `image`

### `monsters`

Purpose: Static monster catalog.

Important columns:

- `id`
- `name`
- `type`
- `health`
- `max_health`
- `attack`
- `defense`
- `speed`
- `maneuverability`
- `vision`
- `image`

Seed examples: Goblin, Goblins, Orc, Troll.

### `resources`

Purpose: Static resource catalog.

Important columns:

- `id`
- `type`
- `description`
- `effect`
- `image`

Seed examples: food, water, wood, stone, iron, barrel.

`Needs verification`: Go migration creates `effect TEXT`, while seed/init SQL appears to handle JSONB in at least one path. The handler normalizes either raw JSON/string values, but the exact type in a running DB depends on initialization order/schema history.

### `artifacts`

Purpose: Static artifact catalog.

Important columns:

- `id`
- `name`
- `description`
- `bonus`
- `image`

Used for quest artifact selection and barrel/artifact drops.

### `inventory_items`

Purpose: Normalized inventory item rows for match inventory.

Important columns:

- `id`
- `instance_id`
- `user_id`
- `item_type`
- `item_id`
- `item_name`
- `image_url`
- `item_description`
- `item_count`
- `base_value`
- `npc_price`
- `durability`
- `acquired_at`
- `expires_at`

### `persisted_artifacts`

Purpose: Persistent artifacts outside match runtime.

Important columns:

- `id`
- `user_id`
- `artifact_type`
- `artifact_id`
- `description`
- `image`
- `base_value`
- `npc_price`
- `rarity`
- `durability`
- `acquired_at`
- `expires_at`

### `match_stats`

Purpose: Completed match summary/history.

Important columns:

- `instance_id`
- `mode`
- `winner_id`
- `winner_group_id`
- `winner_user_ids`
- `participants`
- `created_at`

Migration notes:

- `EnsureMatchStatsRetentionSchema` drops active-match FK so history remains after active match deletion.

### `match_player_stats`

Purpose: Per-player completed match stats.

Important columns:

- `instance_id`
- `user_id`
- `player_name`
- `group_id`
- `character_type`
- `is_winner`
- `survived`
- `deaths`
- `exp_gained`
- `rewards`
- `player_kills`
- `monster_kills`
- `damage_total`
- `damage_to_players`
- `damage_to_monsters`
- `damage_taken`
- `placement`
- `inventory_snapshot`

### `player_friends`

Purpose: Accepted friend relationships.

Important columns:

- `user_id`
- `friend_user_id`
- `created_at`

### `player_friend_requests`

Purpose: Pending friend requests.

Important columns:

- `requester_user_id`
- `target_user_id`
- `created_at`

### `match_player_structures`

Purpose: Per-player structure counts and bonuses in a match.

Important columns:

- `instance_id`
- `user_id`
- `scout_towers_count`
- `turrets_count`
- `walls_count`
- `total_buildings`
- `has_scout_tower_bonus`

## 16. Game State and Data Flow

### Login to Match

```text
Browser
  -> POST /auth/login
  <- JWT
  -> GET /auth/profile
  -> GET /game/profile
  -> GET /game/base/state
  -> GET /game/heroes
  -> POST /matchmaking/join
  -> GET /matchmaking/currentMatch polling
  -> GET /matchmaking/stream SSE
  <- MatchInfo
  -> /game?instance_id=...
```

### Match Initialization

```text
Matchmaking queue has enough players
  -> build teams/group IDs/turn order
  -> POST game service /game/createMatch
      -> generate map
      -> choose quest artifact
      -> insert matches
      -> insert match_monsters
      -> insert match_map_cells
      -> snapshot selected player_characters into match_players
      -> create runtime MatchState
      -> start turn timer
  -> matchmaking stores current match
  -> SSE notifies players
```

### WebSocket State Flow

```text
Game page loads
  -> GET /api/resources
  -> GET /api/monsters
  -> GET /game/match?instance_id=...
  -> open /ws?token=...&instanceId=...
  -> send JOIN_MATCH
  <- MATCH_UPDATE
  <- live UPDATE_* / COMBAT_EXCHANGE events
```

### Combat Flow

```text
Player clicks target
  -> usePlayerActions calls /game/attack
  -> backend validates turn and range
  -> backend computes combat
  -> backend saves match state
  -> backend broadcasts COMBAT_EXCHANGE
  -> frontend queues animation
  -> frontend applies authoritative state
  -> frontend logs relevant action
```

### Match End Flow

```text
Quest artifact holder reaches portal
  -> POST /game/finishMatch
  -> validate artifact
  -> SetMatchWinner
  -> collect MatchResults
  -> service.FinalizeMatch
      -> persist match_stats
      -> persist match_player_stats
      -> award rewards and character XP
      -> cleanup active match rows
  -> broadcast MATCH_ENDED
  -> frontend stores stats and routes to stats screen
```

## 17. Deployment / Docker / Nginx

Docker services:

| Service | Image/build | Host port | Notes |
| --- | --- | --- | --- |
| `postgres` | `postgres:16-alpine` | not published | DB inside `app_net`. |
| `postgres-bootstrap` | `postgres:16-alpine` | none | Ensures `admin` and `game_db`. |
| `adminer` | `adminer:4` | `8080` | DB UI. |
| `auth` | `docker/auth.Dockerfile` | via Nginx | Port `8000` inside network. |
| `gameservice` | `docker/gameservice.Dockerfile` | via Nginx | Port `8001` inside network. |
| `matchmaking` | `docker/matchmaking.Dockerfile` | via Nginx | Port `8002` inside network. |
| `frontend` | `docker/frontend.Dockerfile` | via Nginx | Port `3000` inside network. |
| `nginx` | `nginx:1.27-alpine` | `80` | Public entry point. |

Network:

- `app_net` bridge network.

Volume:

- `postgres_data`.

Nginx WebSocket proxy:

- Route: `/ws`.
- Uses `proxy_http_version 1.1`.
- Sets `Upgrade` and `Connection` headers.
- Long read/send timeouts.

Local/LAN/tunnel usage:

- Set `FRONTEND_ORIGIN` to include the browser origin.
- Keep frontend API envs relative if traffic goes through Nginx.
- If exposing through HTTPS tunnel, use `wss` for WebSocket. Relative `/ws` will work when page is served from the same tunnel origin.

Common 502 causes:

- Service container not started.
- Service crashed during DB migration.
- Nginx stale Docker DNS upstream.
- Wrong service name or port in Nginx.
- Database bootstrap did not finish.

Debug commands:

```bash
docker compose ps
docker compose logs nginx
docker compose logs auth
docker compose logs gameservice
docker compose logs matchmaking
docker compose logs postgres
```

Restart Nginx after upstream changes:

```bash
docker compose restart nginx
```

Reset database:

```bash
docker compose down -v
docker compose up --build
```

## 18. Testing Guide

General checks:

```bash
git diff --check
npm run build
```

Game service tests:

```bash
cd beckend/gameservice
go test ./...
```

Auth service tests:

```bash
cd beckend/auth
go test ./...
```

Matchmaking service tests:

```bash
cd beckend/matchmaking
go test ./...
```

Combat/reflex focused tests:

```bash
cd beckend/gameservice
go test ./handlers -run 'TestMysticArcaneOverburn|TestRangerCriticalShot|TestBerserkerBloodFeast|TestGuardianShieldBlock|TestRangerArmorBreak'
```

Recommended manual multiplayer verification:

1. Start with `docker compose up --build`.
2. Register two users with different classes.
3. Login in two browsers/incognito windows.
4. Build Tavern for a user with enough seeded resources/gold if testing hire flow.
5. Hire/select a hero.
6. Queue both users for `1x1`.
7. Confirm match-ready modal appears via SSE/polling.
8. Enter match and verify both WebSockets receive `MATCH_UPDATE`.
9. Move, attack, end turn.
10. Verify `COMBAT_EXCHANGE` action log and presentation effects.
11. Disconnect one browser and reconnect within grace period.
12. Finish match and verify `/game/stats` and `/matches`.

## 19. Troubleshooting

### Nginx 502

Symptoms:

- Browser shows Bad Gateway.
- `curl http://localhost/healthz/game` fails.

Likely causes:

- Target service not running.
- Game service crashed during migration.
- Nginx DNS/upstream cache issue.

Files to check:

- `deploy/nginx/nginx.conf`
- `docker-compose.yml`
- Service logs.

Commands:

```bash
docker compose ps
docker compose logs nginx
docker compose logs gameservice
docker compose restart nginx
```

Possible fix:

- Fix crashed service error, then restart Nginx.

### Wrong API Base URL

Symptoms:

- Frontend calls `localhost:3000/auth/login` in dev but backend is elsewhere.
- Browser CORS or 404 errors.

Likely causes:

- Incorrect `NEXT_PUBLIC_API_BASE`, `NEXT_PUBLIC_AUTH_BASE`, or `NEXT_PUBLIC_MATCHMAKING_BASE`.

Files to check:

- `example.env`
- `.env`
- `src/utils/serviceUrls.ts`

Possible fix:

- Use empty same-origin values with Nginx, or set full URLs for manual service mode.

### WebSocket Connection Failed

Symptoms:

- Match loads but live updates do not arrive.
- Browser console shows failed WebSocket handshake.

Likely causes:

- Bad `NEXT_PUBLIC_WS_URL`.
- Nginx `/ws` upgrade not working.
- JWT expired or wrong secret between auth/game service.

Files to check:

- `src/hooks/useGameSocket.ts`
- `src/utils/serviceUrls.ts`
- `deploy/nginx/nginx.conf`
- `beckend/gameservice/handlers/ws_handler.go`

Commands:

```bash
docker compose logs nginx
docker compose logs gameservice
```

Possible fix:

- Use `/ws` through same-origin Nginx.
- Confirm `JWT_SECRET_KEY` matches auth and game service.

### Frontend Cannot Reach Backend

Symptoms:

- Login/register/profile requests fail.
- Health routes work only inside Docker.

Likely causes:

- Backend ports are not published in Docker Compose.
- Frontend must use Nginx routes, not direct container names from browser.

Possible fix:

- Use `http://localhost/auth/...`, `http://localhost/game/...`, `http://localhost/matchmaking/...` through Nginx.

### Docker Database Not Initialized

Symptoms:

- Game service logs schema errors.
- Auth health check says users table missing.

Likely causes:

- Bootstrap failed.
- Old volume has incompatible schema.

Commands:

```bash
docker compose logs postgres-bootstrap
docker compose logs postgres
docker compose down -v
docker compose up --build
```

### Database Connection Failed

Symptoms:

- Service exits on startup.
- Logs mention DSN or connection refused.

Likely causes:

- Wrong `AUTH_DB_DSN` or `GAME_DB_DSN`.
- Manual run still uses Docker hostname `postgres`.

Possible fix:

- In Docker use `postgres`.
- In manual host mode use `localhost` or `127.0.0.1` with a published Postgres port.

### CORS / Origin Issues

Symptoms:

- Browser blocks auth calls.

Likely causes:

- `FRONTEND_ORIGIN` does not include current origin.

Files to check:

- `beckend/auth/Auth.go`
- `.env`

Possible fix:

- Add exact origin to `FRONTEND_ORIGIN`.

### Port Binding Issues

Symptoms:

- `docker compose up` fails binding `80` or `8080`.

Likely causes:

- Another service uses the port.

Commands:

```bash
docker compose ps
lsof -i :80
lsof -i :8080
```

Possible fix:

- Stop the conflicting process or change Compose host port.

### JWT Expired

Symptoms:

- Protected routes return 401/403.
- WebSocket closes or cannot connect.

Likely causes:

- JWT expiration is 6 hours.
- localStorage has old user data.

Files to check:

- `src/contexts/AuthContext.tsx`
- `beckend/auth/Auth.go`

Possible fix:

- Log out and log in again.

### Selected Hero Not Reflected

Symptoms:

- Tavern says active hero changed, but match starts with another class.

Likely causes:

- `player_profiles.selected_character_id` not updated.
- Match was created before active hero change.
- `match_players.character_id` snapshot is stale for already-created match.

Files to check:

- `beckend/gameservice/handlers/heroes.go`
- `beckend/gameservice/repository/hero_progression.go`
- `beckend/gameservice/repository/players.go`

Commands:

```bash
docker compose logs gameservice
```

Possible fix:

- Change active hero before joining queue.
- Verify selected character in DB before match creation.

### MaxExperience Shows Wrong Value

Symptoms:

- Profile or match HUD displays unexpected max XP.

Likely causes:

- Profile and match use character snapshot fields.
- Old match snapshots may not reflect latest character progression.

Files to check:

- `beckend/gameservice/repository/hero_progression.go`
- `beckend/gameservice/handlers/profile.go`
- `src/components/ProfilePage.tsx`
- `src/store/slices/gameSlice.ts`

Possible fix:

- Verify `player_characters.max_exp` and `match_players.max_experience`.

### Action Log Shows Other Players' Events

Symptoms:

- Action log includes irrelevant remote combat or resource events.

Likely causes:

- Relevance filter misses a payload field shape.

Files to check:

- `src/utils/actionLog.ts`
- `src/features/game/createWsHandlers.ts`

Possible fix:

- Add the missing payload field to `isCombatExchangeRelevantToCurrentPlayer` or `payloadMatchesCurrentPlayer`.

### Combat Double Damage Floater

Symptoms:

- Damage appears twice over a unit.

Likely causes:

- Both `COMBAT_EXCHANGE` presentation and legacy HP-diff floater show same damage.

Files to check:

- `src/hooks/useCombatPresentationPlayback.ts`
- `src/hooks/useCombatFloaters.ts`

Possible fix:

- Ensure actors in active exchange are included in suppression lists.

### Migration Mismatch

Symptoms:

- Code expects a column but DB does not have it.
- Schema readiness fails.

Likely causes:

- Old volume.
- Partially applied code-driven migration.

Files to check:

- `beckend/gameservice/repository/db.go`
- `beckend/gameservice/repository/base_buildings.go`
- `docker/postgres/init/02-game-seed.sql`

Commands:

```bash
docker compose down -v
docker compose up --build
```

Possible fix:

- For local dev, reset volume.
- For production, write a safe SQL migration and test against a copy.

## 20. Development Conventions

- Do not rename DB/API `agility` to `reflex` casually. UI may label it Reflex, but storage/API currently use `agility`.
- Backend is authoritative for combat, HP, energy, deaths, rewards, and stats.
- Frontend should display explicit combat effects sent by backend.
- Do not infer `BLOCK`, `CRIT`, `LIFESTEAL`, or `OVERBURN` from numeric HP differences.
- Keep combat effects explicit in `COMBAT_EXCHANGE.effects`.
- Write focused tests for balance changes and class mechanics.
- Do not change database schema without updating code-driven migrations.
- Keep API/DTO compatibility when possible, especially frontend field names like `instanceId`, `active_user`, `turnNumber`, and character stat fields.
- Keep active-match state and completed history separate.
- Do not rely on old `player_heroes` or `selected_hero_class_id` without verifying production data.
- Use same-origin relative frontend URLs when deploying through Nginx.
- Treat `match_players` as a snapshot; changing a persistent character after match creation should not mutate an active match unless explicitly designed.

## 21. Known Limitations / TODO

Known from current code:

- Equipment route exists but equipment functionality is not implemented in the inspected frontend.
- Matchmaking HTTP/SSE auth should be reviewed. Server code currently appears to trust `player_id`.
- Auth registration can leave an auth user without a game profile if the game service call fails. Needs transactional or compensating behavior.
- `resources.effect` schema has possible TEXT/JSONB mismatch depending on migration/init path. Needs verification.
- Gameservice CORS uses `*`; production policy should be tightened.
- Artifact HTTP routes are registered without auth middleware in `cmd/main.go`. Needs verification before production exposure.
- Some Library unlock descriptions may not map to implemented gameplay bonuses. Needs verification.
- Combat and Reflex balance values are hard-coded; future changes should be test-backed.
- Action log filtering exists, but new event payload shapes can bypass relevance filtering if not added.
- Combat presentation has explicit suppression for duplicate floaters, but any new damage event path should be tested against duplicate UI.
- Production migration from any old hero schema is not fully clear from current code.

Future improvements visible from project direction:

- Finish equipment system.
- Add stronger matchmaking/auth authorization.
- Add explicit SQL migrations instead of relying only on code-driven schema changes.
- Add more automated integration tests for registration -> profile -> matchmaking -> match -> finish.
- Expand manual/admin tooling for inspecting active matches and match history.
- Improve combat presentation polish after every new combat effect.
- Continue balance tuning with tests for Guardian, Berserker, Ranger, Mystic.

## Documentation Gaps / Needs Verification

- Whether production databases ever contained `player_heroes` or `selected_hero_class_id`, and if a one-time migration exists outside the repository.
- Whether `resources.effect` should be standardized as JSONB or TEXT.
- Whether matchmaking routes must validate JWT before accepting `player_id`.
- Whether unauthenticated artifact routes are intentional.
- Whether `/game/page.tsx` double Redux Provider is intentional or leftover.
- Whether all Library unlockables returned by `/game/base/state` are implemented in gameplay.
- Exact production deployment topology outside Docker Compose/Nginx in this repo.
- Any roadmap items not represented in code, such as chat, trading, collections, or broader equipment design.
