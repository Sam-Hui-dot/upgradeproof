#!/usr/bin/env bash
set -euo pipefail

version=${1:-}
if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "usage: $0 vMAJOR.MINOR.PATCH[-PRERELEASE]" >&2
  exit 2
fi

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
declared_version=$(tr -d '[:space:]' < "$root/VERSION")
if [[ "$version" != "$declared_version" ]]; then
  echo "requested version $version does not match VERSION ($declared_version)" >&2
  exit 2
fi
dist="$root/dist"
mkdir -p "$dist"
find "$dist" -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +
stage_root=$(mktemp -d)
trap 'rm -rf -- "$stage_root"' EXIT

commit=$(git -C "$root" rev-parse HEAD)
build_date=${SOURCE_DATE_EPOCH:+$(date -u -d "@$SOURCE_DATE_EPOCH" +%Y-%m-%dT%H:%M:%SZ)}
build_date=${build_date:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}
ldflags="-s -w -X main.version=$version -X main.commit=$commit -X main.date=$build_date"

build_archive() {
  os=$1
  arch=$2
  executable=upgradeproof
  extension=tar.gz
  if [[ "$os" == windows ]]; then
    executable=upgradeproof.exe
    extension=zip
  fi
  stage="$stage_root/${os}_${arch}"
  mkdir -p "$stage"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "$ldflags" -o "$stage/$executable" ./cmd/upgradeproof
  archive="upgradeproof_${version}_${os}_${arch}.${extension}"
  if [[ "$extension" == zip ]]; then
    (cd "$stage" && zip -q "$dist/$archive" "$executable")
  else
    tar -C "$stage" -czf "$dist/$archive" "$executable"
  fi
}

cd "$root"
build_archive linux amd64
build_archive linux arm64
build_archive darwin amd64
build_archive darwin arm64
build_archive windows amd64

(cd "$dist" && sha256sum upgradeproof_"${version}"_* > checksums.txt)
