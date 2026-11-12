import Foundation

public struct Settings: Codable, Sendable, Equatable {
    public let root: String
    public let threads: Int
    public let staleDays: Int
    public let largeFile: String
    public let protect: [String]
    public let backgroundScans: Bool
    public let lowSpaceGb: Double

    public init(root: String, threads: Int, staleDays: Int, largeFile: String, protect: [String] = [], backgroundScans: Bool, lowSpaceGb: Double) {
        self.root = root
        self.threads = threads
        self.staleDays = staleDays
        self.largeFile = largeFile
        self.protect = protect
        self.backgroundScans = backgroundScans
        self.lowSpaceGb = lowSpaceGb
    }

    private enum CodingKeys: String, CodingKey { case root, threads, staleDays, largeFile, protect, backgroundScans, lowSpaceGb }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        root = try c.decode(String.self, forKey: .root)
        threads = try c.decode(Int.self, forKey: .threads)
        staleDays = try c.decode(Int.self, forKey: .staleDays)
        largeFile = try c.decode(String.self, forKey: .largeFile)
        protect = try decodeArray(String.self, from: c, forKey: .protect)
        backgroundScans = try c.decode(Bool.self, forKey: .backgroundScans)
        lowSpaceGb = try c.decode(Double.self, forKey: .lowSpaceGb)
    }
}
