import Foundation

public struct DuplicatesRequest: Codable, Sendable, Equatable {
    public let path: String?
    public let minSize: UInt64?

    public init(path: String? = nil, minSize: UInt64? = nil) {
        self.path = path
        self.minSize = minSize
    }
}
