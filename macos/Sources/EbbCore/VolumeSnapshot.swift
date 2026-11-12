import Foundation

public struct VolumeSnapshot: Codable, Sendable, Equatable, Identifiable {
    public let name: String
    public let date: String

    public var id: String { name }

    public init(name: String, date: String) {
        self.name = name
        self.date = date
    }
}
