import EbbCore
import EbbRuntime
import Darwin
import Foundation

@main
struct EbbNativeSmoke {
    static func main() async {
        do {
            try await run()
        } catch {
            FileHandle.standardError.write(Data("native smoke failed: \(error.localizedDescription)\n".utf8))
            exit(1)
        }
    }

    private static func run() async throws {
        let env = ProcessInfo.processInfo.environment
        guard env["EBB_SANDBOX"] == "1", let home = env["EBB_HOME"],
              let rawPort = env["EBB_PORT"], let port = Int(rawPort), (1...65535).contains(port) else {
            throw failure("A separate EBB_HOME, EBB_SANDBOX=1 and EBB_PORT are required")
        }
        let root = URL(fileURLWithPath: home).resolvingSymlinksInPath()
        let temp = FileManager.default.temporaryDirectory.resolvingSymlinksInPath().path
        try check(root.path.hasPrefix(temp + "/") || root.path.hasPrefix("/private/tmp/"), "The fixture must be in a temporary directory")
        let token = try String(contentsOf: root.appendingPathComponent(".local/state/ebb/ui-token"), encoding: .utf8)
            .trimmingCharacters(in: .whitespacesAndNewlines)
        let client = try EbbClient(baseURL: URL(string: "http://127.0.0.1:\(port)/api/v1")!, token: token)
        var status: Status = try await client.get("/status")
        try check(status.sandboxed, "Refusing to test a live engine")
        try check(URL(fileURLWithPath: status.defaultRoot).resolvingSymlinksInPath() == root, "Engine root is not the fixture")
        print("ok    sandbox connection and authentication")
        let launcher = await EngineLauncher(environment: env)
        let discovered = try await launcher.connect()
        let ownership = await launcher.ownership
        try check(ownership == .reusedExisting, "The launcher did not reuse the fixture server")
        let _: Status = try await discovered.get("/status")
        await launcher.stop()
        let _: Status = try await client.get("/status")
        print("ok    process discovery and leaving an existing engine running")

        let unusual = root.appendingPathComponent("Documents/A+B & notes", isDirectory: true)
        try FileManager.default.createDirectory(at: unusual, withIntermediateDirectories: true)
        try Data(repeating: 7, count: 2 << 20).write(to: unusual.appendingPathComponent("report#1.bin"))
        let _: ScanState = try await client.post("/scans", body: ScanRequest(root: home, full: true))
        status = try await waitForScan(client)
        try check((status.last?.files ?? 0) > 0, "The scan did not return files")
        print("ok    native scan request and completed status")

        let findings: FindingsResponse = try await client.get("/findings")
        try check(!findings.findings.isEmpty, "The fixture should have findings")
        let _: StorageBreakdown = try await client.get("/volume")
        let apps: AppsResponse = try await client.get("/apps")
        try check(apps.apps.contains { $0.bundleId == "com.example.Sketchpad" }, "The fixture app is missing")
        let history: HistoryResponse = try await client.get("/history")
        try check(!history.snapshots.isEmpty, "The finished scan was not saved")
        var query = URLComponents()
        query.path = "/children"
        query.queryItems = [URLQueryItem(name: "path", value: unusual.path)]
        query.percentEncodedQuery = query.percentEncodedQuery?.replacingOccurrences(of: "+", with: "%2B")
        let children: ChildrenResponse = try await client.get(query.string!)
        try check(children.items.contains { $0.name == "report#1.bin" }, "Special characters did not survive the request")
        print("ok    findings, volume, apps, history and escaped folder requests")

        let installer = root.appendingPathComponent("Downloads/Setup-2.3.dmg").path
        let plan: Plan = try await client.post("/plans", body: PlanRequest(paths: [installer], source: "ui"))
        try check(plan.needsApproval, "Personal files must need approval")
        try check(FileManager.default.fileExists(atPath: installer), "Planning changed the fixture")
        do {
            let _: PlanResult = try await client.post("/plans/\(plan.id)/execute")
            throw failure("An unapproved personal-file plan ran")
        } catch EbbClientError.http(let status, _, let code) {
            try check(status == 409 && code == "needs-approval", "The approval error code was lost")
        }
        let _: Plan = try await client.post("/plans/\(plan.id)/approve")
        let result: PlanResult = try await client.post("/plans/\(plan.id)/execute")
        try check(result.done == 1 && !FileManager.default.fileExists(atPath: installer), "Approved cleanup did not move the file")
        let ops: OpsResponse = try await client.get("/ops")
        guard let operation = ops.ops.first(where: { $0.planId == plan.id && $0.action == "trash" }) else {
            throw failure("Cleanup did not write its operation")
        }
        let _: EmptyResponse = try await client.post("/ops/\(operation.id)/undo")
        try check(FileManager.default.fileExists(atPath: installer), "Undo did not restore the file")
        print("ok    plan, approval boundary, Trash, operation log and undo")

        let _: DuplicatesState = try await client.post("/duplicates", body: DuplicatesRequest(minSize: 1 << 20))
        let deadline = Date().addingTimeInterval(30)
        var duplicates: DuplicatesState = try await client.get("/duplicates")
        while duplicates.running && Date() < deadline {
            try await Task.sleep(nanoseconds: 100_000_000)
            duplicates = try await client.get("/duplicates")
        }
        try check(!duplicates.running && !(duplicates.result?.groups.isEmpty ?? true), "Duplicate search did not return the fixture copies")
        let settings: Settings = try await client.get("/settings")
        let _: Settings = try await client.post("/settings", body: settings)
        let _: PendingPlans = try await client.get("/plans")
        print("ok    duplicate search, settings and empty pending-plan arrays")

        let stream = try await client.events()
        let name = try await withThrowingTaskGroup(of: String.self) { group in
            group.addTask {
                for try await event in stream { return event.name }
                throw failure("The event stream ended without hello")
            }
            group.addTask {
                try await Task.sleep(nanoseconds: 5_000_000_000)
                throw failure("The event stream did not answer")
            }
            let name = try await group.next()!
            group.cancelAll()
            return name
        }
        try check(name == "hello", "The event stream did not send hello")
        print("ok    authenticated live event stream")
        print("all native integration checks passed")
    }

    private static func waitForScan(_ client: EbbClient) async throws -> Status {
        let deadline = Date().addingTimeInterval(30)
        while Date() < deadline {
            let status: Status = try await client.get("/status")
            if !status.scan.running {
                if let error = status.scan.error, !error.isEmpty { throw failure(error) }
                return status
            }
            try await Task.sleep(nanoseconds: 100_000_000)
        }
        throw failure("The fixture scan did not finish")
    }

    private static func check(_ condition: Bool, _ message: String) throws {
        if !condition { throw failure(message) }
    }

    private static func failure(_ message: String) -> NSError {
        NSError(domain: "EbbNativeSmoke", code: 1, userInfo: [NSLocalizedDescriptionKey: message])
    }
}
