import Foundation

public struct PlanResult: Codable, Sendable, Equatable {
    public let planId: String
    public let freed: UInt64
    public let done: Int
    public let refused: Int
    public let failed: Int
    public let manual: Int
    public let ops: [Operation]

    public init(planId: String, freed: UInt64 = 0, done: Int = 0, refused: Int = 0, failed: Int = 0, manual: Int = 0, ops: [Operation] = []) {
        self.planId = planId
        self.freed = freed
        self.done = done
        self.refused = refused
        self.failed = failed
        self.manual = manual
        self.ops = ops
    }

    private enum CodingKeys: String, CodingKey { case planId, freed, done, refused, failed, manual, ops }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        planId = try c.decode(String.self, forKey: .planId)
        freed = try c.decode(UInt64.self, forKey: .freed)
        done = try c.decode(Int.self, forKey: .done)
        refused = try c.decode(Int.self, forKey: .refused)
        failed = try c.decode(Int.self, forKey: .failed)
        manual = try c.decode(Int.self, forKey: .manual)
        ops = try decodeArray(Operation.self, from: c, forKey: .ops)
    }
}
