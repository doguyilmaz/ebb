import EbbCore
import SwiftUI

struct DuplicatesView: View {
    @ObservedObject var store: EbbStore
    @State private var minimum = 10

    private var groups: [DuplicateGroup] { store.duplicates?.result?.groups ?? [] }
    private var extras: [String] { groups.flatMap { $0.copies.dropFirst().map(\.path) } }

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Picker("Minimum file size", selection: $minimum) {
                    Text("1 MB").tag(1)
                    Text("10 MB").tag(10)
                    Text("100 MB").tag(100)
                }.frame(width: 250)
                Spacer()
                if store.duplicateRunning {
                    ProgressView().controlSize(.small)
                    Button("Stop search") { Task { await store.stopDuplicates() } }
                } else {
                    Button("Find duplicates") {
                        Task { await store.findDuplicates(minSize: UInt64(minimum) * 1_000_000) }
                    }.disabled(store.working || store.status?.last == nil)
                }
            }.padding(18)
            if let message = store.duplicates?.error, !message.isEmpty {
                Text(message).foregroundStyle(.red).padding()
            }
            if let result = store.duplicates?.result {
                HStack {
                    Text("\(groups.count) groups · \(DiskFormat.bytes(result.wasted)) in extra copies")
                    Spacer()
                    Button("Review extra copies…") {
                        Task { await store.plan(.init(paths: extras, source: "ui")) }
                    }.disabled(extras.isEmpty || store.working)
                }.padding(.horizontal, 18).padding(.bottom, 12)
            }
            List(Array(groups.enumerated()), id: \.offset) { _, group in
                VStack(alignment: .leading, spacing: 8) {
                    Text("\(DiskFormat.bytes(group.size)) each · \(group.copies.count) copies").font(.headline)
                    ForEach(Array(group.copies.enumerated()), id: \.element.path) { index, copy in
                        HStack {
                            Text(index == 0 ? "Keep" : "Extra copy")
                                .font(.caption).foregroundStyle(index == 0 ? Color.green : Color.secondary).frame(width: 75, alignment: .leading)
                            Text(copy.path).font(.caption.monospaced()).textSelection(.enabled)
                            Spacer()
                            Button("Finder") { store.reveal(copy.path) }
                        }
                    }
                }.padding(.vertical, 8)
            }
            Text("Searches your own files in the last scan. Reviewing extras keeps the first copy in every group.")
                .font(.caption).foregroundStyle(.secondary).padding(14)
        }
    }
}
