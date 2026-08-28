#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
BIN=${1:-"$ROOT/upgradeproof"}
cd "$ROOT/fixtures/safety-bypass"

expect_reject() {
  config=$1
  label=$2
  set +e
  "$BIN" validate -c "$config"
  code=$?
  set -e
  if [ "$code" -ne 2 ]; then
    echo "$label: expected safety/preflight exit 2, got $code" >&2
    exit 1
  fi
}

expect_reject upgradeproof-include-bind.yml "include writable bind"
expect_reject upgradeproof-extends-bind.yml "extends writable bind"
expect_reject upgradeproof-external-volume.yml "included external volume"
expect_reject upgradeproof-explicit-volume.yml "included explicit volume name"
expect_reject upgradeproof-custom-driver.yml "included custom volume driver"
expect_reject upgradeproof-remote-opts.yml "included remote driver_opts"
