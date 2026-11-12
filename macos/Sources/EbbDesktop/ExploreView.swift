import Foundation
import SwiftUI

struct ExploreView: View {
    @ObservedObject var store: EbbStore
    @State private var selected: Set<String> = []
    @State private var filter = ""

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Button {
                    let parent = URL(fileURLWithPath: store.path).deletingLastPathComponent().path
                    Task { await store.openFolder(parent) }
                } label: { Label("Up", systemImage: "arrow.up") }
                .disabled(store.path.isEmpty || store.path == store.status?.last?.root)
                Text(store.path.isEmpty ? (store.status?.last?.root ?? "No scan yet") : store.path)
                    .font(.callout).lineLimit(1).truncationMode(.middle).textSelection(.enabled)
                Spacer()
                TextField("Filter names", text: $filter).frame(width: 180)
            }.padding(16)
            Table(store.children.filter { filter.isEmpty || $0.name.localizedCaseInsensitiveContains(filter) }, selection: $selected) {
                TableColumn("Name") { item in
                    HStack {
                        Image(systemName: item.kind == "dir" ? "folder" : "doc").foregroundStyle(.secondary)
                        if item.kind == "dir" {
                            Button(item.name) {
                                selected.removeAll()
                                Task { await store.openFolder(item.path) }
                            }.buttonStyle(.link)
                        } else { Text(item.name) }
                    }
                }
                TableColumn("On disk") { Text(DiskFormat.bytes($0.alloc)).monospacedDigit() }.width(100)
                TableColumn("Files") { Text($0.files.formatted()).monospacedDigit() }.width(90)
                TableColumn("Finder") { item in
                    Button { store.reveal(item.path) } label: { Image(systemName: "arrow.up.forward.square") }
                        .help("Show \(item.name) in Finder").accessibilityLabel("Show \(item.name) in Finder")
                }.width(55)
            }
            HStack {
                VStack(alignment: .leading, spacing: 3) {
                    Text("\(selected.count) selected").foregroundStyle(.secondary)
                    Text("Smaller files are included in their folder totals.").font(.caption).foregroundStyle(.secondary)
                }
                Spacer()
                Button("Review cleanup…") {
                    Task { await store.plan(.init(paths: Array(selected), source: "ui")) }
                }.disabled(selected.isEmpty || store.working)
            }.padding(16)
        }
        .onChange(of: store.path) { _ in selected.removeAll() }
    }
}
