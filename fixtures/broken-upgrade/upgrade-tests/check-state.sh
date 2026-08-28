#!/bin/sh
set -eu
test "$(curl -fsS http://127.0.0.1:18082/value)" = "upgradeproof-state"
