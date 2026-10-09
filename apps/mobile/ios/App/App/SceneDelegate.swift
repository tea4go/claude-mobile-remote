import UIKit
import Capacitor

// Capacitor 使用 WebView 自身作为根视图。用全屏容器承载它，才能让
// 系统键盘布局指南驱动 WebView 底边，而不缩小控制器或暴露窗口底板。
class KeyboardAwareBridgeViewController: CAPBridgeViewController {
    private var legacyBottomConstraint: NSLayoutConstraint?
    private var legacyKeyboardObservers: [NSObjectProtocol] = []

    deinit {
        legacyKeyboardObservers.forEach { NotificationCenter.default.removeObserver($0) }
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        guard let webView else { return }

        let container = UIView(frame: view.bounds)
        container.backgroundColor = UIColor(red: 250 / 255, green: 251 / 255, blue: 253 / 255, alpha: 1)
        webView.translatesAutoresizingMaskIntoConstraints = false
        container.addSubview(webView)
        view = container
        var constraints = [
            webView.topAnchor.constraint(equalTo: container.topAnchor),
            webView.leadingAnchor.constraint(equalTo: container.leadingAnchor),
            webView.trailingAnchor.constraint(equalTo: container.trailingAnchor)
        ]
        if #available(iOS 17.0, *) {
            container.keyboardLayoutGuide.usesBottomSafeArea = false
            constraints.append(webView.bottomAnchor.constraint(equalTo: container.keyboardLayoutGuide.topAnchor))
        } else {
            // iOS 15/16 的 guide 在收起键盘后只到安全区顶部，会留下底部空白。
            let bottom = webView.bottomAnchor.constraint(equalTo: container.bottomAnchor)
            legacyBottomConstraint = bottom
            constraints.append(bottom)
            for name in [UIResponder.keyboardWillChangeFrameNotification, UIResponder.keyboardWillHideNotification] {
                legacyKeyboardObservers.append(NotificationCenter.default.addObserver(forName: name, object: nil, queue: .main) { [weak self] notification in
                    self?.resizeLegacyWebView(with: notification)
                })
            }
        }
        NSLayoutConstraint.activate(constraints)
    }

    private func resizeLegacyWebView(with notification: Notification) {
        guard let window = view.window, let bottom = legacyBottomConstraint,
              let info = notification.userInfo,
              let keyboardFrame = (info[UIResponder.keyboardFrameEndUserInfoKey] as? NSValue)?.cgRectValue else { return }
        let frame = window.convert(keyboardFrame, from: window.screen.coordinateSpace)
        let docked = frame.minX <= window.bounds.minX && frame.maxX >= window.bounds.maxX
        let overlap = docked ? min(window.bounds.height, max(0, window.bounds.maxY - frame.minY)) : 0
        let duration = (info[UIResponder.keyboardAnimationDurationUserInfoKey] as? NSNumber)?.doubleValue ?? 0
        let curve = (info[UIResponder.keyboardAnimationCurveUserInfoKey] as? NSNumber)?.uintValue ?? 0
        view.layoutIfNeeded()
        bottom.constant = -overlap
        UIView.animate(withDuration: duration, delay: 0,
                       options: UIView.AnimationOptions(rawValue: curve << 16).union(.beginFromCurrentState)) {
            self.view.layoutIfNeeded()
        }
    }
}

class SceneDelegate: UIResponder, UIWindowSceneDelegate {
    var window: UIWindow?

    func scene(_ scene: UIScene, willConnectTo session: UISceneSession, options connectionOptions: UIScene.ConnectionOptions) {
        guard let windowScene = scene as? UIWindowScene else { return }

        window = UIWindow(windowScene: windowScene)
        window?.backgroundColor = UIColor(red: 250 / 255, green: 251 / 255, blue: 253 / 255, alpha: 1)
        window?.rootViewController = KeyboardAwareBridgeViewController()
        window?.makeKeyAndVisible()

        SceneDelegateProxy.shared.scene(scene, willConnectTo: session, options: connectionOptions)
    }

    func scene(_ scene: UIScene, openURLContexts URLContexts: Set<UIOpenURLContext>) {
        SceneDelegateProxy.shared.scene(scene, openURLContexts: URLContexts)
    }

    func scene(_ scene: UIScene, continue userActivity: NSUserActivity) {
        SceneDelegateProxy.shared.scene(scene, continue: userActivity)
    }
}
