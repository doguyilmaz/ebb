import Foundation

public struct EbbEvent: Sendable, Equatable {
    public let name: String
    public let data: Data

    public init(name: String, data: Data) {
        self.name = name
        self.data = data
    }

    public func decode<Value: Decodable & Sendable>(_ type: Value.Type = Value.self) throws -> Value {
        do {
            return try JSONDecoder().decode(type, from: data)
        } catch {
            throw EbbClientError.decoding(error.localizedDescription)
        }
    }
}
