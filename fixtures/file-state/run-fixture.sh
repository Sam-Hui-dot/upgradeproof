#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
BIN=${1:-"$ROOT/upgradeproof"}
cd "$ROOT/fixtures/file-state"
docker build -q -t upgradeproof-fixture-file:v1 --build-arg APP_VERSION=v1 ./app >/dev/null
docker build -q -t upgradeproof-fixture-file:v2 --build-arg APP_VERSION=v2 ./app >/dev/null
"$BIN" validate
"$BIN" test --report-dir .artifacts
python3 -c 'import glob,json,subprocess; r=json.load(open(sorted(glob.glob(".artifacts/*/report.json"))[-1])); assert r["overall_status"] == "passed"; assert len(r["paths"][0]["resolved_images"]) == 3; assert subprocess.run(["docker","image","inspect","upgradeproof-target:"+r["run_id"]],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode != 0'
