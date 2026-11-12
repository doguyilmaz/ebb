import SwiftUI

@main
@MainActor
struct EbbDesktopApp: App {
    @StateObject private var store = EbbStore()
    @NSApplicationDelegateAdaptor(EbbAppDelegate.self) private var delegate

    var body: some Scene {
        WindowGroup("ebb") {
            DesktopView(store: store)
                .frame(minWidth: 980, minHeight: 660)
                .onAppear { delegate.store = store }
                .task { await store.connect() }
        }
        .commands {
            CommandGroup(replacing: .newItem) {
                Button("Refresh") { Task { await store.refresh() } }
                    .keyboardShortcut("r")
                Button("Scan again") { Task { await store.scan() } }
                    .keyboardShortcut("r", modifiers: [.command, .shift])
                    .disabled(store.scanRunning || store.cleaning)
            }
        }
        Settings {
            PreferencesView(store: store)
                .padding(24)
                .frame(width: 600)
                .task { await store.loadSettings() }
        }
    }
}
