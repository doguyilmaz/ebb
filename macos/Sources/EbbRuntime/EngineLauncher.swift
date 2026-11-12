import EbbCore
import Foundation

@MainActor
public final class EngineLauncher {
    public private(set) var ownership: EngineOwnership = .none

    private let bundleURL: URL
    private let environment: [String: String]
    private let fileManager: FileManager
    private var connection: EbbClient?
    private var launchHandle: EngineLaunchHandle?

    public init(
        bundleURL: URL = Bundle.main.bundleURL,
        environment: [String: String] = ProcessInfo.processInfo.environment
    ) {
        self.bundleURL = bundleURL
        self.environment = environment
        self.fileManager = .default
    }

    public func connect() async throws -> EbbClient {
        let paths = try EnginePaths.resolve(environment: environment, fileManager: fileManager)

        if let handle = launchHandle {
            if handle.process.isRunning {
                // Reconnect to our child without relabelling it as an
                // existing daemon. This also handles a server.json write that
                // is still catching up after the process became healthy.
                try await waitForHealth(port: paths.port, process: handle.process)
                let token = try readToken(at: paths.token)
                let client = try EbbClient(baseURL: Self.baseURL(port: paths.port), token: token)
                connection = client
                ownership = .launchedByApp
                return client
            }
            finish(handle)
        }

        if let existing = try await connectToExisting(paths: paths) {
            connection = existing
            ownership = .reusedExisting
            return existing
        }

        let handle = try launch(paths: paths)
        launchHandle = handle
        ownership = .launchedByApp
        do {
            try await waitForHealth(port: paths.port, process: handle.process)
            let token = try readToken(at: paths.token)
            let client = try EbbClient(baseURL: Self.baseURL(port: paths.port), token: token)
            connection = client
            return client
        } catch {
            await stop()
            throw error
        }
    }

    public func stop() async {
        connection = nil
        guard let handle = launchHandle else {
            ownership = .none
            return
        }

        if handle.process.isRunning {
            handle.process.terminate()
            let deadline = Date().addingTimeInterval(2)
            while handle.process.isRunning && Date() < deadline {
                if Task.isCancelled { break }
                try? await Task.sleep(nanoseconds: 50_000_000)
            }
        }

        guard !handle.process.isRunning else {
            // Never force-kill an engine. A cleanup operation may still be
            // writing its result; the process remains owned until it exits.
            return
        }
        finish(handle)
    }

    private func connectToExisting(paths: EnginePaths) async throws -> EbbClient? {
        guard fileManager.fileExists(atPath: paths.serverRecord.path) else {
            return nil
        }
        let record: EngineServerRecord
        do {
            let data = try Data(contentsOf: paths.serverRecord)
            record = try JSONDecoder().decode(EngineServerRecord.self, from: data)
        } catch {
            throw EngineLauncherError.invalidServerRecord
        }
        guard record.pid > 0, (1...65_535).contains(record.port) else {
            throw EngineLauncherError.invalidServerRecord
        }
        guard EngineProcessIdentity.isAlive(pid: record.pid) else {
            return nil
        }
        guard EngineProcessIdentity.isEbb(pid: record.pid) else {
            throw EngineLauncherError.serverProcessNotEbb
        }

        do {
            try await waitForHealth(port: record.port, process: nil)
        } catch EngineLauncherError.readinessTimedOut {
            if !EngineProcessIdentity.isAlive(pid: record.pid) {
                return nil
            }
            throw EngineLauncherError.serverNotReady
        } catch EngineLauncherError.serverNotReady {
            throw EngineLauncherError.serverNotReady
        }

        let token = try readToken(at: paths.token)
        return try EbbClient(baseURL: Self.baseURL(port: record.port), token: token)
    }

    private func launch(paths: EnginePaths) throws -> EngineLaunchHandle {
        let helper = try EnginePaths.helperURL(bundleURL: bundleURL, fileManager: fileManager)
        let process = Process()
        process.executableURL = helper
        process.arguments = ["serve", "--port", String(paths.port)]
        process.environment = environment
        process.standardOutput = FileHandle.nullDevice

        let stderrURL = fileManager.temporaryDirectory
            .appendingPathComponent("ebb-engine-\(UUID().uuidString).stderr")
        guard fileManager.createFile(atPath: stderrURL.path, contents: nil) else {
            throw EngineLauncherError.launchFailed("could not create a temporary error log")
        }
        let stderrHandle: FileHandle
        do {
            stderrHandle = try FileHandle(forWritingTo: stderrURL)
        } catch {
            try? fileManager.removeItem(at: stderrURL)
            throw EngineLauncherError.launchFailed(error.localizedDescription)
        }
        process.standardError = stderrHandle
        do {
            try process.run()
        } catch {
            try? stderrHandle.close()
            try? fileManager.removeItem(at: stderrURL)
            throw EngineLauncherError.launchFailed(error.localizedDescription)
        }
        return EngineLaunchHandle(process: process, stderrURL: stderrURL, stderrHandle: stderrHandle)
    }

    private func waitForHealth(port: Int, process: Process?) async throws {
        let deadline = Date().addingTimeInterval(10)
        while Date() < deadline {
            try Task.checkCancellation()
            if let process, !process.isRunning {
                throw processError(process)
            }
            do {
                let health = try await fetchHealth(port: port)
                guard health.ok, health.port == port else {
                    throw EngineLauncherError.serverNotReady
                }
                return
            } catch is CancellationError {
                throw CancellationError()
            } catch EngineLauncherError.serverNotReady {
                // A listener may be present before the server has completed
                // its own initialization. Keep the bounded startup wait.
            } catch {
                // Connection refusal and an incomplete response are expected
                // while a newly launched engine is coming up.
            }
            try await Task.sleep(nanoseconds: 100_000_000)
        }
        throw EngineLauncherError.readinessTimedOut
    }

    private func fetchHealth(port: Int) async throws -> EngineHealth {
        var request = URLRequest(url: Self.healthURL(port: port))
        request.httpMethod = "GET"
        request.timeoutInterval = 1
        let (data, response) = try await URLSession.shared.data(for: request)
        guard let response = response as? HTTPURLResponse, response.statusCode == 200 else {
            throw EngineLauncherError.serverNotReady
        }
        return try JSONDecoder().decode(EngineHealth.self, from: data)
    }

    private func readToken(at url: URL) throws -> String {
        guard let raw = try? String(contentsOf: url, encoding: .utf8) else {
            throw EngineLauncherError.tokenMissing
        }
        let token = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        guard token.count >= 32,
              token.rangeOfCharacter(from: .whitespacesAndNewlines) == nil else {
            throw EngineLauncherError.tokenMissing
        }
        return token
    }

    private func processError(_ process: Process) -> EngineLauncherError {
        let details: String?
        if let handle = launchHandle {
            try? handle.stderrHandle.synchronize()
            if let data = readStderrTail(from: handle.stderrURL), !data.isEmpty {
                let text = String(decoding: data.suffix(4_096), as: UTF8.self)
                    .trimmingCharacters(in: .whitespacesAndNewlines)
                details = text.isEmpty ? nil : text
            } else {
                details = nil
            }
        } else {
            details = nil
        }
        return .processExited(process.terminationStatus, details)
    }

    private func readStderrTail(from url: URL) -> Data? {
        guard let handle = try? FileHandle(forReadingFrom: url) else { return nil }
        defer { try? handle.close() }
        do {
            let end = try handle.seekToEnd()
            let start = end > 4_096 ? end - 4_096 : 0
            try handle.seek(toOffset: start)
            return try handle.readToEnd() ?? Data()
        } catch {
            return nil
        }
    }

    private func finish(_ handle: EngineLaunchHandle) {
        try? handle.stderrHandle.close()
        try? fileManager.removeItem(at: handle.stderrURL)
        if launchHandle?.process === handle.process {
            launchHandle = nil
            ownership = .none
        }
    }

    private static func baseURL(port: Int) -> URL {
        URL(string: "http://127.0.0.1:\(port)/api/v1")!
    }

    private static func healthURL(port: Int) -> URL {
        URL(string: "http://127.0.0.1:\(port)/api/v1/health")!
    }
}
