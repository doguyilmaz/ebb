import Foundation

public struct Health: Codable, Sendable, Equatable {
    public let ok: Bool
    public let version: String
    public let allowed: Bool
    public let device: String?
    public let port: Int?

    public init(ok: Bool, version: String, allowed: Bool, device: String? = nil, port: Int? = nil) {
        self.ok = ok
        self.version = version
        self.allowed = allowed
        self.device = device
        self.port = port
    }
}
