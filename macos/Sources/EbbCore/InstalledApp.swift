import Foundation

public struct InstalledApp: Codable, Sendable, Equatable, Identifiable {
    public let name: String
    public let bundleId: String
    public let version: String?
    public let path: String
    public let bytes: UInt64
    public let alloc: UInt64
    public let apple: Bool
    public let rootOwned: Bool
    public let executable: String
    public let leftovers: [Target]
    public let leftoverBytes: UInt64

    public var id: String { bundleId.isEmpty ? path : bundleId }

    public init(name: String, bundleId: String, version: String? = nil, path: String, bytes: UInt64, alloc: UInt64, apple: Bool, rootOwned: Bool, executable: String, leftovers: [Target] = [], leftoverBytes: UInt64 = 0) {
        self.name = name
        self.bundleId = bundleId
        self.version = version
        self.path = path
        self.bytes = bytes
        self.alloc = alloc
        self.apple = apple
        self.rootOwned = rootOwned
        self.executable = executable
        self.leftovers = leftovers
        self.leftoverBytes = leftoverBytes
    }

    private enum CodingKeys: String, CodingKey { case name, bundleId, version, path, bytes, alloc, apple, rootOwned, executable, leftovers, leftoverBytes }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        name = try c.decode(String.self, forKey: .name)
        bundleId = try c.decode(String.self, forKey: .bundleId)
        version = try c.decodeIfPresent(String.self, forKey: .version)
        path = try c.decode(String.self, forKey: .path)
        bytes = try c.decode(UInt64.self, forKey: .bytes)
        alloc = try c.decode(UInt64.self, forKey: .alloc)
        apple = try c.decode(Bool.self, forKey: .apple)
        rootOwned = try c.decode(Bool.self, forKey: .rootOwned)
        executable = try c.decode(String.self, forKey: .executable)
        leftovers = try decodeArray(Target.self, from: c, forKey: .leftovers)
        leftoverBytes = try c.decode(UInt64.self, forKey: .leftoverBytes)
    }
}
