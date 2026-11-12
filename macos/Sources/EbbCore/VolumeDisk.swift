import Foundation

public struct VolumeDisk: Codable, Sendable, Equatable, Identifiable {
    public let name: String
    public let path: String
    public let capacity: UInt64
    public let free: UInt64
    public let boot: Bool
    public let readOnly: Bool
    public let network: Bool
    public let fs: String

    public var id: String { path }

    public init(name: String, path: String, capacity: UInt64 = 0, free: UInt64 = 0, boot: Bool = false, readOnly: Bool = false, network: Bool = false, fs: String = "") {
        self.name = name
        self.path = path
        self.capacity = capacity
        self.free = free
        self.boot = boot
        self.readOnly = readOnly
        self.network = network
        self.fs = fs
    }
}
