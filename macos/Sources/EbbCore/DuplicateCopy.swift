import Foundation

public struct DuplicateCopy: Codable, Sendable, Equatable, Identifiable {
    public let path: String
    public let mtime: Int64
    public let bytes: UInt64

    public var id: String { path }

    public init(path: String, mtime: Int64 = 0, bytes: UInt64 = 0) {
        self.path = path
        self.mtime = mtime
        self.bytes = bytes
    }
}
