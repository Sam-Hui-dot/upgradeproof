#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
BIN=${1:-"$ROOT/upgradeproof"}
cd "$ROOT/fixtures/postgres-compose"
docker build -q -t upgradeproof-fixture-postgres:v1 --build-arg APP_VERSION=v1 ./app >/dev/null
"$BIN" validate
"$BIN" test --report-dir .artifacts
python3 -c 'import glob,json,subprocess; r=json.load(open(sorted(glob.glob(".artifacts/*/report.json"))[-1])); assert r["overall_status"] == "passed"; assert r["paths"][0]["checks"][0]["status"] == "passed"; assert subprocess.run(["docker","image","inspect","upgradeproof-fixture-postgres:upgradeproof-target-"+r["run_id"]],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode != 0'
