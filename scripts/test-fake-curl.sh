#!/usr/bin/env bash
set -euo pipefail

# Offline transport shim for this repository's release-candidate Action test.
# The production Action still constructs its fixed GitHub Release URLs and
# performs its normal checksum verification and extraction.
output=""
url=""
original_args=("$@")
while [[ $# -gt 0 ]]; do
  case "$1" in
    --output)
      output=$2
      shift 2
      ;;
    http://*|https://*)
      url=$1
      shift
      ;;
    *)
      shift
      ;;
  esac
done
if [[ ! "$url" =~ ^https://github\.com/Sam-Hui-dot/upgradeproof/releases/download/ ]]; then
	exec /usr/bin/curl "${original_args[@]}"
fi
if [[ -z "$output" || -z "${UPGRADEPROOF_ACTION_TEST_ASSET_DIR:-}" ]]; then
	echo "invalid UpgradeProof release download in curl shim" >&2
	exit 2
fi
source_file="$UPGRADEPROOF_ACTION_TEST_ASSET_DIR/${url##*/}"
if [[ ! -f "$source_file" ]]; then
  echo "release-candidate asset not found: $source_file" >&2
  exit 22
fi
cp -- "$source_file" "$output"
