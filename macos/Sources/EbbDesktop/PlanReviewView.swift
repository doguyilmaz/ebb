import EbbCore
import SwiftUI

struct PlanReviewView: View {
    @ObservedObject var store: EbbStore
    let plan: Plan
    let approveOnly: Bool
    @Environment(\.dismiss) private var dismiss
    @State private var confirmed = false
    @State private var running = false
    @State private var finished = false
    @State private var result: PlanResult?
    @State private var error: String?

    private var runnable: Int { plan.items.filter { $0.blocked == nil && $0.action.kind != "manual" }.count }

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text(finished ? (approveOnly ? "Plan approved" : "Cleanup finished") : "Review the cleanup")
                .font(.title2.weight(.semibold))
            if let result {
                Text("\(result.done) done, \(result.refused) refused, \(result.failed) failed · about \(DiskFormat.bytes(result.freed))")
                ForEach(result.ops) { operation in
                    VStack(alignment: .leading, spacing: 5) {
                        Text(operation.path).font(.caption.monospaced()).textSelection(.enabled)
                        Text(operation.result + (operation.error.map { ": " + $0 } ?? ""))
                            .foregroundStyle(operation.result == "done" ? Color.secondary : Color.red)
                    }
                }
                if result.ops.contains(where: { $0.action == "trash" && $0.result == "done" }) {
                    Text("Items in the Trash can be put back from Activity. Their space is freed when the Trash is emptied.")
                        .font(.caption).foregroundStyle(.secondary)
                }
            } else if finished {
                Text("Nothing has been touched. The client that made this plan can now execute it.")
            } else {
                Text("\(runnable) \(runnable == 1 ? "item" : "items") · about \(DiskFormat.bytes(plan.bytes))").foregroundStyle(.secondary)
                ScrollView {
                    VStack(alignment: .leading, spacing: 14) {
                        ForEach(Array(plan.items.enumerated()), id: \.offset) { _, item in
                            VStack(alignment: .leading, spacing: 5) {
                                HStack {
                                    Text(item.action.kind.capitalized).font(.headline)
                                    Text(item.safety).font(.caption).foregroundStyle(.secondary)
                                    Spacer()
                                    Text(DiskFormat.bytes(item.bytes)).monospacedDigit()
                                }
                                Text(item.path).font(.caption.monospaced()).textSelection(.enabled)
                                if let blocked = item.blocked { Text(blocked).foregroundStyle(.red) }
                                ForEach(item.warnings, id: \.self) { Text($0).font(.caption).foregroundStyle(.secondary) }
                                if !item.command.isEmpty { Text(item.command.joined(separator: " ")).font(.caption.monospaced()) }
                                if let hint = item.action.hint { Text(hint).font(.caption).foregroundStyle(.secondary) }
                            }
                            Divider()
                        }
                    }
                }.frame(minHeight: 160, maxHeight: 360)
                if plan.needsApproval {
                    Toggle("I checked these items; some cannot be rebuilt by a tool.", isOn: $confirmed)
                }
                Text("Each item is checked again before it is touched. Moving to Trash can be undone; deleting or running a tool cannot.")
                    .font(.caption).foregroundStyle(.secondary)
            }
            if let error { Text(error).foregroundStyle(.red).textSelection(.enabled) }
            HStack {
                if running { ProgressView().controlSize(.small); Text("Finishing the cleanup…") }
                Spacer()
                Button(finished ? "Close" : "Cancel") { dismiss() }.disabled(running)
                if !finished {
                    Button(approveOnly ? "Approve plan" : "Clean \(DiskFormat.bytes(plan.bytes))") { Task { await run() } }
                        .keyboardShortcut(.defaultAction)
                        .disabled(running || runnable == 0 || (plan.needsApproval && !confirmed))
                }
            }
        }
        .padding(24).frame(width: 650)
        .interactiveDismissDisabled(running)
    }

    private func run() async {
        running = true
        error = nil
        do {
            result = try await store.execute(plan, approveOnly: approveOnly)
            finished = true
        } catch { self.error = error.localizedDescription }
        running = false
    }
}
