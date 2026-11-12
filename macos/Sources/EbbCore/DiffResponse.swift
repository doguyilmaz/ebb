import Foundation

public struct DiffResponse: Codable, Sendable, Equatable {
    public let from: Meta?
    public let to: Meta
    public let changes: [Change]

    public init(from: Meta? = nil, to: Meta, changes: [Change] = []) {
        self.from = from
        self.to = to
        self.changes = changes
    }

    private enum CodingKeys: String, CodingKey { case from, to, changes }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        from = try c.decodeIfPresent(Meta.self, forKey: .from)
        to = try c.decode(Meta.self, forKey: .to)
        changes = try decodeArray(Change.self, from: c, forKey: .changes)
    }
}
