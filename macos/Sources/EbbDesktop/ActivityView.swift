import SwiftUI

struct ActivityView: View {
    @ObservedObject var store: EbbStore

    private var undone: Set<String> {
        Set(store.operations.filter { $0.action == "undo" && $0.result == "done" }
            .map { String($0.id.dropFirst("undo-".count)) })
    }

    var body: some View {
        List(store.operations) { operation in
            VStack(alignment: .leading, spacing: 7) {
                HStack {
                    Text(operation.action.capitalized).font(.headline)
                    Text(operation.result).foregroundStyle(operation.result == "done" ? Color.secondary : Color.red)
                    Spacer()
                    Text(DiskFormat.bytes(operation.bytes)).monospacedDigit()
                    if operation.action == "trash" && operation.result == "done" && !undone.contains(operation.id) {
                        Button("Put back") { Task { await store.undo(operation) } }.disabled(store.working)
                    }
                }
                Text(operation.path).font(.caption.monospaced()).textSelection(.enabled)
                if let error = operation.error, !error.isEmpty { Text(error).font(.caption).foregroundStyle(.red) }
                Text(DiskFormat.date(operation.time)).font(.caption).foregroundStyle(.secondary)
            }.padding(.vertical, 8)
        }
        .overlay {
            if store.operations.isEmpty && !store.loading {
                EmptyState(title: "Nothing cleaned yet", message: "Cleanup results and items you can put back appear here.",
                           symbol: "list.bullet.rectangle") { EmptyView() }
            }
        }
    }
}
