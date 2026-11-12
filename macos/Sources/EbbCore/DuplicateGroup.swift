import Foundation

public struct DuplicateGroup: Codable, Sendable, Equatable, Identifiable {
    public let size: UInt64
    public let copies: [DuplicateCopy]
    public let wasted: UInt64

    public var id: String { copies.first?.path ?? String(size) }

    public init(size: UInt64, copies: [DuplicateCopy] = [], wasted: UInt64 = 0) {
        self.size = size
        self.copies = copies
        self.wasted = wasted
    }

    private enum CodingKeys: String, CodingKey { case size, copies, wasted }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        size = try c.decode(UInt64.self, forKey: .size)
        copies = try decodeArray(DuplicateCopy.self, from: c, forKey: .copies)
        wasted = try c.decode(UInt64.self, forKey: .wasted)
    }
}
