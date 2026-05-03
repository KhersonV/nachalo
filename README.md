# Nachalo

Multi-service game project:

- frontend built with `Next.js + React + Redux Toolkit`;
- backend built with `Go` and split into 3 services: `auth`, `gameservice`, `matchmaking`;
- data storage in `PostgreSQL`;
- real-time match updates via `WebSocket`.

## What This Project Does

The application allows you to:

- register and authenticate players;
- choose a game mode (PvE/PvP) and join matchmaking queues;
- automatically create a match through the matchmaking service;
- play on the map (turns, attacks, resources, inventory, artifacts);
- receive match events in real time.

## Architecture

Services interact as follows:

- `frontend (localhost:3000)` -> `auth (localhost:8000)` for sign-in/sign-up;
- `frontend (localhost:3000)` -> `matchmaking (localhost:8002)` for queues and `instance_id` lookup;
- `frontend (localhost:3000)` -> `gameservice (localhost:8001)` for match logic and WebSocket;
- `auth` calls `gameservice` during registration to create a game character;
- `matchmaking` calls `gameservice` when enough players are found to create a match.

## Stack

- Frontend: `Next.js 15`, `React 19`, `TypeScript`, `Redux Toolkit`.
- Backend: `Go 1.24`, `gorilla/mux`, `gorilla/websocket`, `JWT`.
- Database: `PostgreSQL`.

## Project Structure

- `src/` - frontend (pages, components, hooks, store).
- `beckend/auth/` - authentication service.
- `beckend/gameservice/` - core game service.
- `beckend/matchmaking/` - queue and matchmaking service.
- `public/` - static game assets (monsters, artifacts, etc.).

## Requirements

- `Node.js` 20+ (LTS recommended)
- `npm` 10+
- `Go` 1.24+
- `PostgreSQL` 14+

## Environment Variables

Minimum required:

- `JWT_SECRET_KEY` (required by `auth` and `gameservice`).

Optional frontend overrides for service addresses:

- `NEXT_PUBLIC_API_BASE` (default: `http://localhost:8001`)
- `NEXT_PUBLIC_WS_URL` (default: `ws://localhost:8001/ws`)

Example for Windows PowerShell:

```powershell
$env:JWT_SECRET_KEY="super_secret_key"
```

## Local Run

### Docker Compose

From a fresh clone, the full application can be started with:

```bash
docker compose up -d --build
```

Compose starts PostgreSQL first, runs `postgres-bootstrap` to ensure both `admin` and `game_db` exist, and only then starts the backend services.

Frontend will be available at `http://localhost`.

Adminer for visual database access will be available at `http://localhost:8080`.
Use:

- System: `PostgreSQL`
- Server: `postgres`
- Username: `admin`
- Password: `admin`
- Database: `admin` or `game_db`

### Manual Run

It is recommended to run the project in 4 separate terminals.

#### 1) Frontend

From the project root:

```bash
npm install
npm run dev
```

Frontend will be available at `http://localhost:3000`.
For devices in the same local network, open `http://<your-computer-ip>:3000`.
The frontend now automatically swaps `localhost` service URLs to the host machine IP when opened from another device.

#### 2) Auth service

```bash
cd beckend/auth
go mod tidy
go run Auth.go
```

Service will run at `http://localhost:8000`.

#### 3) Game service

```bash
cd beckend/gameservice
go mod tidy
go run ./cmd
```

Service will run at `http://localhost:8001`.

#### 4) Matchmaking service

```bash
cd beckend/matchmaking
go mod tidy
go run Match_Making.go
```

Service will run at `http://localhost:8002`.

## Database

Docker Compose reads PostgreSQL connection strings from `.env`:

- `AUTH_DB_DSN=postgres://admin:admin@postgres:5432/admin?sslmode=disable`
- `GAME_DB_DSN=postgres://admin:admin@postgres:5432/game_db?sslmode=disable`

For manual runs without Docker Compose, create the matching databases/users yourself or point these variables at your local PostgreSQL instance.

## Main API (Short)

- Auth:
    - `POST /auth/register`
    - `POST /auth/login`
    - `GET /auth/profile`
- Matchmaking:
    - `POST /matchmaking/join`
    - `POST /matchmaking/cancel`
    - `GET /matchmaking/currentMatch?player_id=...`
- Game:
    - `POST /game/createMatch`
    - `GET /game/match?instance_id=...`
    - `GET /game/equipment`
    - `POST /game/equipment/equip`
    - `POST /game/equipment/unequip`
    - `POST /game/equipment/sell`
    - `POST /game/equipment/discard`
    - `POST /game/equipment/grant-sagecloth-dev` (local/dev only)
    - `POST /game/equipment/grant-item-dev` (local/dev only)
    - `GET /game/match/{instance_id}/my-loot`
    - `POST /game/{instance_id}/player/{id}/move`
    - `POST /game/attack`
    - `POST /game/endTurn`
    - `GET /ws` (WebSocket)

## Equipment Local Testing

The equipment dev grant endpoints are only available when the game service is
started with one of these local/testing flags:

```bash
EQUIPMENT_DEV_GRANT_ENABLED=true
# or
APP_ENV=development
```

Show the frontend test button with:

```bash
NEXT_PUBLIC_EQUIPMENT_DEV_GRANT_ENABLED=true
```

For local drop testing, force monster equipment drops by starting the game
service with:

```bash
EQUIPMENT_DROP_CHANCE_OVERRIDE=100
```

The override accepts `0` to `100` and uses the default `10%` when empty.

`TOKEN` is the auth JWT from login/localStorage.

Grant the full Mystic Sagecloth Set:

```bash
curl -i -X POST "http://localhost/game/equipment/grant-sagecloth-dev" \
  -H "Authorization: Bearer $TOKEN"
```

Grant one item by template code:

```bash
curl -i -X POST "http://localhost/game/equipment/grant-item-dev" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"templateCode":"sagecloth_staff"}'
```

Other Sagecloth template codes:

```bash
sagecloth_jacket
sagecloth_pants
sagecloth_boots
sagecloth_gloves
sagecloth_hood
```

Monster equipment drops:

- when a monster dies, roll equipment drop chance;
- choose a random green starter-set item from the global pool:
  `aegiswarden_set`, `bloodroot_set`, `greenwisp_set`, or `sagecloth_set`;
- do not limit the drop by the killer's current class;
- allow duplicate templates by creating a fresh UUID `item_instances` row;
- set `owner_user_id` to the killer user id;
- use `source = 'drop'`;
- write `item_instance_events.created`;
- broadcast `EQUIPMENT_DROPPED` and show a live action-log message.

Manual monster-drop test:

1. Optionally set `EQUIPMENT_DROP_CHANCE_OVERRIDE=100`.
2. Start the app and log in.
3. Start PvE and kill monsters.
4. If a drop happens, the action log should show `Loot found: ...`.
5. Open `/equipment`.
6. Confirm dropped items appear as separate item instances, including duplicates.
7. Equip compatible items and confirm stats change.

## Tests

`gameservice` includes unit tests for game logic and handlers.

Run:

```bash
cd beckend/gameservice
go test ./...
```

## Current Status

README is updated to reflect the actual project structure and local startup flow for all services.
