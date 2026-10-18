#!/usr/bin/env bash
# Builds a small fake home for trying ebb safely: EBB_HOME=DIR EBB_SANDBOX=1 ebb serve
set -euo pipefail
home="${1:?usage: scripts/demo-home.sh DIR}"
mk() { mkdir -p "$(dirname "$1")"; head -c "$2" /dev/zero >"$1"; }

mk "$home/.bun/install/cache/react@19.2.0@@@1/index.js" $((6 << 20))
mk "$home/.bun/install/cache/next@15.0.0@@@1/dist/server.js" $((9 << 20))
mk "$home/.npm/_cacache/content-v2/sha512/aa/blob" $((5 << 20))
mk "$home/Library/Caches/com.example.editor/Cache.db" $((4 << 20))
mk "$home/Library/Logs/DiagnosticReports/Example.ips" $((3 << 20))
mk "$home/Downloads/Setup-2.3.dmg" $((7 << 20))
mk "$home/.Trash/old-notes.txt" $((2 << 20))
mk "$home/Documents/thesis.pdf" $((3 << 20))

app="$home/Applications/Sketchpad.app"
mk "$app/Contents/MacOS/Sketchpad" $((8 << 20))
cat >"$app/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.example.Sketchpad</string><key>CFBundleName</key><string>Sketchpad</string><key>CFBundleExecutable</key><string>Sketchpad</string><key>CFBundleShortVersionString</key><string>3.1</string></dict></plist>
PLIST
mk "$home/Library/Containers/com.example.Sketchpad/Data/Library/brushes.db" $((5 << 20))
mk "$home/Library/Preferences/com.example.Sketchpad.plist" 900

dd="$home/Library/Developer/Xcode/DerivedData/Gone-abcdef"
mk "$dd/Build/Products/app" $((12 << 20))
cat >"$dd/info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>WorkspacePath</key><string>$home/code/gone/Gone.xcodeproj</string></dict></plist>
PLIST

old="$home/code/old-site"
mk "$old/package.json" 40
mk "$old/src/index.ts" 400
mk "$old/node_modules/big-lib/dist/index.js" $((55 << 20))
find "$old" -exec touch -t 202501010000 {} +

mkdir -p "$home/.config/ebb/rules"
cat >"$home/.config/ebb/rules/demo.toml" <<'TOML'
# The demo's caches are tiny; these copies of built-in rules report them anyway.
[[rule]]
id = "bun.cache"
title = "Bun install cache"
owner = "Bun"
category = "package-cache"
paths = ["~/.bun/install/cache"]
safety = "regenerable"
why = "Bun keeps every version of every package it has ever installed and never evicts old ones."
impact = "The next install downloads the packages it needs again."
min_size = "1MB"
min_target = "1MB"
action = { kind = "delete" }

[[rule]]
id = "app.caches"
title = "App caches"
category = "app-cache"
paths = ["~/Library/Caches/*"]
safety = "regenerable"
why = "Apps keep downloaded and computed data here."
impact = "Apps rebuild their caches as needed."
min_size = "1MB"
min_target = "1MB"
action = { kind = "delete" }

[[rule]]
id = "logs.user"
title = "Logs and crash reports"
category = "logs"
paths = ["~/Library/Logs/*"]
safety = "regenerable"
why = "Apps write logs and crash reports here."
impact = "Old logs are gone; apps start new ones."
min_size = "1MB"
min_target = "1MB"
action = { kind = "delete" }

[[rule]]
id = "installers"
title = "Installers and disk images"
category = "installer"
paths = ["~/Downloads/**/*.{dmg,pkg,xip,iso}"]
safety = "caution"
why = "Installers stay in Downloads after the app is installed."
impact = "You would need to download an installer again to reinstall."
min_size = "1MB"
min_target = "1MB"
action = { kind = "trash" }

[[rule]]
id = "trash"
title = "Trash"
category = "trash"
paths = ["~/.Trash"]
safety = "caution"
why = "Items in the Trash still use disk space until it is emptied."
impact = "Everything in the Trash is deleted permanently."
min_size = "1MB"
min_target = "1MB"
action = { kind = "empty-trash" }
TOML
echo "demo home ready: EBB_HOME=$home EBB_SANDBOX=1 ebb serve --open"
