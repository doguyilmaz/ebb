import SwiftUI

struct InstalledAppsView: View {
    @ObservedObject var store: EbbStore
    @State private var filter = ""

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Text("\(store.apps.count) apps").font(.headline)
                Spacer()
                TextField("Filter apps", text: $filter).frame(width: 200)
            }.padding(18)
            List(store.apps.filter { filter.isEmpty || $0.name.localizedCaseInsensitiveContains(filter) }) { app in
                VStack(alignment: .leading, spacing: 8) {
                    HStack {
                        Image(systemName: "app").foregroundStyle(.secondary)
                        Text(app.name).font(.headline)
                        Spacer()
                        Text(DiskFormat.bytes(app.bytes)).monospacedDigit()
                        Button("Show in Finder") { store.reveal(app.path) }
                        Button("Review uninstall…") {
                            Task { await store.plan(.init(app: app.bundleId, source: "ui")) }
                        }.disabled(store.working || app.apple || app.rootOwned)
                    }
                    Text(app.path).font(.caption.monospaced()).foregroundStyle(.secondary).textSelection(.enabled)
                    if app.leftoverBytes > 0 {
                        Text("\(DiskFormat.bytes(app.leftoverBytes)) in Library data").font(.caption).foregroundStyle(.secondary)
                    }
                }.padding(.vertical, 8)
            }
        }
    }
}
