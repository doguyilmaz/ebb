import Foundation
import XCTest
@testable import EbbRuntime

final class EnginePathsTests: XCTestCase {
    func testSandboxRequiresAnIsolatedHomeAndExplicitPort() throws {
        let home = FileManager.default.temporaryDirectory
            .appendingPathComponent("ebb-runtime-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: home, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: home) }

        let environment = [
            "EBB_SANDBOX": "1",
            "EBB_HOME": home.path,
            "EBB_PORT": "47431",
        ]
        let paths = try EnginePaths.resolve(environment: environment)
        XCTAssertEqual(paths.port, 47_431)
        XCTAssertEqual(paths.stateDirectory.path, home.appendingPathComponent(".local/state/ebb").path)
    }

    func testSandboxWithoutHomeOrPortIsRejected() {
        XCTAssertThrowsError(try EnginePaths.resolve(environment: ["EBB_SANDBOX": "1"])) { error in
            XCTAssertEqual(error as? EngineLauncherError, .invalidSandboxHome)
        }

        let home = FileManager.default.temporaryDirectory
            .appendingPathComponent("ebb-runtime-\(UUID().uuidString)", isDirectory: true)
        try? FileManager.default.createDirectory(at: home, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: home) }
        XCTAssertThrowsError(try EnginePaths.resolve(environment: [
            "EBB_SANDBOX": "1",
            "EBB_HOME": home.path,
        ])) { error in
            XCTAssertEqual(error as? EngineLauncherError, .invalidSandboxPort)
        }
    }

    func testHomeOverrideWithoutSandboxIsRejected() {
        let home = FileManager.default.temporaryDirectory
            .appendingPathComponent("ebb-runtime-\(UUID().uuidString)", isDirectory: true)
        try? FileManager.default.createDirectory(at: home, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: home) }
        XCTAssertThrowsError(try EnginePaths.resolve(environment: ["EBB_HOME": home.path])) { error in
            XCTAssertEqual(error as? EngineLauncherError, .invalidSandboxHome)
        }
    }

    func testHelperMustBeBundledAsEbb() throws {
        let bundle = FileManager.default.temporaryDirectory
            .appendingPathComponent("ebb-runtime-\(UUID().uuidString)", isDirectory: true)
        let helpers = bundle.appendingPathComponent("Contents/Helpers", isDirectory: true)
        try FileManager.default.createDirectory(at: helpers, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: bundle) }

        let wrong = helpers.appendingPathComponent("ebb-engine")
        XCTAssertTrue(FileManager.default.createFile(atPath: wrong.path, contents: Data()))
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: wrong.path)
        XCTAssertThrowsError(try EnginePaths.helperURL(bundleURL: bundle)) { error in
            XCTAssertEqual(error as? EngineLauncherError, .helperNotFound)
        }

        let helper = helpers.appendingPathComponent("ebb")
        XCTAssertTrue(FileManager.default.createFile(atPath: helper.path, contents: Data()))
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: helper.path)
        XCTAssertEqual(try EnginePaths.helperURL(bundleURL: bundle), helper)
    }
}
