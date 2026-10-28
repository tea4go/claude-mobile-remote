import UIKit
import CryptoKit
import Capacitor
import Speech
import AVFoundation
import UserNotifications

@objc(SessionExperiencePlugin)
public class SessionExperiencePlugin: CAPPlugin, CAPBridgedPlugin {
    public let identifier = "SessionExperiencePlugin"
    public let jsName = "SessionExperience"
    public let pluginMethods: [CAPPluginMethod] = [
        CAPPluginMethod(name: "copyDebugLog", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "exportDebugLog", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "readSessionCache", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "writeSessionCache", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "removeSessionCache", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "clearSessionCache", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "recognize", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "stopSpeech", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "cancelSpeech", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "syncPushSessions", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "notificationStatus", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "setNotifications", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "notify", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "clearNotifications", returnType: CAPPluginReturnPromise)
    ]
    @objc func copyDebugLog(_ call: CAPPluginCall) {
        guard let text = call.getString("text"), text.utf8.count <= 256 * 1024 else { call.reject("Invalid debug log"); return }
        DispatchQueue.main.async {
            UIPasteboard.general.setItems([[UIPasteboard.typeAutomatic: text]], options: [.localOnly: true, .expirationDate: Date().addingTimeInterval(300)])
            call.resolve()
        }
    }

    @objc func exportDebugLog(_ call: CAPPluginCall) {
        guard let text = call.getString("text"), text.utf8.count <= 256 * 1024 else { call.reject("Invalid debug log"); return }
        DispatchQueue.main.async { [weak self] in
            guard let controller = self?.bridge?.viewController, controller.presentedViewController == nil else { call.reject("Share unavailable"); return }
            let directory = FileManager.default.temporaryDirectory.appendingPathComponent("cmr-debug-" + UUID().uuidString, isDirectory: true)
            do {
                try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
                let file = directory.appendingPathComponent("mobile-debug.txt")
                try Data(text.utf8).write(to: file, options: [.atomic, .completeFileProtection])
                let share = UIActivityViewController(activityItems: [file], applicationActivities: nil)
                share.popoverPresentationController?.sourceView = controller.view
                share.popoverPresentationController?.sourceRect = CGRect(x: controller.view.bounds.midX, y: controller.view.bounds.midY, width: 1, height: 1)
                share.completionWithItemsHandler = { _, completed, _, error in
                    try? FileManager.default.removeItem(at: directory)
                    if error != nil { call.reject("Share failed") } else { call.resolve(["completed": completed]) }
                }
                controller.present(share, animated: true)
            } catch {
                try? FileManager.default.removeItem(at: directory)
                call.reject("Export failed")
            }
        }
    }

    private let cacheQueue = DispatchQueue(label: "cmr.session-cache", qos: .utility)
    private let cacheLimit = 2 * 1024 * 1024

    private func cacheDirectory() throws -> URL {
        let manager = FileManager.default
        var directory = try manager.url(for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true)
            .appendingPathComponent("SessionDisplayCache", isDirectory: true)
        try manager.createDirectory(at: directory, withIntermediateDirectories: true, attributes: [.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication])
        var values = URLResourceValues()
        values.isExcludedFromBackup = true
        try directory.setResourceValues(values)
        return directory
    }

    private func cacheURL(_ call: CAPPluginCall) throws -> URL {
        guard let id = call.getString("sessionId"), !id.isEmpty, id.utf8.count <= 1024 else { throw NSError(domain: "cache", code: 1) }
        let name = SHA256.hash(data: Data(id.utf8)).map { String(format: "%02x", $0) }.joined()
        return try cacheDirectory().appendingPathComponent(name + ".json")
    }

    private func pruneCache(_ directory: URL) throws {
        let manager = FileManager.default
        let urls = try manager.contentsOfDirectory(at: directory, includingPropertiesForKeys: [.contentModificationDateKey, .fileSizeKey], options: .skipsHiddenFiles)
        var retained: [(URL, Date, Int)] = []
        let oldest = Date().addingTimeInterval(-30 * 24 * 60 * 60)
        for url in urls where url.pathExtension == "json" {
            let values = try url.resourceValues(forKeys: [.contentModificationDateKey, .fileSizeKey])
            let date = values.contentModificationDate ?? .distantPast
            let size = values.fileSize ?? 0
            if date < oldest || size > cacheLimit { try manager.removeItem(at: url) }
            else { retained.append((url, date, size)) }
        }
        var total = retained.reduce(0) { $0 + $1.2 }
        for entry in retained.sorted(by: { $0.1 < $1.1 }) where total > 20 * 1024 * 1024 {
            try manager.removeItem(at: entry.0)
            total -= entry.2
        }
    }

    private func cacheTask(_ call: CAPPluginCall, _ operation: @escaping () throws -> JSObject) {
        cacheQueue.async {
            do {
                let result = try operation()
                DispatchQueue.main.async { call.resolve(result) }
            } catch {
                DispatchQueue.main.async { call.reject("本地消息缓存暂不可用") }
            }
        }
    }

    @objc func readSessionCache(_ call: CAPPluginCall) {
        cacheTask(call) {
            let url = try self.cacheURL(call)
            try self.pruneCache(url.deletingLastPathComponent())
            guard FileManager.default.fileExists(atPath: url.path) else { return ["data": NSNull()] }
            let data = try Data(contentsOf: url)
            guard data.count <= self.cacheLimit, let text = String(data: data, encoding: .utf8) else {
                try FileManager.default.removeItem(at: url)
                return ["data": NSNull()]
            }
            return ["data": text]
        }
    }

    @objc func writeSessionCache(_ call: CAPPluginCall) {
        cacheTask(call) {
            let url = try self.cacheURL(call)
            guard let text = call.getString("data"), let data = text.data(using: .utf8), data.count <= self.cacheLimit,
                  let envelope = try JSONSerialization.jsonObject(with: data) as? [String: Any],
                  envelope["sessionId"] as? String == call.getString("sessionId"), envelope["version"] as? Int == 1 else {
                throw NSError(domain: "cache", code: 2)
            }
            try data.write(to: url, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
            try self.pruneCache(url.deletingLastPathComponent())
            return [:]
        }
    }

    @objc func removeSessionCache(_ call: CAPPluginCall) {
        cacheTask(call) {
            let url = try self.cacheURL(call)
            if FileManager.default.fileExists(atPath: url.path) { try FileManager.default.removeItem(at: url) }
            return [:]
        }
    }

    @objc func clearSessionCache(_ call: CAPPluginCall) {
        cacheTask(call) {
            let directory = try self.cacheDirectory()
            try FileManager.default.removeItem(at: directory)
            return [:]
        }
    }

    private let engine = AVAudioEngine()
    private var speechCall: CAPPluginCall?
    private var recognizer: SFSpeechRecognizer?
    private var interruptionObserver: NSObjectProtocol?
    private var task: SFSpeechRecognitionTask?
    private var request: SFSpeechAudioBufferRecognitionRequest?
    private var timeout: DispatchWorkItem?
    private var transcript = ""
    private var tapInstalled = false
    private var backgroundObserver: NSObjectProtocol?
    private let preferenceKey = "cmr.notifications.enabled"

    override public func load() {
        // CapacitorBridge installs its default notification delegate during init.
        // Reclaim delivery only after that init, before exposing our JS listener.
        UNUserNotificationCenter.current().delegate = SessionNotificationRouter.shared
        SessionNotificationRouter.shared.opened = { [weak self] sessionId in
            self?.notifyListeners("notificationOpened", data: ["sessionId": sessionId], retainUntilConsumed: true)
        }
        interruptionObserver = NotificationCenter.default.addObserver(forName: AVAudioSession.interruptionNotification, object: nil, queue: .main) { [weak self] _ in
            guard self?.speechCall != nil else { return }
            self?.failSpeech()
        }
        backgroundObserver = NotificationCenter.default.addObserver(forName: UIApplication.didEnterBackgroundNotification, object: nil, queue: .main) { [weak self] _ in
            self?.finishSpeech(cancelled: true)
        }
    }

    deinit {
        if let backgroundObserver { NotificationCenter.default.removeObserver(backgroundObserver) }
        if let interruptionObserver { NotificationCenter.default.removeObserver(interruptionObserver) }
        timeout?.cancel()
        engine.stop()
        if tapInstalled { engine.inputNode.removeTap(onBus: 0) }
        task?.cancel()
    }

    @objc func recognize(_ call: CAPPluginCall) {
        DispatchQueue.main.async {
            guard self.speechCall == nil else { call.reject("speech_busy"); return }
            self.speechCall = call
            self.transcript = ""
            SFSpeechRecognizer.requestAuthorization { authorization in
                DispatchQueue.main.async {
                    guard self.speechCall === call else { return }
                    guard authorization == .authorized else { self.failSpeech(); return }
                    AVAudioSession.sharedInstance().requestRecordPermission { granted in
                        DispatchQueue.main.async {
                            guard self.speechCall === call else { return }
                            guard granted, UIApplication.shared.applicationState == .active else { self.failSpeech(); return }
                            self.beginSpeech(call)
                        }
                    }
                }
            }
        }
    }

    private func beginSpeech(_ call: CAPPluginCall) {
        guard let recognizer = SFSpeechRecognizer(locale: Locale(identifier: "zh-CN")), recognizer.isAvailable else { failSpeech(); return }
        self.recognizer = recognizer
        do {
            let audio = AVAudioSession.sharedInstance()
            try audio.setCategory(.record, mode: .measurement, options: .duckOthers)
            try audio.setActive(true, options: .notifyOthersOnDeactivation)
            let request = SFSpeechAudioBufferRecognitionRequest()
            request.shouldReportPartialResults = true
            if recognizer.supportsOnDeviceRecognition { request.requiresOnDeviceRecognition = true }
            self.request = request
            let node = engine.inputNode
            let format = node.outputFormat(forBus: 0)
            guard format.sampleRate > 0, format.channelCount > 0 else { failSpeech(); return }
            node.installTap(onBus: 0, bufferSize: 1024, format: format) { buffer, _ in request.append(buffer) }
            tapInstalled = true
            task = recognizer.recognitionTask(with: request) { [weak self] result, error in
                DispatchQueue.main.async {
                    guard let self, self.speechCall === call else { return }
                    if let result { self.transcript = result.bestTranscription.formattedString }
                    if result?.isFinal == true { self.finishSpeech(cancelled: false) }
                    else if error != nil { self.failSpeech() }
                }
            }
            engine.prepare()
            try engine.start()
            let timeout = DispatchWorkItem { [weak self] in self?.finishSpeech(cancelled: false) }
            self.timeout = timeout
            DispatchQueue.main.asyncAfter(deadline: .now() + 60, execute: timeout)
        } catch { failSpeech() }
    }

    private func cleanupSpeech() -> CAPPluginCall? {
        let call = speechCall
        speechCall = nil
        timeout?.cancel()
        timeout = nil
        engine.stop()
        if tapInstalled { engine.inputNode.removeTap(onBus: 0); tapInstalled = false }
        request?.endAudio()
        task?.cancel()
        task = nil
        recognizer = nil
        request = nil
        try? AVAudioSession.sharedInstance().setActive(false, options: .notifyOthersOnDeactivation)
        return call
    }
    private func finishSpeech(cancelled: Bool) {
        let text = cancelled ? "" : transcript
        cleanupSpeech()?.resolve(["text": text])
        transcript = ""
    }
    private func failSpeech() {
        cleanupSpeech()?.reject("speech_unavailable")
        transcript = ""
    }
    @objc func stopSpeech(_ call: CAPPluginCall) {
        DispatchQueue.main.async { self.finishSpeech(cancelled: false); call.resolve() }
    }
    @objc func cancelSpeech(_ call: CAPPluginCall) {
        DispatchQueue.main.async { self.finishSpeech(cancelled: true); call.resolve() }
    }

    @objc func notificationStatus(_ call: CAPPluginCall) {
        UNUserNotificationCenter.current().getNotificationSettings { settings in
            let allowed = settings.authorizationStatus == .authorized || settings.authorizationStatus == .provisional
            DispatchQueue.main.async {
                let push = SessionPushManager.shared
                call.resolve(["enabled": allowed && push.enabled, "remoteConfigured": push.configured, "remoteEnabled": push.remoteEnabled, "remoteReady": allowed && push.ready, "pushError": push.pushError])
            }
        }
    }
    @objc func setNotifications(_ call: CAPPluginCall) {
        guard call.getBool("enabled") == true else {
            UNUserNotificationCenter.current().removeAllPendingNotificationRequests()
            UNUserNotificationCenter.current().removeAllDeliveredNotifications()
            SessionPushManager.shared.setEnabled(false)
            call.resolve(["enabled": false]); return
        }
        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound]) { allowed, _ in
            DispatchQueue.main.async {
                SessionPushManager.shared.setEnabled(allowed)
                call.resolve(["enabled": allowed])
            }
        }
    }
    @objc func syncPushSessions(_ call: CAPPluginCall) {
        DispatchQueue.main.async {
            SessionNotificationRouter.shared.viewedSession = call.getString("viewedSession") ?? ""
            guard let sessions = call.getArray("sessions", JSObject.self),
                  SessionPushManager.shared.syncSessions(sessions) else { call.reject("invalid_push_sessions"); return }
            call.resolve()
        }
    }
    @objc func notify(_ call: CAPPluginCall) {
        if UIApplication.shared.applicationState == .active || SessionPushManager.shared.ready { call.resolve(); return }
        let messages = ["approval": "有工具操作等待审批", "question": "有问题等待回答", "completed": "Claude 已完成本轮任务", "failed": "Claude 执行失败，请打开会话查看", "stopped": "Claude 会话已结束"]
        guard UserDefaults.standard.bool(forKey: preferenceKey),
              let sessionId = call.getString("sessionId"), !sessionId.isEmpty, sessionId.count <= 256,
              let seq = call.getInt("seq"), seq > 0,
              let kind = call.getString("kind"), let body = messages[kind] else { call.resolve(); return }
        let content = UNMutableNotificationContent()
        content.title = "Claude Mobile Remote"
        content.body = body
        content.sound = .default
        content.userInfo = ["sessionId": sessionId]
        // 每个会话只保留最近一条，防止长任务产生无限通知。
        let id = "cmr.session.\(sessionId)"
        let center = UNUserNotificationCenter.current()
        center.removeDeliveredNotifications(withIdentifiers: [id])
        center.add(UNNotificationRequest(identifier: id, content: content, trigger: nil)) { error in
            if error != nil { call.reject("notification_unavailable") } else { call.resolve() }
        }
    }
    @objc func clearNotifications(_ call: CAPPluginCall) {
        guard let sessionId = call.getString("sessionId") else { call.resolve(); return }
        let id = "cmr.session.\(sessionId)"
        UNUserNotificationCenter.current().removePendingNotificationRequests(withIdentifiers: [id])
        let center = UNUserNotificationCenter.current()
        center.removeDeliveredNotifications(withIdentifiers: [id])
        // Capture before the asynchronous lookup: deleting a pairing also removes its routes.
        let topics = SessionPushManager.shared.topics(for: sessionId)
        center.getDeliveredNotifications { notices in
            DispatchQueue.main.async {
                let ids = notices.filter { notice in
                    guard let topic = notice.request.content.userInfo["topic"] as? String else { return false }
                    return topics.contains(topic)
                }.map { $0.request.identifier }
                center.removeDeliveredNotifications(withIdentifiers: ids)
            }
        }
        call.resolve()
    }
}

// AppDelegate 在启动完成前注册，保留原生冷启动早于 WebView/Keychain 恢复的点击。
final class SessionNotificationRouter: NSObject, UNUserNotificationCenterDelegate {
    static let shared = SessionNotificationRouter()
    var viewedSession = ""
    private var pendingSession: String?
    var opened: ((String) -> Void)? {
        didSet {
            if let pendingSession, let opened {
                self.pendingSession = nil
                opened(pendingSession)
            }
        }
    }
    func userNotificationCenter(_ center: UNUserNotificationCenter, willPresent notification: UNNotification, withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void) {
        DispatchQueue.main.async {
            let info = notification.request.content.userInfo
            if notification.request.trigger is UNPushNotificationTrigger {
                SessionPushManager.shared.received(info)
                completionHandler([]) // Foreground state is shown inside the app.
                return
            }
            completionHandler([]) // No system reminders while the app is in the foreground.
        }
    }
    // Shared by the notification delegate and UIScene cold-start delivery.
    func open(_ response: UNNotificationResponse) {
        guard response.actionIdentifier == UNNotificationDefaultActionIdentifier else { return }
        let info = response.notification.request.content.userInfo
        let remote = response.notification.request.trigger is UNPushNotificationTrigger
        let sessionId: String?
        if remote {
            SessionPushManager.shared.received(info)
            sessionId = (info["topic"] as? String).flatMap { SessionPushManager.shared.session(for: $0) }
        } else { sessionId = info["sessionId"] as? String }
        if let sessionId {
            if let opened = opened { opened(sessionId) }
            else { pendingSession = sessionId }
        }
    }
    func userNotificationCenter(_ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse, withCompletionHandler completionHandler: @escaping () -> Void) {
        DispatchQueue.main.async {
            self.open(response)
            completionHandler()
        }
    }
}
