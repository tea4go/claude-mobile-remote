// swift-tools-version: 5.9
import PackageDescription
let package = Package(
    name: "CMRJPushSDK", platforms: [.iOS(.v15)],
    products: [.library(name: "JPushBridge", targets: ["JPushBridge"])],
    targets: [
        .binaryTarget(name: "JPushBinary", path: "Binaries/jpush-ios-6.2.2.xcframework"),
        .binaryTarget(name: "JCoreBinary", path: "Binaries/jcore-noidfa-ios-5.5.1.xcframework"),
        .target(name: "JPushBridge", dependencies: ["JPushBinary", "JCoreBinary"],
                publicHeadersPath: "include", linkerSettings: [
                    .linkedFramework("CFNetwork"), .linkedFramework("CoreFoundation"),
                    .linkedFramework("CoreTelephony"), .linkedFramework("SystemConfiguration"),
                    .linkedFramework("Security"), .linkedFramework("UserNotifications"),
                    .linkedFramework("UIKit"), .linkedLibrary("z"), .linkedLibrary("resolv"),
                    .unsafeFlags(["-ObjC"])
                ])
    ])
