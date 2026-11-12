// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "Ebb",
    platforms: [.macOS(.v12)],
    products: [
        .library(name: "EbbCore", targets: ["EbbCore"]),
        .library(name: "EbbRuntime", targets: ["EbbRuntime"]),
        .executable(name: "EbbDesktop", targets: ["EbbDesktop"]),
        .executable(name: "EbbNativeSmoke", targets: ["EbbNativeSmoke"]),
    ],
    targets: [
        .target(name: "EbbCore"),
        .target(name: "EbbRuntime", dependencies: ["EbbCore"]),
        .executableTarget(name: "EbbDesktop", dependencies: ["EbbCore", "EbbRuntime"]),
        .executableTarget(name: "EbbNativeSmoke", dependencies: ["EbbCore", "EbbRuntime"]),
        .testTarget(name: "EbbCoreTests", dependencies: ["EbbCore"]),
        .testTarget(name: "EbbRuntimeTests", dependencies: ["EbbCore", "EbbRuntime"]),
    ]
)
