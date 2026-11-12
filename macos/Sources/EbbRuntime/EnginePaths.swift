import Foundation

struct EnginePaths {
    let stateDirectory: URL
    let serverRecord: URL
    let token: URL
    let port: Int

    static func resolve(environment: [String: String], fileManager: FileManager = .default) throws -> EnginePaths {
        let sandbox = environment["EBB_SANDBOX"] == "1"
        let overrideHome = environment["EBB_HOME"]
        let home: URL

        if sandbox {
            guard let overrideHome, !overrideHome.isEmpty else {
                throw EngineLauncherError.invalidSandboxHome
            }
            let candidate = URL(fileURLWithPath: overrideHome).standardizedFileURL.resolvingSymlinksInPath()
            let realHome = fileManager.homeDirectoryForCurrentUser.standardizedFileURL.resolvingSymlinksInPath()
            var isDirectory: ObjCBool = false
            guard fileManager.fileExists(atPath: candidate.path, isDirectory: &isDirectory),
                  isDirectory.boolValue,
                  candidate.path != "/",
                  candidate.path != realHome.path,
                  !realHome.path.hasPrefix(candidate.path + "/"),
                  !candidate.path.hasPrefix(realHome.path + "/") else {
                throw EngineLauncherError.invalidSandboxHome
            }
            home = candidate
        } else {
            guard overrideHome == nil || overrideHome?.isEmpty == true else {
                throw EngineLauncherError.invalidSandboxHome
            }
            home = fileManager.homeDirectoryForCurrentUser
        }

        let port: Int
        if sandbox {
            guard let rawPort = environment["EBB_PORT"], let parsed = Int(rawPort), (1...65_535).contains(parsed) else {
                throw EngineLauncherError.invalidSandboxPort
            }
            port = parsed
        } else {
            port = 47_340
        }

        let stateDirectory = home.appendingPathComponent(".local/state/ebb", isDirectory: true)
        return EnginePaths(
            stateDirectory: stateDirectory,
            serverRecord: stateDirectory.appendingPathComponent("server.json"),
            token: stateDirectory.appendingPathComponent("ui-token"),
            port: port
        )
    }

    static func helperURL(bundleURL: URL, fileManager: FileManager = .default) throws -> URL {
        let helper = bundleURL
            .appendingPathComponent("Contents/Helpers/ebb", isDirectory: false)
            .standardizedFileURL
        guard fileManager.isExecutableFile(atPath: helper.path) else {
            throw EngineLauncherError.helperNotFound
        }
        return helper
    }
}
