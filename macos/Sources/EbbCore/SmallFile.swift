import Foundation

public struct SmallFile: Codable, Sendable, Equatable, Identifiable {
    public let name: String
    public let size: UInt64
    public let modified: Int64

    public var id: String { name }

    public init(name: String, size: UInt64, modified: Int64) {
        self.name = name
        self.size = size
        self.modified = modified
    }
}
