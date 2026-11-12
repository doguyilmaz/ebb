import AppKit

@MainActor
final class EbbAppDelegate: NSObject, NSApplicationDelegate {
    weak var store: EbbStore?

    func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
        guard let store else { return .terminateNow }
        if store.cleaning {
            let alert = NSAlert()
            alert.messageText = "A cleanup is still running"
            alert.informativeText = "Let the current cleanup finish before quitting so its result can be saved."
            alert.addButton(withTitle: "Keep ebb open")
            alert.runModal()
            return .terminateCancel
        }
        Task {
            await store.stop()
            sender.reply(toApplicationShouldTerminate: true)
        }
        return .terminateLater
    }
}
