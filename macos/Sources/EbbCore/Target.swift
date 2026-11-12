import Foundation

public struct Target: Codable, Sendable, Equatable, Identifiable {
    public let path: String
    public let bytes: UInt64
    public let alloc: UInt64
    public let command: [String]

    public var id: String { path }

    public init(path: String, bytes: UInt64, alloc: UInt64, command: [String] = []) {
        self.path = path
        self.bytes = bytes
        self.alloc = alloc
        self.command = command
    }

    private enum CodingKeys: String, CodingKey { case path, bytes, alloc, command }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        path = try c.decode(String.self, forKey: .path)
        bytes = try c.decode(UInt64.self, forKey: .bytes)
        alloc = try c.decode(UInt64.self, forKey: .alloc)
        command = try decodeArray(String.self, from: c, forKey: .command)
    }
}
