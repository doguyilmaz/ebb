import Foundation

public struct ScanState: Codable, Sendable, Equatable {
    public let running: Bool
    public let root: String?
    public let progress: Progress?
    public let error: String?

    public init(running: Bool, root: String? = nil, progress: Progress? = nil, error: String? = nil) {
        self.running = running
        self.root = root
        self.progress = progress
        self.error = error
    }
}
