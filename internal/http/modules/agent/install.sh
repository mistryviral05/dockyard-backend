#!/usr/bin/env bash
set -euo pipefail

SERVER="__SERVER_URL__"
AGENT_IMAGE="__AGENT_IMAGE__"
ACME_EMAIL="__ACME_EMAIL__"

TOKEN=""
PORT="8999"
NETWORK="web"
STATE_DIR="/var/lib/dockyard"
STATE_FILE="$STATE_DIR/agent.json"

RED=$'\033[31m'
GREEN=$'\033[32m'
YELLOW=$'\033[33m'
BOLD=$'\033[1m'
RESET=$'\033[0m'

say()  { printf '%s==>%s %s\n' "$GREEN" "$RESET" "$1"; }
warn() { printf '%s==>%s %s\n' "$YELLOW" "$RESET" "$1"; }
die()  { printf '%s==> %s%s\n' "$RED" "$1" "$RESET" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --token)  TOKEN="${2:-}"; shift 2 ;;
    --server) SERVER="${2:-}"; shift 2 ;;
    --port)   PORT="${2:-}"; shift 2 ;;
    *) die "unknown option: $1" ;;
  esac
done

[ -n "$TOKEN" ] || die "--token is required"
[ "$(id -u)" -eq 0 ] || die "run this as root, or with sudo"

printf '\n%sDockyard agent installer%s\n\n' "$BOLD" "$RESET"

# ----------------------------------------------------------------- packages

if ! command -v curl >/dev/null 2>&1 || ! command -v jq >/dev/null 2>&1; then
  say "installing curl and jq"
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update -qq
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq curl jq >/dev/null
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y -q curl jq >/dev/null
  elif command -v yum >/dev/null 2>&1; then
    yum install -y -q curl jq >/dev/null
  else
    die "could not find apt, dnf or yum, install curl and jq by hand first"
  fi
fi

# ----------------------------------------------------------------- docker

if ! command -v docker >/dev/null 2>&1; then
  say "installing docker"
  curl -fsSL https://get.docker.com | sh >/dev/null
else
  say "docker is already installed"
fi

systemctl enable --now docker >/dev/null 2>&1 || true
docker info >/dev/null 2>&1 || die "docker is installed but not running"

if ! docker network inspect "$NETWORK" >/dev/null 2>&1; then
  say "creating the $NETWORK network"
  docker network create "$NETWORK" >/dev/null
fi

# ----------------------------------------------------------------- traefik

if docker ps --format '{{.Names}}' | grep -qx traefik; then
  say "traefik is already running"
else
  say "starting traefik"
  docker rm -f traefik >/dev/null 2>&1 || true
  docker run -d --name traefik --restart always \
    --network "$NETWORK" \
    -p 80:80 -p 443:443 \
    -v /var/run/docker.sock:/var/run/docker.sock:ro \
    -v dockyard-certs:/certs \
    traefik:v3.1 \
    --providers.docker=true \
    --providers.docker.exposedbydefault=false \
    --providers.docker.network="$NETWORK" \
    --entrypoints.web.address=:80 \
    --entrypoints.websecure.address=:443 \
    --entrypoints.web.http.redirections.entrypoint.to=websecure \
    --entrypoints.web.http.redirections.entrypoint.scheme=https \
    --certificatesresolvers.le.acme.httpchallenge=true \
    --certificatesresolvers.le.acme.httpchallenge.entrypoint=web \
    --certificatesresolvers.le.acme.email="$ACME_EMAIL" \
    --certificatesresolvers.le.acme.storage=/certs/acme.json >/dev/null
fi

# ----------------------------------------------------------------- register

say "detecting the public ip"
PUBLIC_IP="$(curl -fsS --max-time 10 https://api.ipify.org || true)"
[ -n "$PUBLIC_IP" ] || PUBLIC_IP="$(curl -fsS --max-time 10 https://ifconfig.me || true)"
[ -n "$PUBLIC_IP" ] || die "could not detect the public ip of this server"
say "public ip is $PUBLIC_IP"

OS_INFO="$( (. /etc/os-release 2>/dev/null && printf '%s' "$PRETTY_NAME") || uname -s )"
OS_INFO="$OS_INFO ($(uname -m))"
HOSTNAME_VALUE="$(hostname)"

say "registering with $SERVER"

REGISTER_BODY="$(jq -n \
  --arg ip "$PUBLIC_IP" \
  --arg os "$OS_INFO" \
  --arg host "$HOSTNAME_VALUE" \
  '{public_ip: $ip, os_info: $os, hostname: $host}')"

HTTP_RESPONSE="$(curl -sS -w '\n%{http_code}' -X POST \
  "$SERVER/api/v1/agents/register" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "$REGISTER_BODY")"

HTTP_CODE="$(printf '%s' "$HTTP_RESPONSE" | tail -n1)"
HTTP_BODY="$(printf '%s' "$HTTP_RESPONSE" | sed '$d')"

if [ "$HTTP_CODE" != "201" ]; then
  printf '%s\n' "$HTTP_BODY" >&2
  die "registration failed with status $HTTP_CODE"
fi

AGENT_ID="$(printf '%s' "$HTTP_BODY" | jq -r '.data.agent_id')"
AGENT_TOKEN="$(printf '%s' "$HTTP_BODY" | jq -r '.data.token')"
SIGNING_SECRET="$(printf '%s' "$HTTP_BODY" | jq -r '.data.signing_secret')"
DOMAIN="$(printf '%s' "$HTTP_BODY" | jq -r '.data.domain')"
WILDCARD="$(printf '%s' "$HTTP_BODY" | jq -r '.data.wildcard')"
AGENT_PORT="$(printf '%s' "$HTTP_BODY" | jq -r '.data.port')"

[ "$AGENT_ID" != "null" ] || die "the server response did not contain an agent id"
[ -n "$AGENT_PORT" ] && [ "$AGENT_PORT" != "null" ] && PORT="$AGENT_PORT"

say "registered as $AGENT_ID"

# ----------------------------------------------------------------- state

mkdir -p "$STATE_DIR"
chmod 700 "$STATE_DIR"

jq -n \
  --arg id "$AGENT_ID" \
  --arg token "$AGENT_TOKEN" \
  --arg secret "$SIGNING_SECRET" \
  --arg at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  '{agent_id: $id, control_plane_token: $token, signing_secret: $secret, registered_at: $at}' \
  > "$STATE_FILE"

chmod 600 "$STATE_FILE"
say "state written to $STATE_FILE"

# ----------------------------------------------------------------- agent

say "pulling $AGENT_IMAGE"
docker pull "$AGENT_IMAGE" >/dev/null

say "starting the agent"
docker rm -f dockyard-agent >/dev/null 2>&1 || true

docker run -d --name dockyard-agent --restart always \
  --network "$NETWORK" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$STATE_DIR":/var/lib/dockyard \
  -p "$PORT":"$PORT" \
  -e DOCKYARD_CONTROL_PLANE_URL="$SERVER" \
  -e DOCKYARD_STATE_FILE=/var/lib/dockyard/agent.json \
  -e DOCKYARD_AGENT_ADDR="0.0.0.0:$PORT" \
  -e DOCKYARD_AGENT_DOMAIN="$DOMAIN" \
  -e DOCKYARD_TRAEFIK_NETWORK="$NETWORK" \
  -e DOCKYARD_ENV=production \
  -e DOCKYARD_LOG_LEVEL=info \
  "$AGENT_IMAGE" >/dev/null

# ----------------------------------------------------------------- health

say "waiting for the agent to come up"
HEALTHY=0
for _ in $(seq 1 30); do
  if curl -fsS --max-time 2 "http://127.0.0.1:$PORT/healthz" >/dev/null 2>&1; then
    HEALTHY=1
    break
  fi
  sleep 1
done

if [ "$HEALTHY" -ne 1 ]; then
  warn "the agent did not answer on port $PORT"
  warn "logs: docker logs dockyard-agent"
  die "install finished but the agent is not healthy"
fi

printf '\n%sAgent is live%s\n\n' "$GREEN$BOLD" "$RESET"
printf '  agent id   %s\n' "$AGENT_ID"
printf '  domain     %s\n' "$DOMAIN"
printf '  apps       %s\n' "$WILDCARD"
printf '  public ip  %s\n' "$PUBLIC_IP"
printf '\n  logs       docker logs -f dockyard-agent\n'
printf '  restart    docker restart dockyard-agent\n\n'
