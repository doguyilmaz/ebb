#!/usr/bin/env bash
# Builds ebb.app around the universal binary, so macOS shows ebb with its icon
# in privacy settings, Login Items and notifications, and keeps Full Disk
# Access across upgrades. GoReleaser runs it once the universal binary exists.
#   scripts/macapp.sh VERSION BINARY [local] [BUILD_NUMBER]
# The app is signed ad hoc: the tapster hub signs it with the Developer ID,
# notarizes and staples it when it publishes a release. local signs with the
# Developer ID in your keychain instead, to try the app as people get it.
set -euo pipefail
version=${1#v}
version=${version%%-*}
bin=$2
mode=${3:-}
build=${4:-${BUILD_NUMBER:-}}
root="$(cd "$(dirname "$0")/.." && pwd)"
app="$root/dist/macapp/ebb.app"

if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "version must be MAJOR.MINOR.PATCH: $version" >&2
	exit 2
fi
if [[ -z "$build" ]]; then
	IFS=. read -r major minor patch <<<"$version"
	if ((10#$major > 999 || 10#$minor > 999 || 10#$patch > 999)); then
		echo "pass BUILD_NUMBER for version components above 999" >&2
		exit 2
	fi
	build=$((10#$major * 1000000 + 10#$minor * 1000 + 10#$patch))
fi
if [[ ! "$build" =~ ^[0-9]+$ ]]; then
	echo "build number must be an integer: $build" >&2
	exit 2
fi

rm -rf "$app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp "$bin" "$app/Contents/MacOS/ebb"
cp "$root/assets/brand/ebb-spectrum.icns" "$app/Contents/Resources/AppIcon.icns"
sed -e "s/__VERSION__/$version/g" -e "s/__BUILD__/$build/g" "$root/packaging/Info.plist" >"$app/Contents/Info.plist"
plutil -lint -s "$app/Contents/Info.plist"

case $mode in
"") codesign --force --sign - "$app" ;;
local) codesign --force --options runtime --timestamp --sign "Developer ID Application" "$app" ;;
*)
	echo "unknown mode $mode" >&2
	exit 2
	;;
esac
codesign --verify --strict --deep "$app"
