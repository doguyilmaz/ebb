import Foundation

public struct StorageBreakdown: Codable, Sendable, Equatable {
    public let capacity: UInt64
    public let used: UInt64
    public let free: UInt64
    public let dataUsed: UInt64
    public let systemUsed: UInt64
    public let others: [VolumeUse]
    public let purgeable: UInt64
    public let snapshots: [VolumeSnapshot]
    public let scanned: UInt64
    public let notVisible: UInt64
    public let fullDiskAccess: Bool

    public init(capacity: UInt64 = 0, used: UInt64 = 0, free: UInt64 = 0, dataUsed: UInt64 = 0, systemUsed: UInt64 = 0, others: [VolumeUse] = [], purgeable: UInt64 = 0, snapshots: [VolumeSnapshot] = [], scanned: UInt64 = 0, notVisible: UInt64 = 0, fullDiskAccess: Bool = false) {
        self.capacity = capacity
        self.used = used
        self.free = free
        self.dataUsed = dataUsed
        self.systemUsed = systemUsed
        self.others = others
        self.purgeable = purgeable
        self.snapshots = snapshots
        self.scanned = scanned
        self.notVisible = notVisible
        self.fullDiskAccess = fullDiskAccess
    }

    private enum CodingKeys: String, CodingKey { case capacity, used, free, dataUsed, systemUsed, others, purgeable, snapshots, scanned, notVisible, fullDiskAccess }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        capacity = try c.decode(UInt64.self, forKey: .capacity)
        used = try c.decode(UInt64.self, forKey: .used)
        free = try c.decode(UInt64.self, forKey: .free)
        dataUsed = try c.decode(UInt64.self, forKey: .dataUsed)
        systemUsed = try c.decode(UInt64.self, forKey: .systemUsed)
        others = try decodeArray(VolumeUse.self, from: c, forKey: .others)
        purgeable = try c.decode(UInt64.self, forKey: .purgeable)
        snapshots = try decodeArray(VolumeSnapshot.self, from: c, forKey: .snapshots)
        scanned = try c.decode(UInt64.self, forKey: .scanned)
        notVisible = try c.decode(UInt64.self, forKey: .notVisible)
        fullDiskAccess = try c.decode(Bool.self, forKey: .fullDiskAccess)
    }
}
