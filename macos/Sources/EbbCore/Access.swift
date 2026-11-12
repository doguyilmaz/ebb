import Foundation

public struct Access: Codable, Sendable, Equatable {
    public let kind: String
    public let path: String?
    public let name: String?

    public init(kind: String, path: String? = nil, name: String? = nil) {
        self.kind = kind
        self.path = path
        self.name = name
    }
}
