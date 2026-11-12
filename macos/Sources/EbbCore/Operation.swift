import Foundation

public struct Operation: Codable, Sendable, Equatable, Identifiable {
    public let id: String
    public let planId: String
    public let path: String
    public let action: String
    public let time: String
    public let bytes: UInt64
    public let result: String
    public let error: String?
    public let trashUrl: String?

    public init(id: String, planId: String, path: String, action: String, time: String, bytes: UInt64 = 0, result: String, error: String? = nil, trashUrl: String? = nil) {
        self.id = id
        self.planId = planId
        self.path = path
        self.action = action
        self.time = time
        self.bytes = bytes
        self.result = result
        self.error = error
        self.trashUrl = trashUrl
    }
}
