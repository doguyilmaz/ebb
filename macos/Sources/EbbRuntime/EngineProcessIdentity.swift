import Darwin
import Foundation

enum EngineProcessIdentity {
    static func isEbb(pid: Int) -> Bool {
        guard pid > 0 else { return false }
        var buffer = [UInt8](repeating: 0, count: 4_096)
        let length = buffer.withUnsafeMutableBytes { bytes in
            proc_pidpath(pid_t(pid), bytes.baseAddress, UInt32(bytes.count))
        }
        guard length > 0 else { return false }
        let path = String(decoding: buffer.prefix(Int(length)), as: UTF8.self)
        return URL(fileURLWithPath: path).lastPathComponent == "ebb"
    }

    static func isAlive(pid: Int) -> Bool {
        guard pid > 0 else { return false }
        if kill(pid_t(pid), 0) == 0 {
            return true
        }
        // EPERM still proves that a process owns the PID. Let the executable
        // identity check reject it instead of launching over a live listener.
        return errno == EPERM
    }
}
