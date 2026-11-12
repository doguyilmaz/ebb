import Foundation

public struct OpsResponse: Codable, Sendable, Equatable {
    public let ops: [Operation]

    public init(ops: [Operation] = []) {
        self.ops = ops
    }

    private enum CodingKeys: String, CodingKey { case ops }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        ops = try decodeArray(Operation.self, from: c, forKey: .ops)
    }
}
