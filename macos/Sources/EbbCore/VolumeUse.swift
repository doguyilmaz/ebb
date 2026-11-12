import Foundation

public struct VolumeUse: Codable, Sendable, Equatable, Identifiable {
    public let name: String
    public let role: String
    public let device: String
    public let used: UInt64

    public var id: String { device.isEmpty ? name : device }

    public init(name: String, role: String, device: String, used: UInt64) {
        self.name = name
        self.role = role
        self.device = device
        self.used = used
    }
}
