import AppKit
import Combine
import EbbCore
import EbbRuntime
import Foundation

@MainActor
final class EbbStore: ObservableObject {
    @Published var connection: EbbConnection = .opening
    @Published var section: SidebarSection = .overview
    @Published var status: Status?
    @Published var volume: StorageBreakdown?
    @Published var findings: [FindingView] = []
    @Published var reclaimable: UInt64 = 0
    @Published var children: [FileItem] = []
    @Published var path = ""
    @Published var apps: [InstalledApp] = []
    @Published var duplicates: DuplicatesState?
    @Published var history: [Meta] = []
    @Published var operations: [EbbCore.Operation] = []
    @Published var pendingPlans: [Plan] = []
    @Published var settings: EbbCore.Settings?
    @Published var progress: EbbCore.Progress?
    @Published var selectedFindings: Set<String> = []
    @Published var reviewPlan: Plan?
    @Published var reviewOnly = false
    @Published var cleaning = false
    @Published var loading = false
    @Published var error: String?
    @Published var eventWarning: String?
    private let launcher = EngineLauncher()
    private var eventsTask: Task<Void, Never>?
    private var pollingTask: Task<Void, Never>?

    var client: EbbClient? {
        if case .ready(let client) = connection { return client }
        return nil
    }

    var scanRunning: Bool { status?.scan.running == true || progress != nil }
    var duplicateRunning: Bool { duplicates?.running == true }
    var working: Bool { scanRunning || duplicateRunning || cleaning }

    func connect() async {
        guard client == nil else { return }
        connection = .opening
        do {
            let client = try await launcher.connect()
            let initial: Status = try await client.get("/status")
            accept(initial)
            connection = .ready(client)
            let pending: PendingPlans = try await client.get("/plans")
            pendingPlans = pending.plans
            await load()
            observe(client)
        } catch is CancellationError {
            await launcher.stop()
        } catch {
            if Task.isCancelled {
                await launcher.stop()
                return
            }
            connection = .failed(error.localizedDescription)
        }
    }

    func stop() async {
        eventsTask?.cancel()
        pollingTask?.cancel()
        await launcher.stop()
    }

    func refresh() async {
        guard let client else { return }
        do {
            accept(try await client.get("/status"))
            let pending: PendingPlans = try await client.get("/plans")
            pendingPlans = pending.plans
        } catch {
            if !Task.isCancelled { self.error = error.localizedDescription }
        }
        await load()
    }

    func load() async {
        guard let client else { return }
        let requested = section
        loading = true
        defer { loading = false }
        do {
            switch requested {
            case .overview:
                guard status?.last != nil else { return }
                async let disk: StorageBreakdown = client.get("/volume")
                async let found: FindingsResponse = client.get("/findings")
                async let top: ChildrenResponse = client.get("/children?limit=10")
                let values = try await (disk, found, top)
                volume = values.0
                findings = values.1.findings
                reclaimable = values.1.reclaimable
                if section == requested { children = values.2.items }
            case .explore:
                guard status?.last != nil else { return }
                let requestedPath = path
                let query = path.isEmpty ? "/children?limit=2000" : DiskFormat.query("/children", ["path": path, "limit": "2000"])
                let response: ChildrenResponse = try await client.get(query)
                if section == requested && path == requestedPath {
                    path = response.path
                    children = response.items
                }
            case .findings:
                guard status?.last != nil else { return }
                let response: FindingsResponse = try await client.get("/findings")
                findings = response.findings
                reclaimable = response.reclaimable
                selectedFindings.formIntersection(Set(findings.map(\.id)))
            case .apps:
                guard status?.last != nil else { return }
                let response: AppsResponse = try await client.get("/apps")
                apps = response.apps
            case .duplicates:
                duplicates = try await client.get("/duplicates")
            case .history:
                let response: HistoryResponse = try await client.get("/history")
                history = response.snapshots
            case .activity:
                let response: OpsResponse = try await client.get("/ops?limit=500")
                operations = response.ops
            case .settings:
                await loadSettings()
            }
        } catch is CancellationError {
        } catch {
            if !Task.isCancelled { self.error = error.localizedDescription }
        }
    }

    func loadSettings() async {
        guard let client else { return }
        do { settings = try await client.get("/settings") }
        catch { self.error = error.localizedDescription }
    }

    func scan(root: String? = nil) async {
        guard let client, !working else { return }
        do {
            let _: ScanState = try await client.post("/scans", body: ScanRequest(root: root))
            status = try await client.get("/status")
        } catch { self.error = error.localizedDescription }
    }

    func stopScan() async {
        guard let client else { return }
        do {
            let _: ScanState = try await client.delete("/scans/current")
            await refresh()
        } catch { self.error = error.localizedDescription }
    }

    func browseForScan() async {
        let picker = NSOpenPanel()
        picker.canChooseDirectories = true
        picker.canChooseFiles = false
        picker.allowsMultipleSelection = false
        picker.prompt = "Scan folder"
        guard await picker.begin() == .OK, let url = picker.url else { return }
        await scan(root: url.path)
    }

    func openFolder(_ path: String) async {
        section = .explore
        self.path = path
        await load()
    }

    func plan(_ request: PlanRequest) async {
        guard let client, !working else { return }
        do {
            reviewOnly = false
            reviewPlan = try await client.post("/plans", body: request)
        } catch { self.error = error.localizedDescription }
    }

    func reviewExternal(_ plan: Plan) {
        reviewOnly = true
        reviewPlan = plan
    }

    func execute(_ plan: Plan, approveOnly: Bool) async throws -> PlanResult? {
        guard let client else { throw URLError(.notConnectedToInternet) }
        cleaning = true
        defer { cleaning = false }
        let _: Plan = try await client.post("/plans/\(plan.id)/approve")
        if approveOnly {
            await refresh()
            return nil
        }
        let result: PlanResult = try await client.post("/plans/\(plan.id)/execute")
        selectedFindings.removeAll()
        await refresh()
        return result
    }

    func findDuplicates(minSize: UInt64) async {
        guard let client, !working else { return }
        do {
            duplicates = try await client.post("/duplicates", body: DuplicatesRequest(minSize: minSize))
        } catch { self.error = error.localizedDescription }
    }

    func stopDuplicates() async {
        guard let client else { return }
        do { duplicates = try await client.delete("/duplicates/current") }
        catch { self.error = error.localizedDescription }
    }

    func undo(_ operation: EbbCore.Operation) async {
        guard let client, !working else { return }
        do {
            let _: EmptyResponse = try await client.post("/ops/\(operation.id)/undo")
            await refresh()
        } catch { self.error = error.localizedDescription }
    }

    func saveSettings(_ settings: EbbCore.Settings) async {
        guard let client else { return }
        do { self.settings = try await client.post("/settings", body: settings) }
        catch { self.error = error.localizedDescription }
    }

    func reveal(_ path: String) {
        NSWorkspace.shared.activateFileViewerSelecting([URL(fileURLWithPath: path)])
    }

    func openPrivacySettings() {
        if let url = URL(string: "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles") {
            NSWorkspace.shared.open(url)
        }
    }

    private func observe(_ client: EbbClient) {
        eventsTask?.cancel()
        pollingTask?.cancel()
        eventsTask = Task {
            for attempt in 0..<5 {
                do {
                    let events = try await client.events()
                    eventWarning = nil
                    for try await event in events {
                        try Task.checkCancellation()
                        if event.name == "scan.progress" {
                            progress = try event.decode(EbbCore.Progress.self)
                        } else if ["scan.done", "findings.changed", "plan.done", "plans.changed", "duplicates.done", "history.changed"].contains(event.name) {
                            if event.name == "scan.done" { progress = nil }
                            await refresh()
                        }
                    }
                } catch is CancellationError {
                    return
                } catch {
                    if Task.isCancelled { return }
                    eventWarning = "Live updates disconnected. Reconnecting…"
                }
                do { try await Task.sleep(nanoseconds: UInt64(1 << attempt) * 1_000_000_000) }
                catch { return }
            }
            eventWarning = "Live updates are unavailable. Reconnect to restore them."
        }
        pollingTask = Task {
            while !Task.isCancelled {
                do {
                    try await Task.sleep(nanoseconds: 10_000_000_000)
                    accept(try await client.get("/status"))
                } catch is CancellationError { return }
                catch { eventWarning = "ebb is not answering. Reconnect to try again." }
            }
        }
    }

    func reconnect() async {
        eventsTask?.cancel()
        pollingTask?.cancel()
        connection = .opening
        await connect()
    }

    private func accept(_ newStatus: Status) {
        if status?.last?.root != newStatus.last?.root {
            path = ""
            children = []
            selectedFindings.removeAll()
        }
        status = newStatus
    }
}
