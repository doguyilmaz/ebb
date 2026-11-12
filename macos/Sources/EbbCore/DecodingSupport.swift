import Foundation

func decodeArray<Key: CodingKey, Value: Decodable>(
    _ type: Value.Type,
    from container: KeyedDecodingContainer<Key>,
    forKey key: Key
) throws -> [Value] {
    guard container.contains(key), try !container.decodeNil(forKey: key) else {
        return []
    }
    return try container.decode([Value].self, forKey: key)
}

func decodeOptionalArray<Key: CodingKey, Value: Decodable>(
    _ type: Value.Type,
    from container: KeyedDecodingContainer<Key>,
    forKey key: Key
) throws -> [Value]? {
    guard container.contains(key), try !container.decodeNil(forKey: key) else {
        return nil
    }
    return try container.decode([Value].self, forKey: key)
}
