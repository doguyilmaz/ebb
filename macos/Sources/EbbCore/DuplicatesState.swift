import Foundation

public struct DuplicatesState: Codable, Sendable, Equatable {
    public let running: Bool
    public let stopped: Bool
    public let progress: DuplicateProgress?
    public let result: DuplicateResult?
    public let error: String?

    public init(running: Bool = false, stopped: Bool = false, progress: DuplicateProgress? = nil, result: DuplicateResult? = nil, error: String? = nil) {
        self.running = running
        self.stopped = stopped
        self.progress = progress
        self.result = result
        self.error = error
    }
}
