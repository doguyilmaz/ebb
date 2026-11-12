import Foundation

public struct PlanRequest: Codable, Sendable, Equatable {
    public let findingIds: [String]
    public let paths: [String]
    public let preset: String?
    public let app: String?
    public let source: String?

    public init(findingIds: [String] = [], paths: [String] = [], preset: String? = nil, app: String? = nil, source: String? = nil) {
        self.findingIds = findingIds
        self.paths = paths
        self.preset = preset
        self.app = app
        self.source = source
    }

    private enum CodingKeys: String, CodingKey { case findingIds, paths, preset, app, source }

    public func encode(to encoder: Encoder) throws {
        var c = encoder.container(keyedBy: CodingKeys.self)
        if !findingIds.isEmpty { try c.encode(findingIds, forKey: .findingIds) }
        if !paths.isEmpty { try c.encode(paths, forKey: .paths) }
        try c.encodeIfPresent(preset, forKey: .preset)
        try c.encodeIfPresent(app, forKey: .app)
        try c.encodeIfPresent(source, forKey: .source)
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        findingIds = try decodeArray(String.self, from: c, forKey: .findingIds)
        paths = try decodeArray(String.self, from: c, forKey: .paths)
        preset = try c.decodeIfPresent(String.self, forKey: .preset)
        app = try c.decodeIfPresent(String.self, forKey: .app)
        source = try c.decodeIfPresent(String.self, forKey: .source)
    }
}
