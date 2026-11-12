enum SidebarSection: String, CaseIterable, Identifiable {
    case overview = "Overview"
    case explore = "Explore"
    case findings = "Findings"
    case apps = "Apps"
    case duplicates = "Duplicates"
    case history = "History"
    case activity = "Activity"
    case settings = "Settings"

    var id: Self { self }

    var symbol: String {
        switch self {
        case .overview: "chart.pie"
        case .explore: "folder"
        case .findings: "sparkle.magnifyingglass"
        case .apps: "square.grid.2x2"
        case .duplicates: "doc.on.doc"
        case .history: "clock.arrow.circlepath"
        case .activity: "list.bullet.rectangle"
        case .settings: "gearshape"
        }
    }
}
