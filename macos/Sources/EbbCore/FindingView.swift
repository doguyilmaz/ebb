import Foundation

public struct FindingView: Codable, Sendable, Equatable, Identifiable {
    public let id: String
    public let ruleId: String
    public let title: String
    public let category: String
    public let owner: String?
    public let safety: String
    public let bytes: UInt64
    public let alloc: UInt64
    public let targets: [Target]
    public let why: String
    public let impact: String
    public let evidence: [String]
    public let actions: [FindingAction]
    public let notes: String?
    public let userNote: String?
    public let ignored: Bool

    public init(id: String, ruleId: String, title: String, category: String, owner: String? = nil, safety: String, bytes: UInt64, alloc: UInt64, targets: [Target] = [], why: String, impact: String, evidence: [String] = [], actions: [FindingAction] = [], notes: String? = nil, userNote: String? = nil, ignored: Bool = false) {
        self.id = id
        self.ruleId = ruleId
        self.title = title
        self.category = category
        self.owner = owner
        self.safety = safety
        self.bytes = bytes
        self.alloc = alloc
        self.targets = targets
        self.why = why
        self.impact = impact
        self.evidence = evidence
        self.actions = actions
        self.notes = notes
        self.userNote = userNote
        self.ignored = ignored
    }

    private enum CodingKeys: String, CodingKey { case id, ruleId, title, category, owner, safety, bytes, alloc, targets, why, impact, evidence, actions, notes, userNote, ignored }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decode(String.self, forKey: .id)
        ruleId = try c.decode(String.self, forKey: .ruleId)
        title = try c.decode(String.self, forKey: .title)
        category = try c.decode(String.self, forKey: .category)
        owner = try c.decodeIfPresent(String.self, forKey: .owner)
        safety = try c.decode(String.self, forKey: .safety)
        bytes = try c.decode(UInt64.self, forKey: .bytes)
        alloc = try c.decode(UInt64.self, forKey: .alloc)
        targets = try decodeArray(Target.self, from: c, forKey: .targets)
        why = try c.decode(String.self, forKey: .why)
        impact = try c.decode(String.self, forKey: .impact)
        evidence = try decodeArray(String.self, from: c, forKey: .evidence)
        actions = try decodeArray(FindingAction.self, from: c, forKey: .actions)
        notes = try c.decodeIfPresent(String.self, forKey: .notes)
        userNote = try c.decodeIfPresent(String.self, forKey: .userNote)
        ignored = try c.decodeIfPresent(Bool.self, forKey: .ignored) ?? false
    }
}
