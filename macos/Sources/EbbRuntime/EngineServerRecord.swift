import Foundation

struct EngineServerRecord: Decodable, Sendable {
    let pid: Int
    let port: Int
    let version: String
    let daemon: Bool
}

struct EngineHealth: Decodable, Sendable {
    let ok: Bool
    let version: String
    let allowed: Bool
    let port: Int?
}
