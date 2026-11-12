import Foundation

public struct DuplicateResult: Codable, Sendable, Equatable {
    public let root: String
    public let time: String
    public let groups: [DuplicateGroup]
    public let wasted: UInt64
    public let hashed: Int
    public let elapsed: Int64

    public init(root: String, time: String, groups: [DuplicateGroup] = [], wasted: UInt64 = 0, hashed: Int = 0, elapsed: Int64 = 0) {
        self.root = root
        self.time = time
        self.groups = groups
        self.wasted = wasted
        self.hashed = hashed
        self.elapsed = elapsed
    }

    private enum CodingKeys: String, CodingKey { case root, time, groups, wasted, hashed, elapsed }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        root = try c.decode(String.self, forKey: .root)
        time = try c.decode(String.self, forKey: .time)
        groups = try decodeArray(DuplicateGroup.self, from: c, forKey: .groups)
        wasted = try c.decode(UInt64.self, forKey: .wasted)
        hashed = try c.decode(Int.self, forKey: .hashed)
        elapsed = try c.decode(Int64.self, forKey: .elapsed)
    }
}
