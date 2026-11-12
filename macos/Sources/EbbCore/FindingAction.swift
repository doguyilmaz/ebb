import Foundation

public struct FindingAction: Codable, Sendable, Equatable {
    public let kind: String
    public let argv: [String]
    public let requires: String?
    public let fallback: String?
    public let hint: String?
    public let quit: [String]
    public let unload: Bool

    public init(kind: String, argv: [String] = [], requires: String? = nil, fallback: String? = nil, hint: String? = nil, quit: [String] = [], unload: Bool = false) {
        self.kind = kind
        self.argv = argv
        self.requires = requires
        self.fallback = fallback
        self.hint = hint
        self.quit = quit
        self.unload = unload
    }

    private enum CodingKeys: String, CodingKey { case kind, argv, requires, fallback, hint, quit, unload }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        kind = try c.decode(String.self, forKey: .kind)
        argv = try decodeArray(String.self, from: c, forKey: .argv)
        requires = try c.decodeIfPresent(String.self, forKey: .requires)
        fallback = try c.decodeIfPresent(String.self, forKey: .fallback)
        hint = try c.decodeIfPresent(String.self, forKey: .hint)
        quit = try decodeArray(String.self, from: c, forKey: .quit)
        unload = try c.decodeIfPresent(Bool.self, forKey: .unload) ?? false
    }
}
