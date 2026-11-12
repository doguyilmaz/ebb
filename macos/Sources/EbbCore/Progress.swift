import Foundation

public struct Progress: Codable, Sendable, Equatable {
    public let dirs: UInt64
    public let files: UInt64
    public let bytes: UInt64
    public let problems: UInt64
    public let path: String
    public let elapsedMs: Int64

    public init(dirs: UInt64 = 0, files: UInt64 = 0, bytes: UInt64 = 0, problems: UInt64 = 0, path: String = "", elapsedMs: Int64 = 0) {
        self.dirs = dirs
        self.files = files
        self.bytes = bytes
        self.problems = problems
        self.path = path
        self.elapsedMs = elapsedMs
    }
}
