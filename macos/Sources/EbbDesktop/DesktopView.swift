import EbbCore
import SwiftUI

struct DesktopView: View {
    @ObservedObject var store: EbbStore

    var body: some View {
        Group {
            switch store.connection {
            case .opening:
                VStack(spacing: 16) {
                    Image(nsImage: NSApplication.shared.applicationIconImage)
                        .resizable().frame(width: 72, height: 72)
                    Text("Opening ebb…").font(.title2)
                    ProgressView().controlSize(.small)
                    Text("Connecting to the engine on this Mac").foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            case .failed(let message):
                EmptyState(title: "ebb could not open", message: message, symbol: "exclamationmark.triangle") {
                    Button("Try again") { Task { await store.connect() } }
                        .keyboardShortcut(.defaultAction)
                }
            case .ready:
                NavigationView {
                    List(selection: Binding<SidebarSection?>(
                        get: { store.section },
                        set: { if let section = $0 { store.section = section } }
                    )) {
                        ForEach(SidebarSection.allCases) { section in
                            Label(section.rawValue, systemImage: section.symbol)
                                .padding(.vertical, 4)
                                .tag(section)
                        }
                        if !store.pendingPlans.isEmpty {
                            Section("Waiting for approval") {
                                ForEach(store.pendingPlans) { plan in
                                    Button("Review \(DiskFormat.bytes(plan.bytes))") { store.reviewExternal(plan) }
                                }
                            }
                        }
                    }
                    .listStyle(.sidebar)
                    .frame(minWidth: 190, idealWidth: 210, maxWidth: 260)
                    VStack(spacing: 0) {
                        if store.scanRunning {
                            HStack(spacing: 12) {
                                ProgressView().controlSize(.small)
                                VStack(alignment: .leading, spacing: 3) {
                                    Text("Scanning \(store.status?.scan.root ?? store.status?.defaultRoot ?? "this Mac")")
                                    if let p = store.progress {
                                        Text("\(p.files.formatted()) files · \(DiskFormat.bytes(p.bytes))")
                                            .font(.caption).foregroundStyle(.secondary)
                                    }
                                }
                                Spacer()
                                Button("Stop scan") { Task { await store.stopScan() } }
                            }
                            .padding(12).background(Color.accentColor.opacity(0.08))
                        }
                        if let warning = store.eventWarning {
                            HStack {
                                Text(warning).font(.caption)
                                Spacer()
                                Button("Reconnect") { Task { await store.reconnect() } }
                            }.padding(10).background(Color.orange.opacity(0.12))
                        }
                        detail
                            .frame(maxWidth: .infinity, maxHeight: .infinity)
                    }
                    .navigationTitle(store.section.rawValue)
                    .toolbar {
                        ToolbarItemGroup {
                            if store.loading { ProgressView().controlSize(.small) }
                            Button { Task { await store.refresh() } } label: {
                                Label("Refresh", systemImage: "arrow.clockwise")
                            }.help("Refresh the current view")
                            Menu {
                                Button("Scan again") { Task { await store.scan() } }
                                Button("Choose a folder…") { Task { await store.browseForScan() } }
                            } label: {
                                Label("Scan", systemImage: "magnifyingglass")
                            }.disabled(store.working)
                        }
                    }
                }
                .task(id: store.section) { await store.load() }
            }
        }
        .sheet(item: $store.reviewPlan) { plan in
            PlanReviewView(store: store, plan: plan, approveOnly: store.reviewOnly)
        }
        .alert("Could not finish", isPresented: Binding(
            get: { store.error != nil },
            set: { if !$0 { store.error = nil } }
        )) {
            Button("OK") { store.error = nil }
        } message: {
            Text(store.error ?? "")
        }
    }

    @ViewBuilder private var detail: some View {
        switch store.section {
        case .overview: OverviewView(store: store)
        case .explore: ExploreView(store: store)
        case .findings: FindingsView(store: store)
        case .apps: InstalledAppsView(store: store)
        case .duplicates: DuplicatesView(store: store)
        case .history: HistoryView(store: store)
        case .activity: ActivityView(store: store)
        case .settings: PreferencesView(store: store).padding(24)
        }
    }
}
