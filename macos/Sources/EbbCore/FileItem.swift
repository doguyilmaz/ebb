import Foundation

public struct FileItem: Codable, Sendable, Equatable, Identifiable {
    public let path: String
    public let name: String
    public let kind: String
    public let alloc: UInt64
    public let bytes: UInt64
    public let files: Int
    public let modified: Int64
    public let findings: [String]
    public let note: String?

    public var id: String { path }

    public init(path: String, name: String, kind: String, alloc: UInt64, bytes: UInt64, files: Int, modified: Int64, findings: [String] = [], note: String? = nil) {
        self.path = path
        self.name = name
        self.kind = kind
        self.alloc = alloc
        self.bytes = bytes
        self.files = files
        self.modified = modified
        self.findings = findings
        self.note = note
    }

    private enum CodingKeys: String, CodingKey { case path, name, kind, alloc, bytes, files, modified, findings, note }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        path = try c.decode(String.self, forKey: .path)
        name = try c.decode(String.self, forKey: .name)
        kind = try c.decode(String.self, forKey: .kind)
        alloc = try c.decode(UInt64.self, forKey: .alloc)
        bytes = try c.decode(UInt64.self, forKey: .bytes)
        files = try c.decode(Int.self, forKey: .files)
        modified = try c.decode(Int64.self, forKey: .modified)
        findings = try decodeArray(String.self, from: c, forKey: .findings)
        note = try c.decodeIfPresent(String.self, forKey: .note)
    }
}
