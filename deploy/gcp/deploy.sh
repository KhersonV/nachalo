#!/usr/bin/env bash
set -euo pipefail

if ! command -v gcloud >/dev/null 2>&1; then
  GCLOUD_BIN_DIR="/c/Users/User/AppData/Local/Google/Cloud SDK/google-cloud-sdk/bin"
  if [[ -x "${GCLOUD_BIN_DIR}/gcloud" ]]; then
    export PATH="${GCLOUD_BIN_DIR}:${PATH}"
  fi
fi

if ! command -v gcloud >/dev/null 2>&1; then
  echo "Error: gcloud CLI not found in PATH."
  echo "Install Google Cloud CLI first, then reopen terminal."
  echo "Docs: https://cloud.google.com/sdk/docs/install"
  exit 127
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Load deploy variables from deploy/gcp/.env if present.
ENV_FILE="${SCRIPT_DIR}/.env"
if [[ -f "${ENV_FILE}" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
  set +a
fi

# 1) Configuration from environment (with safe defaults for non-sensitive fields).
PROJECT_ID="${PROJECT_ID:-nachalo-20260313193607}"
REGION="${REGION:-europe-west1}"
REPO="${REPO:-nachalo-repo}"
SQL_INSTANCE="${SQL_INSTANCE:-nachalo-pg}"
DB_USER="${DB_USER:-admin}"
DB_PASSWORD="${DB_PASSWORD:-}"
JWT_SECRET="${JWT_SECRET:-}"
TAG="${TAG:-latest}"
SQL_TIER="${SQL_TIER:-db-n1-standard-1}"
SQL_CONN_NAME="${PROJECT_ID}:${REGION}:${SQL_INSTANCE}"

# Service names in Cloud Run
AUTH_SERVICE="auth"
GAME_SERVICE="gameservice"
MATCHMAKING_SERVICE="matchmaking"
FRONTEND_SERVICE="frontend"

MODE="${1:-full}"
case "${MODE}" in
  full|auth|gameservice|matchmaking|frontend|restart) ;;
  *)
    echo "Usage: bash deploy/gcp/deploy.sh [full|auth|gameservice|matchmaking|frontend|restart]"
    exit 1
    ;;
esac

# 2) Guardrails
if [[ -z "${DB_PASSWORD}" || -z "${JWT_SECRET}" || "${PROJECT_ID}" == "REPLACE_PROJECT_ID" || "${DB_PASSWORD}" == "REPLACE_DB_PASSWORD" || "${JWT_SECRET}" == "REPLACE_LONG_RANDOM_JWT_SECRET" ]]; then
  echo "Missing required deploy env vars."
  echo "Set PROJECT_ID, DB_PASSWORD and JWT_SECRET in your shell or in deploy/gcp/.env"
  echo "Tip: copy deploy/gcp/.env.example to deploy/gcp/.env and fill real values."
  exit 1
fi

# Escape PostgreSQL conninfo values for DSN fields like password='...'
escape_pg_conninfo_value() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\'/\\\'}"
  printf "%s" "${value}"
}

cd "${ROOT_DIR}"

gcloud config set project "${PROJECT_ID}"

echo "==> Check project billing"
BILLING_ENABLED="$(gcloud beta billing projects describe "${PROJECT_ID}" --format='value(billingEnabled)' 2>/dev/null || true)"
if [[ "${BILLING_ENABLED}" != "True" && "${BILLING_ENABLED}" != "true" ]]; then
  echo "Billing is NOT enabled for project ${PROJECT_ID}."
  echo "Open this page and link a billing account:"
  echo "https://console.cloud.google.com/billing/linkedaccount?project=${PROJECT_ID}"
  echo "Then rerun: bash deploy/gcp/deploy.sh"
  exit 1
fi

echo "==> Enable required services"
gcloud services enable \
  run.googleapis.com \
  sqladmin.googleapis.com \
  artifactregistry.googleapis.com \
  cloudbuild.googleapis.com \
  secretmanager.googleapis.com

echo "==> Create Artifact Registry (if missing)"
if ! gcloud artifacts repositories describe "${REPO}" --location "${REGION}" >/dev/null 2>&1; then
  gcloud artifacts repositories create "${REPO}" \
    --repository-format=docker \
    --location="${REGION}"
fi

gcloud auth configure-docker "${REGION}-docker.pkg.dev" -q

echo "==> Create Cloud SQL instance/database/user (if missing)"
if ! gcloud sql instances describe "${SQL_INSTANCE}" >/dev/null 2>&1; then
  gcloud sql instances create "${SQL_INSTANCE}" \
    --database-version=POSTGRES_16 \
    --region="${REGION}" \
    --tier="${SQL_TIER}" \
    --storage-size=20
fi

if ! gcloud sql users list --instance "${SQL_INSTANCE}" --format='value(name)' | grep -qx "${DB_USER}"; then
  gcloud sql users create "${DB_USER}" --instance "${SQL_INSTANCE}" --password "${DB_PASSWORD}"
else
  gcloud sql users set-password "${DB_USER}" --instance "${SQL_INSTANCE}" --password "${DB_PASSWORD}"
fi

if ! gcloud sql databases list --instance "${SQL_INSTANCE}" --format='value(name)' | grep -qx "admin"; then
  gcloud sql databases create admin --instance "${SQL_INSTANCE}"
fi

if ! gcloud sql databases list --instance "${SQL_INSTANCE}" --format='value(name)' | grep -qx "game_db"; then
  gcloud sql databases create game_db --instance "${SQL_INSTANCE}"
fi

echo "==> Create/rotate JWT secret"
if gcloud secrets describe nachalo-jwt >/dev/null 2>&1; then
  printf "%s" "${JWT_SECRET}" | gcloud secrets versions add nachalo-jwt --data-file=- >/dev/null
else
  printf "%s" "${JWT_SECRET}" | gcloud secrets create nachalo-jwt --data-file=- >/dev/null
fi

echo "==> Grant secret access to Cloud Run runtime service account"
PROJECT_NUMBER="$(gcloud projects describe "${PROJECT_ID}" --format='value(projectNumber)')"
RUNTIME_SA="${PROJECT_NUMBER}-compute@developer.gserviceaccount.com"
gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:${RUNTIME_SA}" \
  --role="roles/secretmanager.secretAccessor" >/dev/null

AUTH_IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/auth:${TAG}"
GAME_IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/gameservice:${TAG}"
MATCH_IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/matchmaking:${TAG}"
FRONT_IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/frontend:${TAG}"

DB_PASSWORD_ESCAPED="$(escape_pg_conninfo_value "${DB_PASSWORD}")"
GAME_DB_DSN="host=/cloudsql/${SQL_CONN_NAME} user=${DB_USER} password='${DB_PASSWORD_ESCAPED}' dbname=game_db sslmode=disable"
AUTH_DB_DSN="host=/cloudsql/${SQL_CONN_NAME} user=${DB_USER} password='${DB_PASSWORD_ESCAPED}' dbname=admin sslmode=disable"

service_url() {
  gcloud run services describe "$1" --region "${REGION}" --format='value(status.url)' 2>/dev/null || true
}

build_single_service() {
  local service="$1"
  local cfg
  cfg="$(mktemp)"

  case "${service}" in
    auth)
      cat > "${cfg}" <<EOF
steps:
  - name: gcr.io/cloud-builders/docker
    args: ["build", "-f", "docker/auth.Dockerfile", "-t", "${AUTH_IMAGE}", "."]
images: ["${AUTH_IMAGE}"]
EOF
      ;;
    gameservice)
      cat > "${cfg}" <<EOF
steps:
  - name: gcr.io/cloud-builders/docker
    args: ["build", "-f", "docker/gameservice.Dockerfile", "-t", "${GAME_IMAGE}", "."]
images: ["${GAME_IMAGE}"]
EOF
      ;;
    matchmaking)
      cat > "${cfg}" <<EOF
steps:
  - name: gcr.io/cloud-builders/docker
    args: ["build", "-f", "docker/matchmaking.Dockerfile", "-t", "${MATCH_IMAGE}", "."]
images: ["${MATCH_IMAGE}"]
EOF
      ;;
    frontend)
      local game_url auth_url match_url game_ws_url
      game_url="$(service_url "${GAME_SERVICE}")"
      auth_url="$(service_url "${AUTH_SERVICE}")"
      match_url="$(service_url "${MATCHMAKING_SERVICE}")"
      if [[ -z "${game_url}" || -z "${auth_url}" || -z "${match_url}" ]]; then
        echo "Error: frontend build requires existing ${GAME_SERVICE}, ${AUTH_SERVICE}, ${MATCHMAKING_SERVICE} services."
        rm -f "${cfg}"
        exit 1
      fi
      game_ws_url="${game_url/https:/wss:}/ws"
      cat > "${cfg}" <<EOF
steps:
  - name: gcr.io/cloud-builders/docker
    args:
      - build
      - -f
      - docker/frontend.Dockerfile
      - --build-arg
      - NEXT_PUBLIC_API_BASE=${game_url}
      - --build-arg
      - NEXT_PUBLIC_WS_URL=${game_ws_url}
      - --build-arg
      - NEXT_PUBLIC_AUTH_BASE=${auth_url}
      - --build-arg
      - NEXT_PUBLIC_MATCHMAKING_BASE=${match_url}
      - -t
      - ${FRONT_IMAGE}
      - .
images: ["${FRONT_IMAGE}"]
EOF
      ;;
  esac

  gcloud builds submit --config "${cfg}"
  rm -f "${cfg}"
}

deploy_gameservice() {
  local match_url="$1"
  local front_url="$2"
  gcloud run deploy "${GAME_SERVICE}" \
    --image "${GAME_IMAGE}" \
    --region "${REGION}" \
    --allow-unauthenticated \
    --port 8001 \
    --add-cloudsql-instances "${SQL_CONN_NAME}" \
    --set-env-vars "GAME_DB_DSN=${GAME_DB_DSN},MATCHMAKING_URL=${match_url},FRONTEND_ORIGIN=${front_url}" \
    --set-secrets "JWT_SECRET_KEY=nachalo-jwt:latest"
}

deploy_auth() {
  local game_url="$1"
  local front_url="$2"
  gcloud run deploy "${AUTH_SERVICE}" \
    --image "${AUTH_IMAGE}" \
    --region "${REGION}" \
    --allow-unauthenticated \
    --port 8000 \
    --add-cloudsql-instances "${SQL_CONN_NAME}" \
    --set-env-vars "AUTH_DB_DSN=${AUTH_DB_DSN},GAME_SERVICE_URL=${game_url},FRONTEND_ORIGIN=${front_url}" \
    --set-secrets "JWT_SECRET_KEY=nachalo-jwt:latest"
}

deploy_matchmaking() {
  local game_url="$1"
  gcloud run deploy "${MATCHMAKING_SERVICE}" \
    --image "${MATCH_IMAGE}" \
    --region "${REGION}" \
    --allow-unauthenticated \
    --port 8002 \
    --add-cloudsql-instances "${SQL_CONN_NAME}" \
    --set-env-vars "GAME_SERVICE_URL=${game_url},GAME_DB_DSN=${GAME_DB_DSN}"
}

deploy_frontend() {
  gcloud run deploy "${FRONTEND_SERVICE}" \
    --image "${FRONT_IMAGE}" \
    --region "${REGION}" \
    --allow-unauthenticated \
    --port 3000
}

if [[ "${MODE}" == "restart" ]]; then
  echo "==> Restart all Cloud Run services (no image rebuild)"

  GAME_URL="$(service_url "${GAME_SERVICE}")"
  MATCH_URL="$(service_url "${MATCHMAKING_SERVICE}")"
  AUTH_URL="$(service_url "${AUTH_SERVICE}")"
  FRONT_URL="$(service_url "${FRONTEND_SERVICE}")"

  MATCH_URL="${MATCH_URL:-https://${MATCHMAKING_SERVICE}-REPLACE.a.run.app}"
  GAME_URL="${GAME_URL:-https://${GAME_SERVICE}-REPLACE.a.run.app}"
  AUTH_URL="${AUTH_URL:-https://${AUTH_SERVICE}-REPLACE.a.run.app}"
  FRONT_URL="${FRONT_URL:-https://placeholder.invalid}"

  deploy_gameservice "${MATCH_URL}" "${FRONT_URL}"
  GAME_URL="$(service_url "${GAME_SERVICE}")"
  deploy_matchmaking "${GAME_URL}"
  MATCH_URL="$(service_url "${MATCHMAKING_SERVICE}")"
  deploy_auth "${GAME_URL}" "${FRONT_URL}"
  deploy_frontend

  FRONT_URL="$(service_url "${FRONTEND_SERVICE}")"
  AUTH_URL="$(service_url "${AUTH_SERVICE}")"
  GAME_URL="$(service_url "${GAME_SERVICE}")"
  MATCH_URL="$(service_url "${MATCHMAKING_SERVICE}")"

  echo "\nRestart complete:"
  echo "Frontend:    ${FRONT_URL}"
  echo "Auth:        ${AUTH_URL}"
  echo "Gameservice: ${GAME_URL}"
  echo "Matchmaking: ${MATCH_URL}"
  exit 0
fi

if [[ "${MODE}" == "auth" ]]; then
  echo "==> Build auth image"
  build_single_service auth
  GAME_URL="$(service_url "${GAME_SERVICE}")"
  FRONT_URL="$(service_url "${FRONTEND_SERVICE}")"
  GAME_URL="${GAME_URL:-https://${GAME_SERVICE}-REPLACE.a.run.app}"
  FRONT_URL="${FRONT_URL:-https://placeholder.invalid}"
  echo "==> Deploy auth"
  deploy_auth "${GAME_URL}" "${FRONT_URL}"
  exit 0
fi

if [[ "${MODE}" == "gameservice" ]]; then
  echo "==> Build gameservice image"
  build_single_service gameservice
  MATCH_URL="$(service_url "${MATCHMAKING_SERVICE}")"
  FRONT_URL="$(service_url "${FRONTEND_SERVICE}")"
  MATCH_URL="${MATCH_URL:-https://${MATCHMAKING_SERVICE}-REPLACE.a.run.app}"
  FRONT_URL="${FRONT_URL:-https://placeholder.invalid}"
  echo "==> Deploy gameservice"
  deploy_gameservice "${MATCH_URL}" "${FRONT_URL}"
  exit 0
fi

if [[ "${MODE}" == "matchmaking" ]]; then
  echo "==> Build matchmaking image"
  build_single_service matchmaking
  GAME_URL="$(service_url "${GAME_SERVICE}")"
  GAME_URL="${GAME_URL:-https://${GAME_SERVICE}-REPLACE.a.run.app}"
  echo "==> Deploy matchmaking"
  deploy_matchmaking "${GAME_URL}"
  exit 0
fi

if [[ "${MODE}" == "frontend" ]]; then
  echo "==> Build frontend image"
  build_single_service frontend
  echo "==> Deploy frontend"
  deploy_frontend
  exit 0
fi

echo "==> Build images via Cloud Build"
gcloud builds submit \
  --config deploy/gcp/cloudbuild.yaml \
  --substitutions "_REGION=${REGION},_REPO=${REPO},_TAG=${TAG},_API_BASE=https://${GAME_SERVICE}-REPLACE.a.run.app,_WS_URL=wss://${GAME_SERVICE}-REPLACE.a.run.app/ws,_AUTH_BASE=https://${AUTH_SERVICE}-REPLACE.a.run.app,_MATCHMAKING_BASE=https://${MATCHMAKING_SERVICE}-REPLACE.a.run.app"

echo "==> Deploy gameservice"
deploy_gameservice "https://${MATCHMAKING_SERVICE}-REPLACE.a.run.app" "https://placeholder.invalid"

GAME_URL="$(gcloud run services describe "${GAME_SERVICE}" --region "${REGION}" --format='value(status.url)')"
GAME_WS_URL="${GAME_URL/https:/wss:}/ws"

echo "==> Deploy matchmaking"
deploy_matchmaking "${GAME_URL}"

MATCH_URL="$(gcloud run services describe "${MATCHMAKING_SERVICE}" --region "${REGION}" --format='value(status.url)')"

echo "==> Deploy auth"
deploy_auth "${GAME_URL}" "https://placeholder.invalid"

AUTH_URL="$(gcloud run services describe "${AUTH_SERVICE}" --region "${REGION}" --format='value(status.url)')"

echo "==> Rebuild frontend image with real backend URLs"
gcloud builds submit \
  --config deploy/gcp/cloudbuild.yaml \
  --substitutions "_REGION=${REGION},_REPO=${REPO},_TAG=${TAG},_API_BASE=${GAME_URL},_WS_URL=${GAME_WS_URL},_AUTH_BASE=${AUTH_URL},_MATCHMAKING_BASE=${MATCH_URL}"

echo "==> Deploy frontend"
deploy_frontend

FRONT_URL="$(gcloud run services describe "${FRONTEND_SERVICE}" --region "${REGION}" --format='value(status.url)')"

echo "==> Update backend CORS origins to real frontend URL"
deploy_gameservice "${MATCH_URL}" "${FRONT_URL}" >/dev/null

deploy_auth "${GAME_URL}" "${FRONT_URL}" >/dev/null

echo "\nDeployment complete:"
echo "Frontend:    ${FRONT_URL}"
echo "Auth:        ${AUTH_URL}"
echo "Gameservice: ${GAME_URL}"
echo "Matchmaking: ${MATCH_URL}"

echo "\nIMPORTANT:"
echo "1) Create Cloud SQL instance/databases/users and run SQL init scripts before first game launch."
echo "2) Replace REPLACE placeholders in this script if any remain."
