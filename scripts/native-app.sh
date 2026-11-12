#!/usr/bin/env bash
# Builds a local SwiftUI app beside the existing web-window app. Does not install it.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
version="${1:-0.2.0}"
build="${2:-${BUILD_NUMBER:-$(git rev-list --count HEAD)}}"
if [[ ! "$version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
  echo "version must be MAJOR.MINOR.PATCH: $version" >&2
  exit 2
fi
if [[ ! "$build" =~ ^[0-9]+$ ]]; then
  echo "build number must be an integer: $build" >&2
  exit 2
fi
swift build --package-path macos -c release --product EbbDesktop
swift_bin="$(swift build --package-path macos -c release --show-bin-path)"
app="$root/dist/native/ebb.app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Helpers" "$app/Contents/Resources"
cp "$swift_bin/EbbDesktop" "$app/Contents/MacOS/EbbDesktop"
MACOSX_DEPLOYMENT_TARGET=12.0 \
  CGO_CFLAGS="${CGO_CFLAGS:--O2 -g} -mmacosx-version-min=12.0" \
  CGO_LDFLAGS="${CGO_LDFLAGS:--O2 -g} -mmacosx-version-min=12.0" \
  go build -ldflags "-X github.com/doguyilmaz/ebb/internal/cli.Version=$version" -o "$app/Contents/Helpers/ebb" ./cmd/ebb
cp assets/brand/ebb-spectrum.icns "$app/Contents/Resources/AppIcon.icns"
sed -e "s/__VERSION__/$version/g" -e "s/__BUILD__/$build/g" packaging/NativeInfo.plist > "$app/Contents/Info.plist"
plutil -lint -s "$app/Contents/Info.plist"
codesign --force --sign - "$app/Contents/Helpers/ebb"
codesign --force --sign - "$app"
codesign --verify --strict --deep "$app"
echo "Built $app ($version, build $build)"
