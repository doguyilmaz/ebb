import EbbCore
import SwiftUI

struct FindingsView: View {
    @ObservedObject var store: EbbStore
    @State private var filter = ""

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Text("\(DiskFormat.bytes(store.reclaimable)) to review").font(.headline)
                Spacer()
                TextField("Filter findings", text: $filter).frame(width: 200)
            }.padding(18)
            List(selection: $store.selectedFindings) {
                ForEach(store.findings.filter { filter.isEmpty || $0.title.localizedCaseInsensitiveContains(filter) }) { finding in
                    VStack(alignment: .leading, spacing: 7) {
                        HStack {
                            Text(finding.title).font(.headline)
                            Spacer()
                            Text(finding.safety.capitalized).font(.caption).foregroundStyle(.secondary)
                            Text(DiskFormat.bytes(finding.bytes)).monospacedDigit()
                        }
                        Text(finding.why).font(.callout).foregroundStyle(.secondary)
                        Text(finding.impact).font(.caption)
                        ForEach(finding.targets, id: \.path) { target in
                            Text(target.path).font(.caption.monospaced()).foregroundStyle(.secondary)
                                .lineLimit(1).truncationMode(.middle).textSelection(.enabled)
                        }
                    }.padding(.vertical, 8).tag(finding.id)
                }
            }
            HStack {
                Text("\(store.selectedFindings.count) selected").foregroundStyle(.secondary)
                Spacer()
                Button("Review cleanup…") {
                    Task { await store.plan(.init(findingIds: Array(store.selectedFindings), source: "ui")) }
                }.disabled(store.selectedFindings.isEmpty || store.working)
            }.padding(16)
        }
    }
}
