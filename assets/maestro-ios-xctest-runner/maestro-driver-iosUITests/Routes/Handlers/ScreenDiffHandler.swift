import Foundation
import XCTest
import CryptoKit
import FlyingFox
import os

@MainActor
struct IsScreenStaticHandler: HTTPHandler {
    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: String(describing: Self.self)
    )

    /// Height (points) of the top band left out of the comparison. It covers the
    /// status bar on every iPhone (62 pt with the Dynamic Island) and iPad.
    private static let statusBarInset: CGFloat = 64

    func handleRequest(_ request: FlyingFox.HTTPRequest) async throws -> FlyingFox.HTTPResponse {
        do {
            let screenshot1 = XCUIScreen.main.screenshot()
            let screenshot2 = XCUIScreen.main.screenshot()
            let hash1 = Self.contentDigest(screenshot1)
            let hash2 = Self.contentDigest(screenshot2)

            let isScreenStatic = hash1 == hash2

            let response = ["isScreenStatic" : isScreenStatic]

            let responseData = try JSONSerialization.data(
                withJSONObject: response,
                options: .prettyPrinted
            )
            return HTTPResponse(statusCode: .ok, body: responseData)
        } catch let error {
            return AppError(message: "Detecting screen static request failed. Error \(error.localizedDescription)").httpResponse
        }
    }

    /// Hash of the raw pixels below the status bar. The status bar changes on its
    /// own (clock, Dynamic Island live activities), so on a physical iPhone a
    /// whole-screen hash never matched and every settle wait ran to its timeout.
    /// Hashing the bitmap also skips encoding two PNGs per probe.
    private static func contentDigest(_ screenshot: XCUIScreenshot) -> SHA256.Digest {
        guard let image = screenshot.image.cgImage,
              let data = image.dataProvider?.data as Data? else {
            return SHA256.hash(data: screenshot.pngRepresentation)
        }
        let skippedRows = min(Int((statusBarInset * UIScreen.main.scale).rounded()), image.height)
        let start = min(skippedRows * image.bytesPerRow, data.count)
        return SHA256.hash(data: data[start...])
    }
}
