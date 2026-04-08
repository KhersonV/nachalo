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

It is recommended to run the project in 4 separate terminals.

### 1) Frontend

From the project root:

```bash
npm install
npm run dev
```

Frontend will be available at `http://localhost:3000`.

### 2) Auth service

```bash
cd beckend/auth
go mod tidy
go run Auth.go
```

Service will run at `http://localhost:8000`.

### 3) Game service

```bash
cd beckend/gameservice
go mod tidy
go run ./cmd
```

Service will run at `http://localhost:8001`.

### 4) Matchmaking service

```bash
cd beckend/matchmaking
go mod tidy
go run Match_Making.go
```

Service will run at `http://localhost:8002`.

## Database

In the current codebase, PostgreSQL connection strings are hardcoded in:

- `beckend/auth/Auth.go`
- `beckend/gameservice/repository/db.go`

Before first run, make sure required databases/users exist and `connStr` values match your local setup.

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
    - `POST /game/{instance_id}/player/{id}/move`
    - `POST /game/attack`
    - `POST /game/endTurn`
    - `GET /ws` (WebSocket)

## Tests

`gameservice` includes unit tests for game logic and handlers.

Run:

```bash
cd beckend/gameservice
go test ./...
```

## Current Status

README is updated to reflect the actual project structure and local startup flow for all services.
