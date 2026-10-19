#!/usr/bin/env bash
# Builds ebb.app around the universal binary, so macOS shows ebb with its icon
# in privacy settings, Login Items and notifications, and keeps Full Disk
# Access across upgrades. GoReleaser runs it once the universal binary exists.
#   scripts/macapp.sh VERSION BINARY snapshot|local|release
# snapshot signs ad hoc, local signs with the Developer ID in your keychain,
# release imports the certificate from the environment, then notarizes and
# staples so Gatekeeper accepts the app offline.
set -euo pipefail
version=${1#v}
version=${version%%-*}
bin=$2
mode=$3
root="$(cd "$(dirname "$0")/.." && pwd)"
app="$root/dist/macapp/ebb.app"

rm -rf "$app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp "$bin" "$app/Contents/MacOS/ebb"
cp "$root/assets/brand/ebb-spectrum.icns" "$app/Contents/Resources/AppIcon.icns"
sed "s/__VERSION__/$version/g" "$root/packaging/Info.plist" >"$app/Contents/Info.plist"
plutil -lint -s "$app/Contents/Info.plist"

case $mode in
snapshot)
	codesign --force --sign - "$app"
	exit 0
	;;
local)
	codesign --force --options runtime --timestamp --sign "Developer ID Application" "$app"
	codesign --verify --strict "$app"
	exit 0
	;;
release) ;;
*)
	echo "unknown mode $mode" >&2
	exit 2
	;;
esac

tmp=$(mktemp -d)
kc="$tmp/ebb.keychain-db"
pw=$(uuidgen)
trap 'security delete-keychain "$kc" 2>/dev/null || true; rm -rf "$tmp"' EXIT

# The secrets hold a file's contents, base64 for the certificate; the API key may also be the PEM text itself.
secret() {
	if [[ $1 == -----BEGIN* ]]; then printf '%s\n' "$1" >"$2"; else base64 --decode <<<"$1" >"$2"; fi
}
secret "$MACOS_SIGN_P12" "$tmp/cert.p12"
secret "$MACOS_NOTARY_KEY" "$tmp/key.p8"

security create-keychain -p "$pw" "$kc"
security set-keychain-settings -lut 3600 "$kc"
security unlock-keychain -p "$pw" "$kc"
security import "$tmp/cert.p12" -k "$kc" -P "$MACOS_SIGN_PASSWORD" -T /usr/bin/codesign
security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$pw" "$kc" >/dev/null
# shellcheck disable=SC2046
security list-keychains -d user -s "$kc" $(security list-keychains -d user | tr -d '"')

codesign --force --options runtime --timestamp --keychain "$kc" --sign "Developer ID Application" "$app"
codesign --verify --strict -vv "$app"

ditto -c -k --keepParent "$app" "$tmp/ebb.zip"
xcrun notarytool submit "$tmp/ebb.zip" --key "$tmp/key.p8" --key-id "$MACOS_NOTARY_KEY_ID" --issuer "$MACOS_NOTARY_ISSUER_ID" \
	--wait --timeout 20m --output-format json >"$tmp/notary.json"
if [ "$(plutil -extract status raw -o - "$tmp/notary.json")" != Accepted ]; then
	cat "$tmp/notary.json" >&2
	exit 1
fi
xcrun stapler staple "$app"
xcrun stapler validate "$app"
spctl --assess --type execute -vv "$app"
