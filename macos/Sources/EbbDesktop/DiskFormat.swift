import Foundation

@MainActor
enum DiskFormat {
    static func bytes<T: BinaryInteger>(_ count: T) -> String {
        ByteCountFormatter.string(fromByteCount: Int64(clamping: count), countStyle: .decimal)
    }

    static func date(_ value: String) -> String {
        let parser = ISO8601DateFormatter()
        parser.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        var date = parser.date(from: value)
        if date == nil {
            parser.formatOptions = [.withInternetDateTime]
            date = parser.date(from: value)
        }
        guard let date else { return value }
        return DateFormatter.localizedString(from: date, dateStyle: .medium, timeStyle: .short)
    }

    static func query(_ path: String, _ values: [String: String]) -> String {
        var url = URLComponents()
        url.path = path
        url.queryItems = values.sorted { $0.key < $1.key }.map { URLQueryItem(name: $0.key, value: $0.value) }
        // Go treats '+' as a space in a query. A literal plus in a filename must survive.
        url.percentEncodedQuery = url.percentEncodedQuery?.replacingOccurrences(of: "+", with: "%2B")
        return url.string ?? path
    }
}
