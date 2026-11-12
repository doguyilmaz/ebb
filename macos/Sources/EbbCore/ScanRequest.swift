import Foundation

public struct ScanRequest: Codable, Sendable, Equatable {
    public let root: String?
    public let threads: Int?
    public let gentle: Bool
    public let full: Bool

    public init(root: String? = nil, threads: Int? = nil, gentle: Bool = false, full: Bool = false) {
        self.root = root
        self.threads = threads
        self.gentle = gentle
        self.full = full
    }
}
