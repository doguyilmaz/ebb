import SwiftUI

struct HistoryView: View {
    @ObservedObject var store: EbbStore

    var body: some View {
        List(store.history) { snapshot in
            VStack(alignment: .leading, spacing: 8) {
                HStack {
                    Text(DiskFormat.date(snapshot.time)).font(.headline)
                    Spacer()
                    Text(DiskFormat.bytes(snapshot.scanned)).monospacedDigit()
                }
                Text(snapshot.root).font(.caption.monospaced()).textSelection(.enabled)
                Text("\(snapshot.files.formatted()) files · \(snapshot.dirs.formatted()) folders\(snapshot.incremental == true ? " · incremental" : "")")
                    .font(.caption).foregroundStyle(.secondary)
            }.padding(.vertical, 8)
        }
        .overlay {
            if store.history.isEmpty && !store.loading {
                EmptyState(title: "No scan history yet", message: "Finished scans appear here.", symbol: "clock") {
                    Button("Start a scan") { Task { await store.scan() } }.disabled(store.working)
                }
            }
        }
    }
}
