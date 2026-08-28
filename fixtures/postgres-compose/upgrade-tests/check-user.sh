#!/bin/sh
set -eu
test "$(curl -fsS http://127.0.0.1:18083/users)" = "Ada"
test "$(docker compose -f "$UPGRADEPROOF_COMPOSE_FILE" -p "$UPGRADEPROOF_PROJECT" ps -q postgres)" = "$(cat "$UPGRADEPROOF_REPORT_DIR/postgres-before.container-id")"
