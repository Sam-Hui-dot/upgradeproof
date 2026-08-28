#!/bin/sh
set -eu
curl -fsS -X POST http://127.0.0.1:18083/seed
docker compose -f "$UPGRADEPROOF_COMPOSE_FILE" -p "$UPGRADEPROOF_PROJECT" ps -q postgres > "$UPGRADEPROOF_REPORT_DIR/postgres-before.container-id"
