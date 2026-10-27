import AppKit
import CoreImage
import Vision

// Only public TestFlight links; never use this command for session credentials.
guard CommandLine.arguments.count == 3,
      let input = URL(string: CommandLine.arguments[1]), input.scheme == "https",
      input.host == "testflight.apple.com", input.path.hasPrefix("/join/"),
      let filter = CIFilter(name: "CIQRCodeGenerator") else { fatalError("Invalid public invitation URL") }
let expected = CommandLine.arguments[1]
filter.setValue(Data(expected.utf8), forKey: "inputMessage")
filter.setValue("M", forKey: "inputCorrectionLevel")
let raw = filter.outputImage!
let bordered = raw.extent.insetBy(dx: -4, dy: -4)
let white = CIImage(color: CIColor.white).cropped(to: bordered)
let qr = raw.composited(over: white).transformed(by: CGAffineTransform(scaleX: 12, y: 12))
guard let image = CIContext(options: [.useSoftwareRenderer: true]).createCGImage(qr, from: qr.extent) else {
    throw NSError(domain: "TestFlightQR", code: 1, userInfo: [NSLocalizedDescriptionKey: "QR rendering failed"])
}
let request = VNDetectBarcodesRequest()
request.symbologies = [.qr]
try VNImageRequestHandler(cgImage: image).perform([request])
guard request.results?.first?.payloadStringValue == expected else { fatalError("QR decode verification failed") }
let data = NSBitmapImageRep(cgImage: image).representation(using: .png, properties: [:])!
try data.write(to: URL(fileURLWithPath: CommandLine.arguments[2]), options: .withoutOverwriting)
print("QR generated and decoded successfully")
