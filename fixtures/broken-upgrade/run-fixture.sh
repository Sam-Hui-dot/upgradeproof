#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
BIN=${1:-"$ROOT/upgradeproof"}
cd "$ROOT/fixtures/broken-upgrade"
docker build -q -t upgradeproof-fixture-broken:v1 --build-arg BROKEN=0 ./app >/dev/null
"$BIN" validate
set +e
"$BIN" test --report-dir .artifacts
CODE=$?
set -e
test "$CODE" -eq 1
python3 -c 'import glob,json; r=json.load(open(sorted(glob.glob(".artifacts/*/report.json"))[-1])); p=r["paths"][0]; assert r["overall_status"] == "failed"; assert p["checks"][0]["name"] == "state-value-preserved"; assert p["checks"][0]["status"] == "failed"'
