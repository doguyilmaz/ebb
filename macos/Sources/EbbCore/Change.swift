import Foundation

public struct Change: Codable, Sendable, Equatable, Identifiable {
    public let path: String
    public let before: UInt64
    public let after: UInt64
    public let delta: Int64

    public var id: String { path }

    public init(path: String, before: UInt64 = 0, after: UInt64 = 0, delta: Int64 = 0) {
        self.path = path
        self.before = before
        self.after = after
        self.delta = delta
    }
}
