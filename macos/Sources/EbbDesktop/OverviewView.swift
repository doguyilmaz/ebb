import SwiftUI

struct OverviewView: View {
    @ObservedObject var store: EbbStore

    var body: some View {
        if let last = store.status?.last {
            ScrollView {
                VStack(alignment: .leading, spacing: 24) {
                    VStack(alignment: .leading, spacing: 6) {
                        Text("Where your disk space went").font(.largeTitle.weight(.bold))
                        Text(last.root).font(.callout).foregroundStyle(.secondary).textSelection(.enabled)
                        Text("Last scan \(DiskFormat.date(last.time)) · \(last.files.formatted()) files")
                            .font(.caption).foregroundStyle(.secondary)
                    }
                    if let disk = store.volume {
                        HStack(spacing: 36) {
                            metric("Used", DiskFormat.bytes(disk.used))
                            metric("Free", DiskFormat.bytes(disk.free))
                            metric("Found to review", DiskFormat.bytes(store.reclaimable))
                        }
                        ProgressView(value: Double(disk.used), total: Double(max(disk.capacity, 1)))
                            .tint(.accentColor)
                        Text("\(DiskFormat.bytes(disk.scanned)) visible to this scan · \(DiskFormat.bytes(disk.notVisible)) outside its view")
                            .font(.caption).foregroundStyle(.secondary)
                    }
                    if store.status?.fullDiskAccess == false {
                        GroupBox {
                            HStack(alignment: .top) {
                                Label("Some folders are hidden by macOS", systemImage: "lock.shield")
                                Spacer()
                                Button("Full Disk Access…") { store.openPrivacySettings() }
                            }.padding(6)
                        }
                    }
                    HStack {
                        Text("Largest folders").font(.headline)
                        Spacer()
                        Button("Explore all") { Task { await store.openFolder(last.root) } }
                    }
                    ForEach(store.children.prefix(10)) { item in
                        HStack {
                            Image(systemName: item.kind == "dir" ? "folder" : "doc").foregroundStyle(.secondary)
                            Button(item.name) { Task { await store.openFolder(item.path) } }
                                .buttonStyle(.plain).disabled(item.kind != "dir")
                            Spacer()
                            Text(DiskFormat.bytes(item.alloc)).monospacedDigit()
                        }
                        Divider()
                    }
                    Button("Review findings") { store.section = .findings }
                }.padding(28)
            }
        } else {
            EmptyState(title: "See where your disk space went",
                       message: "Choose a folder or scan this Mac. ebb reads files first; every cleanup has a separate review.",
                       symbol: "internaldrive") {
                if store.status?.fullDiskAccess == false {
                    Text("macOS hides protected folders until Full Disk Access is enabled.")
                        .font(.caption).foregroundStyle(.secondary)
                    Button("Full Disk Access…") { store.openPrivacySettings() }
                }
                HStack {
                    Button("Choose a folder…") { Task { await store.browseForScan() } }
                    Button("Scan this Mac") { Task { await store.scan() } }
                        .keyboardShortcut(.defaultAction)
                }.disabled(store.working)
            }
        }
    }

    private func metric(_ title: String, _ value: String) -> some View {
        VStack(alignment: .leading, spacing: 5) {
            Text(title).font(.callout).foregroundStyle(.secondary)
            Text(value).font(.system(size: 28, weight: .semibold, design: .rounded)).monospacedDigit()
        }
    }
}
