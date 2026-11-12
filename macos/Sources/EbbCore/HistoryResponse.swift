import Foundation

public struct HistoryResponse: Codable, Sendable, Equatable {
    public let snapshots: [Meta]

    public init(snapshots: [Meta] = []) {
        self.snapshots = snapshots
    }

    private enum CodingKeys: String, CodingKey { case snapshots }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        snapshots = try decodeArray(Meta.self, from: c, forKey: .snapshots)
    }
}
