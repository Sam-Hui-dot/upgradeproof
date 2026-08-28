#!/usr/bin/env bash
set -euo pipefail

binary=""
temp_dir=""

cleanup() {
  if [[ -n "$temp_dir" && -d "$temp_dir" ]]; then
    rm -rf -- "$temp_dir"
  fi
}
trap cleanup EXIT

if [[ -z "$binary" ]]; then
  version=$(tr -d '[:space:]' < "$GITHUB_ACTION_PATH/VERSION")
  if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
    echo "UpgradeProof Action requires a release tag such as v0.1.0; got '$version'" >&2
    exit 2
  fi

  case "${RUNNER_OS:-}" in
    Linux) os=linux; extension=tar.gz; executable=upgradeproof ;;
    macOS) os=darwin; extension=tar.gz; executable=upgradeproof ;;
    Windows) os=windows; extension=zip; executable=upgradeproof.exe ;;
    *) echo "unsupported runner OS: ${RUNNER_OS:-unknown}" >&2; exit 2 ;;
  esac
  case "${RUNNER_ARCH:-}" in
    X64) arch=amd64 ;;
    ARM64) arch=arm64 ;;
    *) echo "unsupported runner architecture: ${RUNNER_ARCH:-unknown}" >&2; exit 2 ;;
  esac
  if [[ "$os" == windows && "$arch" != amd64 ]]; then
    echo "UpgradeProof v0.1 release artifacts do not include windows/$arch" >&2
    exit 2
  fi

  asset="upgradeproof_${version}_${os}_${arch}.${extension}"
  base_url="https://github.com/Sam-Hui-dot/upgradeproof/releases/download/${version}"
  temp_dir=$(mktemp -d)
  curl --fail --location --proto '=https' --tlsv1.2 --output "$temp_dir/$asset" "$base_url/$asset"
  curl --fail --location --proto '=https' --tlsv1.2 --output "$temp_dir/checksums.txt" "$base_url/checksums.txt"

  expected=$(awk -v name="$asset" '$2 == name || $2 == "*" name { print $1; exit }' "$temp_dir/checksums.txt")
  if [[ ! "$expected" =~ ^[0-9a-fA-F]{64}$ ]]; then
    echo "no valid SHA256 checksum found for $asset" >&2
    exit 3
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$temp_dir/$asset" | awk '{print $1}')
  elif command -v shasum >/dev/null 2>&1; then
    actual=$(shasum -a 256 "$temp_dir/$asset" | awk '{print $1}')
  else
    echo "no SHA256 verification tool is available" >&2
    exit 3
  fi
  actual=$(printf '%s' "$actual" | tr '[:upper:]' '[:lower:]')
  expected=$(printf '%s' "$expected" | tr '[:upper:]' '[:lower:]')
  if [[ "$actual" != "$expected" ]]; then
    echo "SHA256 checksum mismatch for $asset" >&2
    exit 3
  fi

  if [[ "$extension" == zip ]]; then
    unzip -q "$temp_dir/$asset" -d "$temp_dir/bin"
  else
    mkdir -p "$temp_dir/bin"
    tar -xzf "$temp_dir/$asset" -C "$temp_dir/bin"
  fi
  binary="$temp_dir/bin/$executable"
fi

if [[ ! -f "$binary" ]]; then
  echo "UpgradeProof binary does not exist: $binary" >&2
  exit 2
fi
chmod +x "$binary"

case "${UPGRADEPROOF_INPUT_KEEP:-false}" in
  true|false) ;;
  *) echo "keep-on-failure must be true or false" >&2; exit 2 ;;
esac

args=(test -c "${UPGRADEPROOF_INPUT_CONFIG:-upgradeproof.yml}" --report-dir "${UPGRADEPROOF_INPUT_REPORT_DIR:-.upgradeproof}")
if [[ -n "${UPGRADEPROOF_INPUT_PATH:-}" ]]; then
  args+=(--path "$UPGRADEPROOF_INPUT_PATH")
fi
if [[ "${UPGRADEPROOF_INPUT_KEEP:-false}" == true ]]; then
  args+=(--keep-on-failure)
fi

"$binary" "${args[@]}"
