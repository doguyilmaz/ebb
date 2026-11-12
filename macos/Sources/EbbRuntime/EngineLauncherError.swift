import Foundation

public enum EngineLauncherError: Error, Equatable, LocalizedError, Sendable {
    case invalidSandboxHome
    case invalidSandboxPort
    case invalidServerRecord
    case serverProcessNotEbb
    case serverNotReady
    case helperNotFound
    case launchFailed(String)
    case processExited(Int32, String?)
    case readinessTimedOut
    case tokenMissing

    public var errorDescription: String? {
        switch self {
        case .invalidSandboxHome:
            "Sandbox mode requires EBB_SANDBOX=1 and an isolated EBB_HOME."
        case .invalidSandboxPort:
            "Sandbox mode requires a valid EBB_PORT between 1 and 65535."
        case .invalidServerRecord:
            "ebb wrote an invalid server status record."
        case .serverProcessNotEbb:
            "The recorded ebb server PID belongs to another program."
        case .serverNotReady:
            "The recorded ebb server did not answer its health check."
        case .helperNotFound:
            "The bundled ebb helper was not found at Contents/Helpers/ebb."
        case .launchFailed(let message):
            "Could not start the bundled ebb engine: \(message)"
        case .processExited(let status, let details):
            if let details, !details.isEmpty {
                "The ebb engine exited with status \(status): \(details)"
            } else {
                "The ebb engine exited with status \(status)."
            }
        case .readinessTimedOut:
            "The ebb engine did not become ready before the startup timeout."
        case .tokenMissing:
            "The ebb engine is ready but its local API token is missing."
        }
    }
}
