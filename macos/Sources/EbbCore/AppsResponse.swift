import Foundation

public struct AppsResponse: Codable, Sendable, Equatable {
    public let apps: [InstalledApp]
    public let note: String?

    public init(apps: [InstalledApp] = [], note: String? = nil) {
        self.apps = apps
        self.note = note
    }

    private enum CodingKeys: String, CodingKey { case apps, note }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        apps = try decodeArray(InstalledApp.self, from: c, forKey: .apps)
        note = try c.decodeIfPresent(String.self, forKey: .note)
    }
}
