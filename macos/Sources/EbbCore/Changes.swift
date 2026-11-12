import Foundation

public struct Changes: Codable, Sendable, Equatable {
    public let folders: Int
    public let lost: Bool
    public let ready: Bool
    public let lostWhy: String?

    public init(folders: Int, lost: Bool, ready: Bool, lostWhy: String? = nil) {
        self.folders = folders
        self.lost = lost
        self.ready = ready
        self.lostWhy = lostWhy
    }
}
