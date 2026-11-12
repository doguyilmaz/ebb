import Foundation

public struct Plan: Codable, Sendable, Equatable, Identifiable {
    public let id: String
    public let created: String
    public let expires: String
    public let source: String
    public let items: [PlanItem]
    public let bytes: UInt64
    public let needsApproval: Bool
    public let approved: Bool

    public init(id: String, created: String, expires: String, source: String, items: [PlanItem] = [], bytes: UInt64 = 0, needsApproval: Bool = false, approved: Bool = false) {
        self.id = id
        self.created = created
        self.expires = expires
        self.source = source
        self.items = items
        self.bytes = bytes
        self.needsApproval = needsApproval
        self.approved = approved
    }

    private enum CodingKeys: String, CodingKey { case id, created, expires, source, items, bytes, needsApproval, approved }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decode(String.self, forKey: .id)
        created = try c.decode(String.self, forKey: .created)
        expires = try c.decode(String.self, forKey: .expires)
        source = try c.decode(String.self, forKey: .source)
        items = try decodeArray(PlanItem.self, from: c, forKey: .items)
        bytes = try c.decode(UInt64.self, forKey: .bytes)
        needsApproval = try c.decodeIfPresent(Bool.self, forKey: .needsApproval) ?? false
        approved = try c.decodeIfPresent(Bool.self, forKey: .approved) ?? false
    }
}
