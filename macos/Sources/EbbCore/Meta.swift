import Foundation

public struct Meta: Codable, Sendable, Equatable, Identifiable {
    public let id: String
    public let time: String
    public let root: String
    public let used: UInt64
    public let free: UInt64
    public let scanned: UInt64
    public let elapsed: Int64
    public let dirs: UInt64
    public let files: UInt64
    public let eventId: UInt64?
    public let fseventsUuid: String?
    public let incremental: Bool?
    public let changed: UInt64?

    public init(id: String, time: String, root: String, used: UInt64 = 0, free: UInt64 = 0, scanned: UInt64 = 0, elapsed: Int64 = 0, dirs: UInt64 = 0, files: UInt64 = 0, eventId: UInt64? = nil, fseventsUuid: String? = nil, incremental: Bool? = nil, changed: UInt64? = nil) {
        self.id = id
        self.time = time
        self.root = root
        self.used = used
        self.free = free
        self.scanned = scanned
        self.elapsed = elapsed
        self.dirs = dirs
        self.files = files
        self.eventId = eventId
        self.fseventsUuid = fseventsUuid
        self.incremental = incremental
        self.changed = changed
    }
}
