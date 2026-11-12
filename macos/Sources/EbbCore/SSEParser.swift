import Foundation

struct SSEParser {
    private var name = "message"
    private var dataLines: [String] = []

    mutating func consume(_ line: String) -> EbbEvent? {
        if line.isEmpty {
            guard !dataLines.isEmpty || name != "message" else {
                reset()
                return nil
            }
            let event = EbbEvent(name: name, data: Data(dataLines.joined(separator: "\n").utf8))
            reset()
            return event
        }
        if line.first == ":" {
            return nil
        }
        let field: String
        let value: String
        if let separator = line.firstIndex(of: ":") {
            field = String(line[..<separator])
            var start = line.index(after: separator)
            if start < line.endIndex, line[start] == " " {
                start = line.index(after: start)
            }
            value = String(line[start...])
        } else {
            field = line
            value = ""
        }
        switch field {
        case "event":
            name = value
        case "data":
            dataLines.append(value)
        default:
            break
        }
        return nil
    }

    mutating func finish() -> EbbEvent? {
        consume("")
    }

    private mutating func reset() {
        name = "message"
        dataLines.removeAll(keepingCapacity: true)
    }
}
