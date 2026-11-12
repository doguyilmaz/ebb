import Foundation

public struct Status: Codable, Sendable, Equatable {
    public let version: String
    public let scan: ScanState
    public let last: Meta?
    public let defaultRoot: String
    public let fullDiskAccess: Bool
    public let access: Access
    public let askAvailable: Bool
    public let sandboxed: Bool
    public let changes: Changes?

    public init(version: String, scan: ScanState, last: Meta? = nil, defaultRoot: String, fullDiskAccess: Bool, access: Access, askAvailable: Bool, sandboxed: Bool, changes: Changes? = nil) {
        self.version = version
        self.scan = scan
        self.last = last
        self.defaultRoot = defaultRoot
        self.fullDiskAccess = fullDiskAccess
        self.access = access
        self.askAvailable = askAvailable
        self.sandboxed = sandboxed
        self.changes = changes
    }
}
