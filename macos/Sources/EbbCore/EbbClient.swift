import Foundation

public actor EbbClient {
    private let baseURL: URL
    private let token: String
    private let session: URLSession

    public init(baseURL: URL, token: String, session: URLSession = .shared) throws {
        guard Self.isValidBaseURL(baseURL) else {
            throw EbbClientError.invalidEndpoint
        }
        guard Self.isValidToken(token) else {
            throw EbbClientError.invalidToken
        }
        self.baseURL = Self.normalizedBaseURL(baseURL)
        self.token = token
        self.session = session
    }

    public func get<Response: Decodable & Sendable>(
        _ path: String,
        as type: Response.Type = Response.self
    ) async throws -> Response {
        try await perform(path: path, method: "GET", body: nil, as: type)
    }

    public func post<Response: Decodable & Sendable>(
        _ path: String,
        as type: Response.Type = Response.self
    ) async throws -> Response {
        try await perform(path: path, method: "POST", body: nil, as: type)
    }

    public func post<Body: Encodable & Sendable, Response: Decodable & Sendable>(
        _ path: String,
        body: Body,
        as type: Response.Type = Response.self
    ) async throws -> Response {
        let encoded: Data
        do {
            encoded = try JSONEncoder().encode(body)
        } catch {
            throw EbbClientError.decoding("request could not be encoded: " + error.localizedDescription)
        }
        return try await perform(path: path, method: "POST", body: encoded, as: type)
    }

    public func delete<Response: Decodable & Sendable>(
        _ path: String,
        as type: Response.Type = Response.self
    ) async throws -> Response {
        try await perform(path: path, method: "DELETE", body: nil, as: type)
    }

    /// Opens an authenticated live event stream using the native client's header.
    /// The returned stream owns the HTTP request and cancels it when its consumer
    /// is cancelled or releases the stream.
    public func events() async throws -> AsyncThrowingStream<EbbEvent, Error> {
        var request = try makeRequest(path: "/events", method: "GET", body: nil, timeout: 60)
        request.setValue("text/event-stream", forHTTPHeaderField: "Accept")
        let streamRequest = request
        let session = self.session

        return AsyncThrowingStream { continuation in
            let task = Task {
                do {
                    let (bytes, response) = try await session.bytes(for: streamRequest)
                    guard let http = response as? HTTPURLResponse else {
                        throw EbbClientError.invalidResponse
                    }
                    guard (200..<300).contains(http.statusCode) else {
                        throw Self.httpError(status: http.statusCode, data: nil)
                    }

                    var parser = SSEParser()
                    var lineBytes: [UInt8] = []
                    lineBytes.reserveCapacity(256)
                    for try await byte in bytes {
                        try Task.checkCancellation()
                        if byte == 0x0A {
                            var line = String(decoding: lineBytes, as: UTF8.self)
                            if line.last == "\r" {
                                line.removeLast()
                            }
                            if let event = parser.consume(line) {
                                continuation.yield(event)
                            }
                            lineBytes.removeAll(keepingCapacity: true)
                        } else {
                            lineBytes.append(byte)
                            if lineBytes.count > 1_048_576 {
                                throw EbbClientError.decoding("SSE line exceeds 1 MiB")
                            }
                        }
                    }
                    if !lineBytes.isEmpty {
                        let line = String(decoding: lineBytes, as: UTF8.self)
                        if let event = parser.consume(line) {
                            continuation.yield(event)
                        }
                    }
                    if let event = parser.finish() {
                        continuation.yield(event)
                    }
                    continuation.finish()
                } catch is CancellationError {
                    continuation.finish()
                } catch let error as EbbClientError {
                    continuation.finish(throwing: error)
                } catch let error as URLError {
                    continuation.finish(throwing: EbbClientError.transport(error))
                } catch {
                    continuation.finish(throwing: error)
                }
            }
            continuation.onTermination = { _ in
                task.cancel()
            }
        }
    }

    private func perform<Response: Decodable & Sendable>(
        path: String,
        method: String,
        body: Data?,
        as type: Response.Type
    ) async throws -> Response {
        let timeout: TimeInterval = method == "GET" ? 30 : 660
        let request = try makeRequest(path: path, method: method, body: body, timeout: timeout)
        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await session.data(for: request)
        } catch is CancellationError {
            throw CancellationError()
        } catch let error as URLError {
            throw EbbClientError.transport(error)
        }
        guard let http = response as? HTTPURLResponse else {
            throw EbbClientError.invalidResponse
        }
        guard (200..<300).contains(http.statusCode) else {
            throw Self.httpError(status: http.statusCode, data: data)
        }
        if data.isEmpty {
            if let empty = EmptyResponse() as? Response {
                return empty
            }
            throw EbbClientError.invalidResponse
        }
        do {
            return try JSONDecoder().decode(type, from: data)
        } catch {
            throw EbbClientError.decoding(error.localizedDescription)
        }
    }

    private func makeRequest(
        path: String,
        method: String,
        body: Data?,
        timeout: TimeInterval
    ) throws -> URLRequest {
        guard let url = makeURL(path: path) else {
            throw EbbClientError.invalidEndpoint
        }
        var request = URLRequest(url: url)
        request.httpMethod = method
        request.timeoutInterval = timeout
        request.setValue("Bearer " + token, forHTTPHeaderField: "Authorization")
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        if let body {
            request.httpBody = body
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }
        return request
    }

    private func makeURL(path: String) -> URL? {
        guard !path.contains("://"), !path.hasPrefix("//"), !path.contains("#") else {
            return nil
        }
        guard let relative = URLComponents(string: path), relative.scheme == nil,
              relative.host == nil, relative.user == nil, relative.password == nil,
              relative.fragment == nil else {
            return nil
        }

        let decodedPath = relative.percentEncodedPath.removingPercentEncoding ?? relative.path
        let segments = decodedPath.split(separator: "/", omittingEmptySubsequences: true)
        guard !segments.contains(where: { $0 == "." || $0 == ".." }) else {
            return nil
        }

        var base = URLComponents(url: baseURL, resolvingAgainstBaseURL: false)
        guard base != nil else { return nil }
        let basePath = base!.percentEncodedPath.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        let relativePath = relative.percentEncodedPath.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        let combined = [basePath, relativePath].filter { !$0.isEmpty }.joined(separator: "/")
        base!.percentEncodedPath = "/" + combined

        // Preserve form encoding. Rebuilding queryItems turns %2B into '+' and
        // Go then reads a different filename because it treats '+' as a space.
        base!.percentEncodedQuery = relative.percentEncodedQuery
        base!.fragment = nil
        return base!.url
    }

    private static func isValidBaseURL(_ url: URL) -> Bool {
        guard let components = URLComponents(url: url, resolvingAgainstBaseURL: false),
              components.scheme?.lowercased() == "http",
              let host = components.host?.lowercased(),
              ["127.0.0.1", "localhost", "::1"].contains(host),
              components.user == nil,
              components.password == nil,
              components.query == nil,
              components.fragment == nil else {
            return false
        }
        let path = components.path.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        return path == "api/v1"
    }

    private static func normalizedBaseURL(_ url: URL) -> URL {
        var components = URLComponents(url: url, resolvingAgainstBaseURL: false)!
        components.path = "/api/v1"
        components.query = nil
        components.fragment = nil
        return components.url!
    }

    private static func isValidToken(_ token: String) -> Bool {
        !token.isEmpty && token.rangeOfCharacter(from: .whitespacesAndNewlines) == nil &&
            !token.unicodeScalars.contains(where: { CharacterSet.controlCharacters.contains($0) })
    }

    private static func httpError(status: Int, data: Data?) -> EbbClientError {
        if let data, let payload = try? JSONDecoder().decode(APIErrorPayload.self, from: data), !payload.error.isEmpty {
            return .http(status: status, message: payload.error, code: payload.code)
        }
        return .http(status: status, message: HTTPURLResponse.localizedString(forStatusCode: status), code: nil)
    }
}
