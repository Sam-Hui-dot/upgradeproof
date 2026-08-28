#!/bin/sh
set -eu
test "$(curl -fsS http://127.0.0.1:18081/value)" = "upgradeproof-state"
