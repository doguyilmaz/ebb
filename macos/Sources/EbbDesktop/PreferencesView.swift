import EbbCore
import SwiftUI

struct PreferencesView: View {
    @ObservedObject var store: EbbStore
    @State private var root = ""
    @State private var threads = 0
    @State private var staleDays = 90
    @State private var largeFile = "1GB"
    @State private var protect = ""
    @State private var background = true
    @State private var lowSpace = 20.0
    @State private var saving = false
    @State private var saved = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 22) {
                Text("Settings").font(.title2.weight(.semibold))
                GroupBox("Scanning") {
                    VStack(alignment: .leading, spacing: 12) {
                        TextField("Default root (empty uses this Mac)", text: $root)
                        Stepper("Scanner workers: \(threads == 0 ? "automatic" : String(threads))", value: $threads, in: 0...128)
                        Stepper("Stale after \(staleDays) days", value: $staleDays, in: 1...3650)
                        HStack { Text("Large file threshold"); TextField("1GB", text: $largeFile).frame(width: 120) }
                    }.padding(8)
                }
                GroupBox("Protected paths") {
                    VStack(alignment: .leading, spacing: 8) {
                        Text("One path or glob per line. Protected files cannot be cleaned.")
                            .font(.caption).foregroundStyle(.secondary)
                        TextEditor(text: $protect).font(.system(.body, design: .monospaced)).frame(height: 100)
                            .accessibilityLabel("Protected paths")
                    }.padding(8)
                }
                GroupBox("Background service preferences") {
                    VStack(alignment: .leading, spacing: 10) {
                        Toggle("Keep scans fresh when the background service is running", isOn: $background)
                        HStack {
                            Text("Warn below")
                            TextField("GB", value: $lowSpace, format: .number).frame(width: 80)
                            Text("GB free")
                        }
                    }.padding(8)
                }
                HStack {
                    Button("Full Disk Access…") { store.openPrivacySettings() }
                    Spacer()
                    if saved { Text("Saved").foregroundStyle(.secondary) }
                    Button(saving ? "Saving…" : "Save settings") { Task { await save() } }
                        .disabled(saving || store.settings == nil)
                }
                if let status = store.status {
                    Text("Engine \(status.version) · \(status.sandboxed ? "sandbox data" : "this Mac")")
                        .font(.caption).foregroundStyle(.secondary)
                }
            }.frame(maxWidth: 620, alignment: .leading)
        }
        .onAppear { read() }
        .onChange(of: store.settings?.root) { _ in read() }
    }

    private func read() {
        guard let settings = store.settings else { return }
        root = settings.root
        threads = settings.threads
        staleDays = settings.staleDays
        largeFile = settings.largeFile
        protect = settings.protect.joined(separator: "\n")
        background = settings.backgroundScans
        lowSpace = settings.lowSpaceGb
    }

    private func save() async {
        saving = true
        saved = false
        let settings = EbbCore.Settings(
            root: root, threads: threads, staleDays: staleDays, largeFile: largeFile,
            protect: protect.split(separator: "\n").map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty },
            backgroundScans: background, lowSpaceGb: lowSpace
        )
        await store.saveSettings(settings)
        saving = false
        saved = store.error == nil
    }
}
