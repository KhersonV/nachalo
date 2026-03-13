# Deploy to Google Cloud (Cloud Run + Cloud SQL)

This folder contains a deployment workflow for this project on GCP.

## Files

- `deploy/gcp/deploy.sh`: main deploy script.
- `deploy/gcp/cloudbuild.yaml`: build pipeline for images.
- `deploy/gcp/.env.example`: template for deployment variables.

## 1) Setup

1. Install Google Cloud CLI and login.
2. Ensure `gcloud` is in PATH.

Windows Git Bash example:

```bash
export PATH="/c/Users/User/AppData/Local/Google/Cloud SDK/google-cloud-sdk/bin:$PATH"
gcloud --version
```

## 2) Configure env

Create local deploy env file (gitignored):

```bash
cp deploy/gcp/.env.example deploy/gcp/.env
```

Fill real values in `deploy/gcp/.env`:

- `PROJECT_ID`
- `DB_PASSWORD`
- `JWT_SECRET`

Optional defaults are already provided for:

- `REGION`
- `REPO`
- `SQL_INSTANCE`
- `DB_USER`
- `TAG`
- `SQL_TIER`

## 3) Deploy modes

### Full deploy (build + deploy all)

```bash
bash deploy/gcp/deploy.sh full
```

`full` is the default mode, so this is equivalent:

```bash
bash deploy/gcp/deploy.sh
```

### Deploy a single service

```bash
bash deploy/gcp/deploy.sh frontend
bash deploy/gcp/deploy.sh auth
bash deploy/gcp/deploy.sh gameservice
bash deploy/gcp/deploy.sh matchmaking
```

### Fast restart of all Cloud Run services (no rebuild)

```bash
bash deploy/gcp/deploy.sh restart
```

This is the closest cloud equivalent to `docker compose down && up`.

## 4) What full deploy does

1. Validates env vars.
2. Enables required APIs.
3. Ensures Artifact Registry exists.
4. Ensures Cloud SQL instance/user/databases exist.
5. Rotates Secret Manager secret `nachalo-jwt`.
6. Builds/pushes images.
7. Deploys `gameservice`, `matchmaking`, `auth`, `frontend`.
8. Updates backend CORS origin with actual frontend URL.

## 5) Database access

### Cloud SQL from terminal

```bash
export PATH="/c/Users/User/AppData/Local/Google/Cloud SDK/google-cloud-sdk/bin:$PATH"
PGPASSWORD='YOUR_DB_PASSWORD' gcloud sql connect nachalo-pg \
    --user=admin \
    --database=game_db \
    --project=YOUR_PROJECT_ID
```

### Cloud SQL from console

Google Cloud Console -> SQL -> instance -> Query editor.

### Local DB (docker)

Adminer is available at `http://localhost:8080`.

## 6) Troubleshooting

- `gcloud: command not found`
    - Add Cloud SDK `bin` to PATH in current shell.
- CORS/auth issues
    - Re-deploy `auth` and `frontend`.
- Gameplay data errors (`/api/resources`)
    - Re-deploy `gameservice` and verify DB schema/seed.

## 7) Security

- Never commit `deploy/gcp/.env`.
- Keep secrets only in env / Secret Manager.
- Rotate `DB_PASSWORD` and `JWT_SECRET` if leaked.
