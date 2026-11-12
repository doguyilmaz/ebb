import Foundation

public struct DuplicateProgress: Codable, Sendable, Equatable {
    public let phase: String
    public let done: Int
    public let total: Int

    public init(phase: String = "", done: Int = 0, total: Int = 0) {
        self.phase = phase
        self.done = done
        self.total = total
    }
}
