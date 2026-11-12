import Foundation

public enum EbbClientError: Error, Sendable, LocalizedError, Equatable {
    case invalidEndpoint
    case invalidToken
    case transport(URLError)
    case http(status: Int, message: String, code: String?)
    case invalidResponse
    case decoding(String)

    public var errorDescription: String? {
        switch self {
        case .invalidEndpoint:
            return "ebb endpoint must be a loopback /api/v1 URL"
        case .invalidToken:
            return "ebb token is empty"
        case let .transport(error):
            return error.localizedDescription
        case let .http(status, message, code):
            if let code, !code.isEmpty {
                return "ebb request failed (\(status), \(code)): \(message)"
            }
            return "ebb request failed (\(status)): \(message)"
        case .invalidResponse:
            return "ebb returned an invalid HTTP response"
        case let .decoding(message):
            return "ebb response could not be decoded: \(message)"
        }
    }
}
