import Foundation

public struct ChildrenResponse: Codable, Sendable, Equatable {
    public let path: String
    public let items: [FileItem]
    public let small: [SmallFile]

    public init(path: String, items: [FileItem] = [], small: [SmallFile] = []) {
        self.path = path
        self.items = items
        self.small = small
    }

    private enum CodingKeys: String, CodingKey { case path, items, small }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        path = try c.decode(String.self, forKey: .path)
        items = try decodeArray(FileItem.self, from: c, forKey: .items)
        small = try decodeArray(SmallFile.self, from: c, forKey: .small)
    }
}
