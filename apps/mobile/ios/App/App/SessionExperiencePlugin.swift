import Capacitor
import JPushBridge
import CryptoKit
import Speech
import AVFoundation
import UserNotifications

@objc(SessionExperiencePlugin)
public class SessionExperiencePlugin: CAPPlugin, CAPBridgedPlugin {
    public let identifier = "SessionExperiencePlugin"
    public let jsName = "SessionExperience"
    public let pluginMethods: [CAPPluginMethod] = [
        CAPPluginMethod(name: "recognize", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "stopSpeech", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "cancelSpeech", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "syncPushSessions", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "notificationStatus", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "setNotifications", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "notify", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "clearNotifications", returnType: CAPPluginReturnPromise)
    ]
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
            call.resolve(["enabled": allowed && UserDefaults.standard.bool(forKey: self.preferenceKey), "remoteConfigured": SessionPush.shared.configured, "remoteReady": SessionPush.shared.ready, "pushError": SessionPush.shared.lastError])
        }
    }
    @objc func setNotifications(_ call: CAPPluginCall) {
        guard call.getBool("enabled") == true else {
            UNUserNotificationCenter.current().removeAllPendingNotificationRequests()
            UNUserNotificationCenter.current().removeAllDeliveredNotifications()
            DispatchQueue.main.async { SessionPush.shared.disable { code in
                if code == 0 {
                    UserDefaults.standard.set(false, forKey: self.preferenceKey)
                    call.resolve(["enabled": false])
                } else { call.reject("push_disable_failed") }
            } }; return
        }
        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound]) { allowed, _ in
            UserDefaults.standard.set(allowed, forKey: self.preferenceKey)
            DispatchQueue.main.async {
                if allowed { SessionPush.shared.enable() }
                call.resolve(["enabled": allowed, "remoteConfigured": SessionPush.shared.configured, "remoteReady": SessionPush.shared.ready])
            }
        }
    }
    @objc func syncPushSessions(_ call: CAPPluginCall) {
        let sessions = call.getArray("sessions", JSObject.self) ?? []
        guard sessions.count <= 100 else { call.reject("push_session_limit"); return }
        var topics: [String] = []
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        let plain = ISO8601DateFormatter()
        for session in sessions {
            guard let id = session["sessionId"] as? String, !id.isEmpty,
                  let token = session["pairToken"] as? String, !token.isEmpty,
                  let expires = session["expiresAt"] as? String,
                  let date = formatter.date(from: expires) ?? plain.date(from: expires), date > Date() else { continue }
            let digest = SHA256.hash(data: Data(("cmr-push-v1\0" + id + "\0" + token).utf8))
            topics.append("cmr_" + digest.prefix(16).map { String(format: "%02x", $0) }.joined())
        }
        let viewed = call.getString("viewedSession") ?? ""
        DispatchQueue.main.async {
            SessionPush.shared.setTopics(topics, viewed: viewed)
            call.resolve()
        }
    }
    @objc func notify(_ call: CAPPluginCall) {
        // APNs is the notification source when configured; no duplicate local alert.
        if SessionPush.shared.configured { call.resolve(); return }
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
        UNUserNotificationCenter.current().removeDeliveredNotifications(withIdentifiers: [id])
        call.resolve()
    }
}

// AppDelegate 在启动完成前注册，保留原生冷启动早于 WebView/Keychain 恢复的点击。
final class SessionNotificationRouter: NSObject, UNUserNotificationCenterDelegate {
    static let shared = SessionNotificationRouter()
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
        let info = notification.request.content.userInfo
        if notification.request.trigger is UNPushNotificationTrigger { JPUSHService.handleRemoteNotification(info) }
        let viewed = info["sessionId"] as? String == SessionPush.shared.viewedSession
        completionHandler(UserDefaults.standard.bool(forKey: "cmr.notifications.enabled") && !viewed ? [.banner, .sound] : [])
    }
    func userNotificationCenter(_ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse, withCompletionHandler completionHandler: @escaping () -> Void) {
        if response.notification.request.trigger is UNPushNotificationTrigger { JPUSHService.handleRemoteNotification(response.notification.request.content.userInfo) }
        if let sessionId = response.notification.request.content.userInfo["sessionId"] as? String {
            DispatchQueue.main.async {
                if let opened = self.opened { opened(sessionId) }
                else { self.pendingSession = sessionId }
            }
        }
        completionHandler()
    }
}

// Tags are notification-only SHA-256 capabilities. Raw Pair Tokens never leave
// this call or enter UserDefaults, logs or JPush. No advertising SDK is linked.
final class SessionPush {
    static let shared = SessionPush()
    private var started = false
    private var launchOptions: [UIApplication.LaunchOptionsKey: Any]?
    private var topics: [String] = []
    private var generation = 0
    private(set) var ready = false
    private var tagsReady = false
    private var apnsReady = false
    private(set) var lastError = ""
    var viewedSession = ""
    var configured: Bool {
        let key = Bundle.main.object(forInfoDictionaryKey: "CMRJPushAppKey") as? String ?? ""
        return key.count == 24 && key.allSatisfy { $0.isHexDigit }
    }
    func launch(_ options: [UIApplication.LaunchOptionsKey: Any]?) {
        launchOptions = options
        if UserDefaults.standard.bool(forKey: "cmr.notifications.enabled") { enable() }
    }
    func enable() {
        guard configured else { lastError = "missing_app_key"; return }
        if !started {
            JPUSHService.setLogOFF()
            #if DEBUG
            let production = false
            #else
            let production = true
            #endif
            let key = Bundle.main.object(forInfoDictionaryKey: "CMRJPushAppKey") as! String
            JPUSHService.setup(withOption: launchOptions, appKey: key, channel: "AppStore", apsForProduction: production, advertisingIdentifier: nil)
            started = true
        }
        JPUSHService.turnOnPush()
        UIApplication.shared.registerForRemoteNotifications()
        updateTags()
    }
    func registeredAPNs() {
        apnsReady = true
        ready = tagsReady
        if lastError == "apns_registration_failed" { lastError = "" }
    }
    func failedAPNs() {
        apnsReady = false
        ready = false
        lastError = "apns_registration_failed"
    }
    func disable(_ completion: @escaping (Int) -> Void) {
        ready = false
        generation += 1
        guard started else { completion(0); return }
        JPUSHService.turnOffPush { code in completion(Int(code)) }
    }
    func setTopics(_ topics: [String], viewed: String) {
        viewedSession = viewed
        guard self.topics != topics else { return }
        self.topics = topics
        if started && UserDefaults.standard.bool(forKey: "cmr.notifications.enabled") { updateTags() }
    }
    private func updateTags() {
        generation += 1
        let current = generation
        ready = false
        JPUSHService.registrationIDCompletionHandler { [weak self] code, _ in
            DispatchQueue.main.async {
                guard let self, current == self.generation else { return }
                guard code == 0 else { self.lastError = "registration_\(code)"; return }
                let completion: (Int, Set<AnyHashable>?, Int) -> Void = { code, _, _ in
                    DispatchQueue.main.async {
                        guard current == self.generation else { return }
                        self.tagsReady = code == 0
                        self.ready = self.tagsReady && self.apnsReady
                        self.lastError = code == 0 ? "" : "tags_\(code)"
                    }
                }
                if self.topics.isEmpty { JPUSHService.cleanTags(completion, seq: current) }
                else { JPUSHService.setTags(Set(self.topics), completion: completion, seq: current) }
            }
        }
    }
}
