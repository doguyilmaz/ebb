import Foundation

public struct FindingsResponse: Codable, Sendable, Equatable {
    public let findings: [FindingView]
    public let reclaimable: UInt64
    public let notes: [String]

    public init(findings: [FindingView] = [], reclaimable: UInt64 = 0, notes: [String] = []) {
        self.findings = findings
        self.reclaimable = reclaimable
        self.notes = notes
    }

    private enum CodingKeys: String, CodingKey { case findings, reclaimable, notes }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        findings = try decodeArray(FindingView.self, from: c, forKey: .findings)
        reclaimable = try c.decode(UInt64.self, forKey: .reclaimable)
        notes = try decodeArray(String.self, from: c, forKey: .notes)
    }
}
