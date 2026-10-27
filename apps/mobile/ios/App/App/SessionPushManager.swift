import UIKit
import CryptoKit
import UserNotifications
import JPushBridge

// Notification-only routes. Pair Tokens never leave the secure pairing store.
final class SessionPushManager {
    static let shared = SessionPushManager()
    private let preference = "cmr.notifications.enabled"
    private let consentKey = "cmr.push.consent.v1"
    private let routesKey = "cmr.push.routes.v1"
    private var launchOptions: [UIApplication.LaunchOptionsKey: Any]?
    private var started = false
    private var apnsRegistered = false
    private var syncing = false
    private var appliedRoutes: [String: Date] = [:]
    private var appliedRegistrationID = ""
    private var preferenceRevision = 0
    private var retryAttempts = 0
    private var routes: [String: (session: String, expires: Date)] = [:]
    private var expiryTimer: Timer?
    private var observers: [NSObjectProtocol] = []
    private var apnsError = ""
    private var routeError = ""
    private let registrationURL = URL(string: "https://edo.iwhalecloud.com/verfiles/notify/v1/events")!
    var pushError: String { apnsError.isEmpty ? routeError : apnsError }
    private var appKey: String {
        let value = Bundle.main.object(forInfoDictionaryKey: "CMRJPushAppKey") as? String ?? ""
        return value.range(of: "^[0-9a-fA-F]{24}$", options: .regularExpression) != nil ? value : ""
    }
    var configured: Bool { !appKey.isEmpty }
    var enabled: Bool { UserDefaults.standard.bool(forKey: preference) }
    var remoteEnabled: Bool { enabled && UserDefaults.standard.bool(forKey: consentKey) }
    var ready: Bool { started && remoteEnabled && apnsRegistered && !JPUSHService.registrationID().isEmpty && appliedRegistrationID == JPUSHService.registrationID() && appliedRoutes == desiredRoutes && pushError.isEmpty }
    private var desiredRoutes: [String: Date] { routes.filter { $0.value.expires > Date() }.mapValues { $0.expires } }

    func launch(_ options: [UIApplication.LaunchOptionsKey: Any]?) {
        launchOptions = options
        let saved = UserDefaults.standard.array(forKey: routesKey) as? [[String: Any]] ?? []
        for route in saved {
            if let topic = route["topic"] as? String, validTopic(topic),
               let session = route["session"] as? String, !session.isEmpty, session.count <= 256,
               let expiry = route["expires"] as? Double, Date(timeIntervalSince1970: expiry) > Date() {
                routes[topic] = (session, Date(timeIntervalSince1970: expiry))
            }
        }
        observers.append(NotificationCenter.default.addObserver(forName: .jpfNetworkDidLogin, object: nil, queue: .main) { [weak self] _ in
            self?.syncRegistrations()
        })
        observers.append(NotificationCenter.default.addObserver(forName: UIApplication.didBecomeActiveNotification, object: nil, queue: .main) { [weak self] _ in self?.refresh() })
        refresh()
    }
    func refresh() {
        prune()
        UNUserNotificationCenter.current().getNotificationSettings { settings in
            DispatchQueue.main.async {
                let allowed = settings.authorizationStatus == .authorized || settings.authorizationStatus == .provisional
                if self.remoteEnabled && allowed { self.start() }
                else if self.started { self.stop() }
            }
        }
    }
    func setEnabled(_ value: Bool) {
        preferenceRevision += 1
        retryAttempts = 0
        UserDefaults.standard.set(value, forKey: preference)
        UserDefaults.standard.set(value, forKey: consentKey)
        if value { start() } else { stop() }
    }
    private func start() {
        guard remoteEnabled, configured else { return }
        if !started {
            JPUSHService.setLogOFF()
            let production = (Bundle.main.object(forInfoDictionaryKey: "CMRAPNSEnvironment") as? String) == "production"
            JPUSHService.setup(withOption: launchOptions, appKey: appKey, channel: "cmr", apsForProduction: production, advertisingIdentifier: nil)
            launchOptions = nil
            started = true
        }
        JPUSHService.turnOnPush()
        UIApplication.shared.registerForRemoteNotifications()
        syncRegistrations()
    }
    private func stop() {
        apnsRegistered = false
        let oldRoutes = appliedRoutes
        let oldID = appliedRegistrationID
        appliedRoutes = [:]
        appliedRegistrationID = ""
        apnsError = ""
        routeError = ""
        if !oldID.isEmpty {
            for topic in oldRoutes.keys { sendRegistration(method: "DELETE", topic: topic, registrationID: oldID, expiry: nil) { _ in } }
        }
        if started {
            let revision = preferenceRevision
            JPUSHService.turnOffPush { _ in
                // Only recover a newer off/on action, not a revoked OS permission.
                if self.preferenceRevision != revision && self.remoteEnabled { self.refresh() }
            }
        }
    }
    func registered(_ token: Data) {
        guard remoteEnabled, started else { return }
        JPUSHService.registerDeviceToken(token)
        apnsRegistered = true
        apnsError = ""
        syncRegistrations()
    }
    func registrationFailed() { apnsRegistered = false; apnsError = "apns_registration_failed" }
    func received(_ info: [AnyHashable: Any]) {
        if started { JPUSHService.handleRemoteNotification(info) }
    }
    func session(for topic: String) -> String? {
        guard validTopic(topic), let route = routes[topic], route.expires > Date(), remoteEnabled else { return nil }
        return route.session
    }
    private func validTopic(_ topic: String) -> Bool {
        topic.range(of: "^cmr_[0-9a-f]{32}$", options: .regularExpression) != nil
    }
    static func topic(session: String, token: String) -> String {
        let data = Data(("cmr-push-v1\0" + session + "\0" + token).utf8)
        return "cmr_" + SHA256.hash(data: data).prefix(16).map { String(format: "%02x", $0) }.joined()
    }
    func syncSessions(_ sessions: [[String: Any]]) -> Bool {
        guard sessions.count <= 1000 else { routeError = "too_many_push_sessions"; return false }
        let formatter = ISO8601DateFormatter()
        var updated: [String: (session: String, expires: Date)] = [:]
        for record in sessions {
            guard let session = record["sessionId"] as? String, !session.isEmpty, session.count <= 256,
                  let token = record["pairToken"] as? String, !token.isEmpty, token.count <= 1024,
                  let date = record["expiresAt"] as? String else { return false }
            formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
            var expiry = formatter.date(from: date)
            if expiry == nil { formatter.formatOptions = [.withInternetDateTime]; expiry = formatter.date(from: date) }
            guard let expiry else { return false }
            if expiry > Date() { updated[Self.topic(session: session, token: token)] = (session, expiry) }
        }
        routes = updated
        retryAttempts = 0
        saveRoutes()
        syncRegistrations()
        return true
    }
    private func saveRoutes() {
        UserDefaults.standard.set(routes.map { ["topic": $0.key, "session": $0.value.session, "expires": $0.value.expires.timeIntervalSince1970] }, forKey: routesKey)
        expiryTimer?.invalidate()
        if let next = routes.values.map({ $0.expires }).min() {
            expiryTimer = Timer.scheduledTimer(withTimeInterval: max(1, next.timeIntervalSinceNow), repeats: false) { [weak self] _ in
                self?.prune(); self?.syncRegistrations()
            }
        }
    }
    private func prune() {
        let valid = routes.filter { $0.value.expires > Date() }
        if valid.count != routes.count { routes = valid }
        saveRoutes()
    }
    private func syncRegistrations() {
        let registrationID = JPUSHService.registrationID()
        guard started, remoteEnabled, apnsRegistered, !syncing, !registrationID.isEmpty else { return }
        if appliedRegistrationID != registrationID {
            if !appliedRegistrationID.isEmpty {
                for topic in appliedRoutes.keys { sendRegistration(method: "DELETE", topic: topic, registrationID: appliedRegistrationID, expiry: nil) { _ in } }
            }
            appliedRoutes = [:]
            appliedRegistrationID = registrationID
        }
        let desired = desiredRoutes
        let stale = appliedRoutes.keys.filter { desired[$0] == nil }.sorted()
        let changed = desired.keys.filter { appliedRoutes[$0] != desired[$0] }.sorted()
        guard let topic = stale.first ?? changed.first else { routeError = ""; return }
        let method = stale.contains(topic) ? "DELETE" : "PUT"
        syncing = true
        sendRegistration(method: method, topic: topic, registrationID: registrationID, expiry: desired[topic]) { success in
            self.syncing = false
            guard self.remoteEnabled, self.started, self.apnsRegistered else {
                if success && method == "PUT" { self.sendRegistration(method: "DELETE", topic: topic, registrationID: registrationID, expiry: nil) { _ in } }
                return
            }
            if success {
                if method == "DELETE" { self.appliedRoutes.removeValue(forKey: topic) }
                else if let expiry = desired[topic] { self.appliedRoutes[topic] = expiry }
                self.routeError = ""
                self.retryAttempts = 0
                self.syncRegistrations()
            } else {
                self.routeError = "push_subscription_failed"
                if self.retryAttempts < 3 {
                    self.retryAttempts += 1
                    DispatchQueue.main.asyncAfter(deadline: .now() + 2) { self.syncRegistrations() }
                }
            }
        }
    }

    private func sendRegistration(method: String, topic: String, registrationID: String, expiry: Date?, completion: @escaping (Bool) -> Void) {
        var body: [String: Any] = ["topic": topic, "registrationId": registrationID]
        if let expiry {
            let formatter = ISO8601DateFormatter()
            formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
            body["expiresAt"] = formatter.string(from: expiry)
        }
        guard let data = try? JSONSerialization.data(withJSONObject: body) else { completion(false); return }
        var request = URLRequest(url: registrationURL)
        request.httpMethod = method
        request.httpBody = data
        request.timeoutInterval = 10
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        URLSession.shared.dataTask(with: request) { _, response, error in
            DispatchQueue.main.async { completion(error == nil && (response as? HTTPURLResponse)?.statusCode == 204) }
        }.resume()
    }
}
