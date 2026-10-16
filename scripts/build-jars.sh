#!/usr/bin/env bash
# Build the patched maestro jars for a maestro version that ships as a class
# overlay (overlay/<version>/) instead of full jars in assets/jars/<version>/.
#
# maestro 2.10.0's maestro-cli jar is ~105 MB, over GitHub's 100 MB file limit,
# so the repo keeps only the recompiled classes from
# patches/maestro-<version>-physical-device.patch. This script downloads the
# official maestro.zip for that version, checks its sha256, swaps the patched
# classes into the stock jars and writes dist/maestro-jars-<version>.zip.
#
# Usage: scripts/build-jars.sh 2.10.0
set -euo pipefail

version="${1:?usage: $0 <maestro-version>}"
root="$(cd "$(dirname "$0")/.." && pwd)"
overlay="$root/overlay/$version"
out="$root/dist"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

[ -d "$overlay" ] || { echo "no overlay for maestro $version" >&2; exit 1; }

echo "Downloading maestro $version..."
curl -fsSL -o "$work/maestro.zip" \
  "https://github.com/mobile-dev-inc/Maestro/releases/download/cli-$version/maestro.zip"
(cd "$work" && shasum -a 256 -c "$overlay/maestro.zip.sha256")

unzip -q "$work/maestro.zip" -d "$work/src"
jars="$work/jars"
mkdir -p "$jars"
cp "$work"/src/maestro/lib/maestro-*.jar "$jars/"

for dir in "$overlay"/*.jar; do
  jar="$(basename "$dir")"
  [ -f "$jars/$jar" ] || { echo "overlay targets missing jar $jar" >&2; exit 1; }
  # Every overlay class must replace an existing entry: a new name would mean
  # the overlay was compiled from a different source than this release.
  (cd "$dir" && find . -type f | sed 's|^\./||') | while read -r entry; do
    unzip -Z1 "$jars/$jar" "$entry" >/dev/null 2>&1 \
      || { echo "$jar has no entry $entry" >&2; exit 1; }
  done
  (cd "$dir" && zip -q -r "$jars/$jar" .)
  echo "patched $jar"
done

mkdir -p "$out"
rm -f "$out/maestro-jars-$version.zip"
(cd "$jars" && zip -q -r "$out/maestro-jars-$version.zip" .)
echo "wrote dist/maestro-jars-$version.zip"
