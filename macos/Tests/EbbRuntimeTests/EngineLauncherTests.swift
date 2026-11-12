import Foundation
import XCTest
@testable import EbbRuntime

@MainActor
final class EngineLauncherTests: XCTestCase {
    func testUnrelatedAndOutOfRangeProcessesAreRejected() {
        XCTAssertFalse(EngineProcessIdentity.isEbb(pid: Int(ProcessInfo.processInfo.processIdentifier)))
        XCTAssertFalse(EngineProcessIdentity.isEbb(pid: Int.max))
        XCTAssertFalse(EngineProcessIdentity.isAlive(pid: Int.max))
    }

    func testMalformedServerRecordFailsBeforeReadingOrLaunching() async throws {
        let home = FileManager.default.temporaryDirectory
            .appendingPathComponent("ebb-runtime-\(UUID().uuidString)", isDirectory: true)
        let state = home.appendingPathComponent(".local/state/ebb", isDirectory: true)
        let bundle = home.appendingPathComponent("Bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: state, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: bundle, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: home) }
        try Data("not json".utf8).write(to: state.appendingPathComponent("server.json"))
        try Data("secret-token-that-must-not-be-read".utf8).write(to: state.appendingPathComponent("ui-token"))

        let launcher = EngineLauncher(bundleURL: bundle, environment: [
            "EBB_HOME": home.path,
            "EBB_SANDBOX": "1",
            "EBB_PORT": "47432",
        ])
        do {
            _ = try await launcher.connect()
            XCTFail("malformed status should stop before launch")
        } catch let error as EngineLauncherError {
            XCTAssertEqual(error, .invalidServerRecord)
            XCTAssertEqual(launcher.ownership, .none)
        }
    }

    func testStoppingWithoutAnOwnedProcessDoesNotChangeTheMachine() async {
        let launcher = EngineLauncher(environment: [
            "EBB_HOME": "/private/tmp/ebb-runtime-test",
            "EBB_SANDBOX": "1",
            "EBB_PORT": "47433",
        ])
        await launcher.stop()
        XCTAssertEqual(launcher.ownership, .none)
    }
}
