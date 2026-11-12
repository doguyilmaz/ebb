import Foundation

public struct PendingPlans: Codable, Sendable, Equatable {
    public let plans: [Plan]

    public init(plans: [Plan] = []) {
        self.plans = plans
    }

    private enum CodingKeys: String, CodingKey { case plans }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        plans = try decodeArray(Plan.self, from: c, forKey: .plans)
    }
}
