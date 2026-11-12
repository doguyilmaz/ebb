import Darwin
import Foundation

enum EngineProcessIdentity {
    static func isEbb(pid: Int) -> Bool {
        guard pid > 0, pid <= Int(Int32.max) else { return false }
        var buffer = [UInt8](repeating: 0, count: 4_096)
        let length = buffer.withUnsafeMutableBytes { bytes in
            proc_pidpath(pid_t(pid), bytes.baseAddress, UInt32(bytes.count))
        }
        guard length > 0 else { return false }
        let path = String(decoding: buffer.prefix(Int(length)), as: UTF8.self)
        return URL(fileURLWithPath: path).lastPathComponent == "ebb"
    }

    static func isAlive(pid: Int) -> Bool {
        guard pid > 0, pid <= Int(Int32.max) else { return false }
        // A process owned by another user must not receive this user's token.
        // Match the Go client's refusal when kill(pid, 0) returns EPERM.
        return kill(pid_t(pid), 0) == 0
    }
}
