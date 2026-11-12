# Native macOS client

The native client uses SwiftUI and the existing Go engine. It presents scan progress, disk usage, folder browsing, findings, installed apps, duplicate groups, history, cleanup plans, approvals, operation history, undo, and settings. There is no embedded web page.

The minimum deployment target is macOS 12. Build with a Swift 6 toolchain, Xcode command line tools, the project's Go toolchain, and Bun.

```sh
make native
```

The locally signed app is written to `dist/native/ebb.app`. It contains one app icon and a backend at `Contents/Helpers/ebb`. The default marketing version is `0.2.0`; the build is a separate integer. Explicit values can be passed to `scripts/native-app.sh 0.2.0 42`.

This is a local preview build for the current Mac's architecture. It does not replace `/Applications/ebb.app`, install a launch agent, or publish a release. Distribution signing, notarization, universal packaging, and the web client's disk visualizations are separate release work.

## Engine lifecycle

The app discovers a running engine through `~/.local/state/ebb/server.json`. It checks the process and loopback health endpoint before reading `ui-token`. Requests carry the token in the Authorization header.

When there is no running engine, the app starts its bundled helper on `127.0.0.1:47340`. Quitting stops only a helper it started. An existing background service keeps running. A cleanup in progress must finish before the app quits; the backend also waits for an in-flight item to finish and be logged during shutdown.

Full Disk Access is the same macOS requirement as for the existing app. Without it, a scan can proceed but some protected folders remain outside its view. The UI provides a link to the relevant settings.

## Verification

```sh
make native-check
make native-smoke
```

The smoke command starts a real Go server with a fresh demo home and exercises the Swift transport: scan, findings, special-character folder paths, app inventory, history, approval refusal, Trash, operation log, undo, duplicates, settings, and live events. The temporary server and home are removed afterward.

To inspect the native UI with demo data:

```sh
D=$(mktemp -d)
scripts/demo-home.sh "$D"
EBB_HOME="$D" EBB_SANDBOX=1 EBB_PORT=47431 dist/native/ebb.app/Contents/MacOS/EbbDesktop
```

Quit the app before removing the demo directory. The launcher rejects a home override without sandbox mode and requires an explicit port for sandbox launches.

The core transport and DTOs live in `Sources/EbbCore`, process discovery and ownership in `Sources/EbbRuntime`, and SwiftUI screens and application state in `Sources/EbbDesktop`. The native UI uses the same plan/approve/execute API and backend protections as the web client and CLI.
