import Foundation

public struct PlanItem: Codable, Sendable, Equatable {
    public let path: String
    public let title: String?
    public let bytes: UInt64
    public let safety: String
    public let action: FindingAction
    public let command: [String]
    public let findingId: String?
    public let warnings: [String]
    public let blocked: String?

    public init(path: String, title: String? = nil, bytes: UInt64 = 0, safety: String, action: FindingAction, command: [String] = [], findingId: String? = nil, warnings: [String] = [], blocked: String? = nil) {
        self.path = path
        self.title = title
        self.bytes = bytes
        self.safety = safety
        self.action = action
        self.command = command
        self.findingId = findingId
        self.warnings = warnings
        self.blocked = blocked
    }

    private enum CodingKeys: String, CodingKey { case path, title, bytes, safety, action, command, findingId, warnings, blocked }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        path = try c.decode(String.self, forKey: .path)
        title = try c.decodeIfPresent(String.self, forKey: .title)
        bytes = try c.decode(UInt64.self, forKey: .bytes)
        safety = try c.decode(String.self, forKey: .safety)
        action = try c.decode(FindingAction.self, forKey: .action)
        command = try decodeArray(String.self, from: c, forKey: .command)
        findingId = try c.decodeIfPresent(String.self, forKey: .findingId)
        warnings = try decodeArray(String.self, from: c, forKey: .warnings)
        blocked = try c.decodeIfPresent(String.self, forKey: .blocked)
    }
}
