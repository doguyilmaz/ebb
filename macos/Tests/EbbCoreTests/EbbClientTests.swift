import Foundation
import XCTest
@testable import EbbCore

final class EbbClientTests: XCTestCase {
    override func tearDown() {
        URLProtocolStub.reset()
        super.tearDown()
    }

    func testRejectsNonLoopbackEndpointAndBlankToken() throws {
        XCTAssertThrowsError(try EbbClient(baseURL: URL(string: "https://example.com/api/v1")!, token: "token")) { error in
            XCTAssertEqual(error as? EbbClientError, .invalidEndpoint)
        }
        XCTAssertThrowsError(try EbbClient(baseURL: URL(string: "http://127.0.0.1:47340/api/v1")!, token: "")) { error in
            XCTAssertEqual(error as? EbbClientError, .invalidToken)
        }
        XCTAssertThrowsError(try EbbClient(baseURL: URL(string: "http://127.0.0.1:47340/api/v2")!, token: "token")) { error in
            XCTAssertEqual(error as? EbbClientError, .invalidEndpoint)
        }
    }

    func testGetUsesBearerAuthAndDecodesResponse() async throws {
        let client = try makeClient { request in
            XCTAssertEqual(request.httpMethod, "GET")
            XCTAssertEqual(request.value(forHTTPHeaderField: "Authorization"), "Bearer test-token-123")
            XCTAssertEqual(request.timeoutInterval, 30, accuracy: 0.001)
            let body = #"{"ok":true,"version":"1.2.3","allowed":true,"port":47340}"#.data(using: .utf8)!
            return Self.response(for: request, status: 200, body: body)
        }

        let health: Health = try await client.get("/health")
        XCTAssertTrue(health.ok)
        XCTAssertEqual(health.version, "1.2.3")
        XCTAssertEqual(health.port, 47_340)
    }

    func testPathAndQueryAreEscapedWithoutChangingFilename() async throws {
        let client = try makeClient { request in
            let components = URLComponents(url: try XCTUnwrap(request.url), resolvingAgainstBaseURL: false)
            let query = try XCTUnwrap(components?.queryItems)
            XCTAssertEqual(query.first(where: { $0.name == "path" })?.value, "/Users/me/a b?c+#")
            XCTAssertEqual(query.first(where: { $0.name == "limit" })?.value, "10")
            XCTAssertEqual(query.first(where: { $0.name == "includeSmall" })?.value, "true")
            let body = #"{"path":"/Users/me","items":null,"small":null}"#.data(using: .utf8)!
            return Self.response(for: request, status: 200, body: body)
        }

        var components = URLComponents()
        components.path = "/children"
        components.queryItems = [
            URLQueryItem(name: "path", value: "/Users/me/a b?c+#"),
            URLQueryItem(name: "limit", value: "10"),
            URLQueryItem(name: "includeSmall", value: "true")
        ]
        let response: ChildrenResponse = try await client.get(try XCTUnwrap(components.string))
        XCTAssertEqual(response.path, "/Users/me")
        XCTAssertTrue(response.items.isEmpty)
        XCTAssertTrue(response.small.isEmpty)
    }

    func testLiteralPlusSurvivesGoQueryDecoding() async throws {
        let client = try makeClient { request in
            let components = URLComponents(url: try XCTUnwrap(request.url), resolvingAgainstBaseURL: false)
            let encoded = try XCTUnwrap(components?.percentEncodedQuery)
            let formValue = encoded.replacingOccurrences(of: "+", with: " ").removingPercentEncoding
            XCTAssertEqual(formValue, "path=/tmp/A+B & notes")
            return Self.response(for: request, status: 200, body: Data(#"{"path":"/tmp","items":[]}"#.utf8))
        }
        let _: ChildrenResponse = try await client.get("/children?path=%2Ftmp%2FA%2BB%20%26%20notes")
    }

    func testNullArraysNormalizeToEmptyAndEmptyResponseAccepts204() async throws {
        let client = try makeClient { request in
            if request.url?.path.hasSuffix("/children") == true {
                let body = #"{"path":"/tmp","items":null,"small":null}"#.data(using: .utf8)!
                return Self.response(for: request, status: 200, body: body)
            }
            return Self.response(for: request, status: 204, body: Data())
        }

        let children: ChildrenResponse = try await client.get("/children")
        XCTAssertEqual(children.items, [])
        XCTAssertEqual(children.small, [])

        let _: EmptyResponse = try await client.post("/ops/op-1/undo")
    }

    func testHTTPErrorPreservesStatusCodeAndAPIMessage() async throws {
        let client = try makeClient { request in
            let body = #"{"error":"scan already running","code":"scan-running"}"#.data(using: .utf8)!
            return Self.response(for: request, status: 409, body: body)
        }

        do {
            let _: Health = try await client.get("/status")
            XCTFail("request should fail")
        } catch let error as EbbClientError {
            XCTAssertEqual(error, .http(status: 409, message: "scan already running", code: "scan-running"))
            XCTAssertTrue(error.localizedDescription.contains("scan already running"))
            XCTAssertTrue(error.localizedDescription.contains("scan-running"))
        }
    }

    func testPostEncodesRequestAndUsesLongMutationTimeout() async throws {
        let client = try makeClient { request in
            XCTAssertEqual(request.httpMethod, "POST")
            XCTAssertEqual(request.timeoutInterval, 660, accuracy: 0.001)
            XCTAssertEqual(request.value(forHTTPHeaderField: "Content-Type"), "application/json")
            let body = try XCTUnwrap(Self.requestBody(request))
            let json = try XCTUnwrap(JSONSerialization.jsonObject(with: body) as? [String: Any])
            XCTAssertEqual(json["source"] as? String, "ui")
            XCTAssertNil(json["findingIds"])
            let response = #"{"id":"plan-1","created":"2026-11-12T10:00:00Z","expires":"2026-11-12T10:10:00Z","source":"ui","items":null,"bytes":0,"needsApproval":false,"approved":false}"#.data(using: .utf8)!
            return Self.response(for: request, status: 200, body: response)
        }

        let plan: Plan = try await client.post("/plans", body: PlanRequest(source: "ui"))
        XCTAssertEqual(plan.id, "plan-1")
        XCTAssertTrue(plan.items.isEmpty)
    }

    func testEventsUseHeaderAuthAndParseMultilineSSE() async throws {
        let client = try makeClient { request in
            XCTAssertEqual(request.httpMethod, "GET")
            XCTAssertEqual(request.value(forHTTPHeaderField: "Authorization"), "Bearer test-token-123")
            XCTAssertEqual(request.value(forHTTPHeaderField: "Accept"), "text/event-stream")
            XCTAssertNil(request.url?.query)
            let sse = "event: hello\ndata: {\"ok\":true}\n\n: keep-alive\nevent: scan.progress\ndata: {\"dirs\":1,\ndata: \"files\":2,\ndata: \"bytes\":0,\ndata: \"problems\":0,\ndata: \"path\":\"\",\ndata: \"elapsedMs\":0}\n\n"
            return Self.response(for: request, status: 200, body: Data(sse.utf8), contentType: "text/event-stream")
        }

        let stream = try await client.events()
        var events: [EbbEvent] = []
        for try await event in stream {
            events.append(event)
        }
        XCTAssertEqual(events.map(\.name), ["hello", "scan.progress"])
        XCTAssertEqual(events.count, 2)
        guard events.count == 2 else { return }
        XCTAssertEqual(String(decoding: events[0].data, as: UTF8.self), #"{"ok":true}"#)
        let progress = try events[1].decode(Progress.self)
        XCTAssertEqual(progress.dirs, 1)
        XCTAssertEqual(progress.files, 2)
    }

    private func makeClient(handler: @escaping (URLRequest) throws -> (HTTPURLResponse, Data)) throws -> EbbClient {
        URLProtocolStub.handler = handler
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [URLProtocolStub.self]
        let session = URLSession(configuration: configuration)
        return try EbbClient(
            baseURL: URL(string: "http://127.0.0.1:47340/api/v1/")!,
            token: "test-token-123",
            session: session
        )
    }

    private static func response(for request: URLRequest, status: Int, body: Data, contentType: String = "application/json") -> (HTTPURLResponse, Data) {
        let response = HTTPURLResponse(
            url: request.url!,
            statusCode: status,
            httpVersion: nil,
            headerFields: ["Content-Type": contentType]
        )!
        return (response, body)
    }

    private static func requestBody(_ request: URLRequest) -> Data? {
        if let body = request.httpBody {
            return body
        }
        guard let stream = request.httpBodyStream else { return nil }
        stream.open()
        defer { stream.close() }
        var data = Data()
        var buffer = [UInt8](repeating: 0, count: 4_096)
        while stream.hasBytesAvailable {
            let count = stream.read(&buffer, maxLength: buffer.count)
            if count <= 0 { break }
            data.append(buffer, count: count)
        }
        return data
    }
}
