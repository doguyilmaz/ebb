import Foundation

struct EngineLaunchHandle {
    let process: Process
    let stderrURL: URL
    let stderrHandle: FileHandle
}
