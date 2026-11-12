import Foundation

struct APIErrorPayload: Decodable, Sendable {
    let error: String
    let code: String?
}
