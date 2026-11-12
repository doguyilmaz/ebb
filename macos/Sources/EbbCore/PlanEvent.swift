import Foundation

public struct PlanEvent: Codable, Sendable, Equatable {
    public let item: Int
    public let path: String
    public let state: String
    public let bytes: UInt64?
    public let err: String?

    public init(item: Int, path: String, state: String, bytes: UInt64? = nil, err: String? = nil) {
        self.item = item
        self.path = path
        self.state = state
        self.bytes = bytes
        self.err = err
    }
}
