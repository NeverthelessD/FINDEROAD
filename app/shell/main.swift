// FINDEROAD — Dock 에 보이는 맥 앱 껍데기
// made by. Nevertheless_D
//
// 하는 일
//   1) 앱 안의 엔진(finderoad-engine)을 '이 앱의 자식'으로 켜고, 화면을 앱 자체 창(WKWebView)에 띄워요.
//   2) 앱을 종료(⌘Q)하면 엔진에게 끄라고 하고, 안 꺼지면 종료 신호를 보내서 모두 깔끔하게 끝내요.
//      앱이 갑자기 사라져도 엔진은 표준 입력이 닫히는 걸 보고 스스로 끝나요.
//      (숨은 도우미 앱 · 종료 신호 무시 · 엔진만 남겨 두는 실행기 같은 방식은 쓰지 않아요 — macOS '백그라운드 앱 활동' 규칙을 따름)
//   3) 화면이 부탁하는 맥 기능: Quick Look · 휴지통 · 클립보드(Finder 와 주고받기) · 폴더 고르기 · 폴더 끌어다 놓기 · 화면 모드
//   4) 엔진이 고장으로 꺼지면 다시 켜고, 업데이트 뒤에는 새 버전을 다시 열어요. 모든 단계를 launcher.log 에 적어요.

import AppKit
import Darwin
import Quartz
import WebKit

// MARK: - 경로 · 기록

enum Paths {
    static let support: URL = {
        let base = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
        return base.appendingPathComponent("FINDEROAD", isDirectory: true)
    }()
    static func file(_ name: String) -> URL { support.appendingPathComponent(name) }
}

enum Log {
    static func write(_ message: String) {
        let fm = FileManager.default
        try? fm.createDirectory(at: Paths.support, withIntermediateDirectories: true)
        let url = Paths.file("launcher.log")
        if let attrs = try? fm.attributesOfItem(atPath: url.path), let size = attrs[.size] as? NSNumber, size.intValue > 262_144 {
            let prev = Paths.file("launcher.prev.log")
            try? fm.removeItem(at: prev)
            try? fm.moveItem(at: url, to: prev)
        }
        let df = DateFormatter()
        df.locale = Locale(identifier: "en_US_POSIX")
        df.dateFormat = "MM-dd HH:mm:ss"
        let line = "\(df.string(from: Date())) [앱 \(getpid())] \(message)\n"
        guard let data = line.data(using: .utf8) else { return }
        if let h = try? FileHandle(forWritingTo: url) {
            h.seekToEndOfFile()
            h.write(data)
            h.closeFile()
        } else {
            try? data.write(to: url)
        }
    }
}

func jsonString(_ v: Any) -> String {
    guard JSONSerialization.isValidJSONObject(v), let d = try? JSONSerialization.data(withJSONObject: v), let s = String(data: d, encoding: .utf8) else { return "null" }
    return s
}

// 지우거나 옮기면 큰일 나는 곳은 막아요 (엔진의 protectedPath 와 같은 규칙)
func protectedPath(_ p: String) -> String? {
    let path = (p as NSString).standardizingPath
    if !path.hasPrefix("/") { return "경로가 올바르지 않아요" }
    let blocked = ["/", "/System", "/Library", "/Applications", "/Users", "/Volumes", "/private", "/usr", "/bin", "/sbin", "/etc", "/var", "/opt", "/cores", "/dev", "/tmp", "/Network"]
    if blocked.contains(path) { return "시스템에 꼭 필요한 폴더라서 할 수 없어요" }
    if path.hasPrefix("/System/") { return "시스템 파일이라서 할 수 없어요" }
    if path.hasPrefix("/Volumes/") && !path.dropFirst("/Volumes/".count).contains("/") { return "디스크 전체는 다룰 수 없어요" }
    let home = NSHomeDirectory()
    if path == home || (path as NSString).deletingLastPathComponent == "/Users" { return "사용자 폴더 전체는 다룰 수 없어요" }
    for n in ["Desktop", "Documents", "Downloads", "Library", "Movies", "Music", "Pictures", "Public", "Applications", ".Trash"] {
        if path == (home as NSString).appendingPathComponent(n) { return "맥의 기본 폴더라서 할 수 없어요" }
    }
    return nil
}

// MARK: - 엔진 (이 앱의 자식)

final class Engine {
    var process: Process?
    var port: Int = 0
    let token: String
    let build: String
    var onReady: ((Int) -> Void)?
    var onExit: ((Int32, Process.TerminationReason) -> Void)?
    private var stdinPipe: Pipe?
    private var outBuffer = Data()

    init() {
        let v = Bundle.main.url(forResource: "version", withExtension: "txt")
            .flatMap { try? String(contentsOf: $0, encoding: .utf8) } ?? "0.0.0"
        build = v.trimmingCharacters(in: .whitespacesAndNewlines)
        token = Engine.randomToken()
    }

    var isRunning: Bool { process?.isRunning ?? false }

    func start() throws {
        guard let exe = Bundle.main.url(forResource: "finderoad-engine", withExtension: nil) else {
            throw NSError(domain: "FINDEROAD", code: 1, userInfo: [NSLocalizedDescriptionKey: "앱 안에서 엔진을 찾지 못했어요"])
        }
        let fm = FileManager.default
        try? fm.createDirectory(at: Paths.support, withIntermediateDirectories: true)
        try? fm.removeItem(at: Paths.file("restart-requested"))
        // 지난번 엔진 출력은 engine.prev.out 으로
        let out = Paths.file("engine.out")
        try? fm.removeItem(at: Paths.file("engine.prev.out"))
        try? fm.moveItem(at: out, to: Paths.file("engine.prev.out"))
        fm.createFile(atPath: out.path, contents: nil)

        let p = Process()
        p.executableURL = exe
        var env = ProcessInfo.processInfo.environment
        for k in env.keys where k.hasPrefix("FINDEROAD_") && k != "FINDEROAD_TEST_CHOOSE" && k != "FINDEROAD_UPDATE_URL" { env.removeValue(forKey: k) }
        env["FINDEROAD_SHELL"] = "native"
        env["FINDEROAD_TOKEN"] = token
        env["FINDEROAD_SUPPORT"] = Paths.support.path
        env["FINDEROAD_APP"] = Bundle.main.bundlePath
        env["FINDEROAD_PARENT_PID"] = String(getpid())
        env["FINDEROAD_STDIN_WATCH"] = "1"
        p.environment = env
        // 표준 입력: 앱이 들고 있다가, 앱이 끝나면(어떤 방식으로든) 닫혀서 엔진이 알아채요
        let inPipe = Pipe()
        p.standardInput = inPipe
        stdinPipe = inPipe
        // 표준 출력: 엔진이 알려 주는 주소(FINDEROAD_PORT=…) 한 줄
        let outPipe = Pipe()
        p.standardOutput = outPipe
        p.standardError = FileHandle(forWritingAtPath: out.path) ?? FileHandle.nullDevice
        outBuffer = Data()
        port = 0
        outPipe.fileHandleForReading.readabilityHandler = { [weak self] h in
            let d = h.availableData
            if d.isEmpty { h.readabilityHandler = nil; return }
            DispatchQueue.main.async { self?.consume(d) }
        }
        p.terminationHandler = { [weak self] proc in
            let code = proc.terminationStatus
            let reason = proc.terminationReason
            DispatchQueue.main.async { self?.exited(code, reason) }
        }
        try p.run()
        process = p
        Log.write("엔진 켬 (pid \(p.processIdentifier), v\(build))")
    }

    private func consume(_ d: Data) {
        guard port == 0 else { return }
        outBuffer.append(d)
        guard let s = String(data: outBuffer, encoding: .utf8) else { return }
        for line in s.split(separator: "\n") where line.hasPrefix("FINDEROAD_PORT=") {
            if let v = Int(line.dropFirst("FINDEROAD_PORT=".count).trimmingCharacters(in: .whitespaces)), v > 0 {
                port = v
                Log.write("엔진 준비됨 (127.0.0.1:\(v))")
                onReady?(v)
                return
            }
        }
    }

    private func exited(_ code: Int32, _ reason: Process.TerminationReason) {
        process = nil
        try? stdinPipe?.fileHandleForWriting.close()
        stdinPipe = nil
        onExit?(code, reason)
    }

    /// 127.0.0.1 엔진에 짧게 요청해요
    func post(_ path: String, timeout: TimeInterval) {
        guard port > 0, let url = URL(string: "http://127.0.0.1:\(port)\(path)") else { return }
        var req = URLRequest(url: url, cachePolicy: .reloadIgnoringLocalCacheData, timeoutInterval: timeout)
        req.httpMethod = "POST"
        req.setValue(token, forHTTPHeaderField: "x-token")
        req.setValue("application/json", forHTTPHeaderField: "content-type")
        req.httpBody = "{}".data(using: .utf8)
        let sem = DispatchSemaphore(value: 0)
        let task = URLSession.shared.dataTask(with: req) { _, _, _ in sem.signal() }
        task.resume()
        if sem.wait(timeout: .now() + timeout + 0.5) == .timedOut { task.cancel() }
    }

    /// 앱을 종료할 때: 엔진에게 정중히 끄라고 하고, 안 꺼지면 종료 신호(TERM → KILL)를 보내요
    func stop(completion: @escaping () -> Void) {
        guard let p = process, p.isRunning else { completion(); return }
        DispatchQueue.global(qos: .userInitiated).async {
            self.post("/api/quit", timeout: 1.5)
            for _ in 0..<30 where p.isRunning { usleep(100_000) }
            if p.isRunning {
                p.terminate()
                for _ in 0..<20 where p.isRunning { usleep(100_000) }
            }
            if p.isRunning { kill(p.processIdentifier, SIGKILL) }
            DispatchQueue.main.async { completion() }
        }
    }

    static func randomToken() -> String {
        let chars = Array("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")
        return String((0..<32).map { _ in chars[Int(arc4random_uniform(UInt32(chars.count)))] })
    }
}

// MARK: - 창 · 화면

/// Quick Look 패널은 응답자 사슬에서 '내가 맡을게요' 하는 곳을 찾아요. 창이 그 역할을 해요.
final class MainWindow: NSWindow, QLPreviewPanelDataSource, QLPreviewPanelDelegate {
    var qlItems: [URL] = []

    override func acceptsPreviewPanelControl(_ panel: QLPreviewPanel!) -> Bool { true }

    override func beginPreviewPanelControl(_ panel: QLPreviewPanel!) {
        panel.dataSource = self
        panel.delegate = self
    }

    override func endPreviewPanelControl(_ panel: QLPreviewPanel!) {
        panel.dataSource = nil
        panel.delegate = nil
    }

    func numberOfPreviewItems(in panel: QLPreviewPanel!) -> Int { qlItems.count }

    func previewPanel(_ panel: QLPreviewPanel!, previewItemAt index: Int) -> QLPreviewItem! {
        qlItems[index] as NSURL
    }

    // Quick Look 창에서 Space 를 누르면 닫혀요 (Finder 처럼)
    func previewPanel(_ panel: QLPreviewPanel!, handle event: NSEvent!) -> Bool {
        if event.type == .keyDown && event.keyCode == 49 {
            panel.orderOut(nil)
            return true
        }
        return false
    }
}

/// 위쪽 막대의 빈 곳을 잡고 창을 옮길 수 있게 해요 (두 번 누르면 확대/축소)
final class DragView: NSView {
    override var mouseDownCanMoveWindow: Bool { true }
    override func acceptsFirstMouse(for event: NSEvent?) -> Bool { true }
    override func mouseDown(with event: NSEvent) {
        guard let w = window else { return }
        if event.clickCount == 2 {
            let action = UserDefaults.standard.string(forKey: "AppleActionOnDoubleClick") ?? "Maximize"
            if action == "Minimize" { w.performMiniaturize(nil) } else if action != "None" { w.performZoom(nil) }
            return
        }
        w.performDrag(with: event)
    }
}

/// Finder 에서 폴더를 끌어다 놓으면 검색 범위에 넣어요
final class FRWebView: WKWebView {
    var onFolders: (([String]) -> Void)?
    var onDragging: ((Bool) -> Void)?

    private func folderPaths(_ info: NSDraggingInfo) -> [String] {
        let urls = (info.draggingPasteboard.readObjects(forClasses: [NSURL.self], options: [.urlReadingFileURLsOnly: true]) as? [URL]) ?? []
        return urls.compactMap { u in
            let v = try? u.resourceValues(forKeys: [.isDirectoryKey, .isPackageKey])
            return (v?.isDirectory == true && v?.isPackage != true) ? u.path : nil
        }
    }

    override func draggingEntered(_ sender: NSDraggingInfo) -> NSDragOperation {
        if !folderPaths(sender).isEmpty { onDragging?(true); return .copy }
        return super.draggingEntered(sender)
    }

    override func draggingUpdated(_ sender: NSDraggingInfo) -> NSDragOperation {
        if !folderPaths(sender).isEmpty { return .copy }
        return super.draggingUpdated(sender)
    }

    override func draggingExited(_ sender: NSDraggingInfo?) {
        onDragging?(false)
        super.draggingExited(sender)
    }

    override func performDragOperation(_ sender: NSDraggingInfo) -> Bool {
        let dirs = folderPaths(sender)
        onDragging?(false)
        if !dirs.isEmpty { onFolders?(dirs); return true }
        return super.performDragOperation(sender)
    }
}

// MARK: - 앱

final class AppDelegate: NSObject, NSApplicationDelegate, WKNavigationDelegate, WKUIDelegate, NSWindowDelegate, WKScriptMessageHandlerWithReply {
    var window: MainWindow!
    var web: FRWebView!
    var dragViews: [DragView] = []
    let engine = Engine()
    var quitting = false
    var pageURL: URL?
    var crashes: [Date] = []
    var pageReady = false
    let headerHeight: CGFloat = 52
    let selftest = ProcessInfo.processInfo.environment["FINDEROAD_SELFTEST"] == "1"

    func applicationDidFinishLaunching(_ notification: Notification) {
        Log.write("시작 v\(engine.build) · macOS \(ProcessInfo.processInfo.operatingSystemVersionString) · \(Bundle.main.bundlePath)")
        buildMenu()
        makeWindow()
        showLoading()
        engine.onReady = { [weak self] port in self?.loadApp(port: port) }
        engine.onExit = { [weak self] code, reason in self?.engineExited(code: code, reason: reason) }
        startEngine()
    }

    func startEngine() {
        do {
            try engine.start()
            DispatchQueue.main.asyncAfter(deadline: .now() + 20) { [weak self] in
                guard let self = self, !self.quitting, self.engine.port == 0, self.engine.isRunning else { return }
                Log.write("⚠️ 엔진이 20초 안에 준비되지 않았어요")
                self.startFailed(NSError(domain: "FINDEROAD", code: 3, userInfo: [NSLocalizedDescriptionKey: "엔진이 준비되지 않았어요"]))
            }
        } catch {
            Log.write("⚠️ 시작 실패: \(error.localizedDescription)")
            startFailed(error)
        }
    }

    func loadApp(port: Int) {
        guard let url = URL(string: "http://127.0.0.1:\(port)/?t=\(engine.token)") else { return }
        pageURL = url
        pageReady = false
        web.load(URLRequest(url: url))
    }

    func startFailed(_ error: Error) {
        let a = NSAlert()
        a.alertStyle = .warning
        a.messageText = "FINDEROAD를 시작하지 못했어요"
        a.informativeText = "\(error.localizedDescription)\n앱을 다시 실행해 보세요. 계속 안 되면 맥을 재시동한 뒤 시도해 주세요."
        a.addButton(withTitle: "다시 시도")
        a.addButton(withTitle: "종료")
        if a.runModal() == .alertFirstButtonReturn {
            engine.stop { self.startEngine() }
        } else {
            quitting = true
            NSApp.terminate(nil)
        }
    }

    func engineExited(code: Int32, reason: Process.TerminationReason) {
        if quitting { return }
        if FileManager.default.fileExists(atPath: Paths.file("restart-requested").path) {
            relaunch()
            return
        }
        if reason == .exit && code == 0 {
            Log.write("엔진이 끝나서 앱도 닫아요")
            quitting = true
            NSApp.terminate(nil)
            return
        }
        // 고장으로 꺼짐 → 2분에 3번까지 다시 켜요
        let now = Date()
        crashes = crashes.filter { now.timeIntervalSince($0) < 120 } + [now]
        Log.write("⚠️ 엔진이 갑자기 꺼졌어요 (\(reason == .uncaughtSignal ? "신호" : "코드") \(code))")
        if crashes.count > 3 {
            startFailed(NSError(domain: "FINDEROAD", code: 4, userInfo: [NSLocalizedDescriptionKey: "엔진이 계속 멈춰요"]))
            return
        }
        showLoading()
        startEngine()
    }

    /// 업데이트된 새 버전을 macOS 정식 방법(LaunchServices)으로 다시 열고, 지금 앱은 끝내요
    func relaunch() {
        Log.write("새 버전으로 다시 열어요")
        quitting = true
        try? FileManager.default.removeItem(at: Paths.file("restart-requested"))
        let appURL = URL(fileURLWithPath: Bundle.main.bundlePath)
        let cfg = NSWorkspace.OpenConfiguration()
        cfg.createsNewApplicationInstance = true
        cfg.activates = true
        NSWorkspace.shared.openApplication(at: appURL, configuration: cfg) { _, error in
            if let error = error { Log.write("⚠️ 다시 열지 못했어요: \(error.localizedDescription)") }
            DispatchQueue.main.async { NSApp.terminate(nil) }
        }
    }

    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        if !flag { showMain() }
        return true
    }

    // 창을 닫아도 앱은 Dock 에 남아요 (Dock 아이콘을 누르면 다시 열림). 끝내려면 ⌘Q.
    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { false }

    func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
        quitting = true
        guard engine.isRunning else { return .terminateNow }
        Log.write("종료: 엔진을 끄는 중")
        engine.stop {
            Log.write("종료 완료")
            NSApp.reply(toApplicationShouldTerminate: true)
        }
        return .terminateLater
    }

    // MARK: 창

    func makeWindow() {
        let cfg = WKWebViewConfiguration()
        cfg.applicationNameForUserAgent = "FINDEROADShell/\(engine.build)"
        cfg.userContentController.addScriptMessageHandler(self, contentWorld: .page, name: "fr")
        if #available(macOS 12.3, *) { cfg.preferences.isElementFullscreenEnabled = true }
        web = FRWebView(frame: NSRect(x: 0, y: 0, width: 1320, height: 860), configuration: cfg)
        web.navigationDelegate = self
        web.uiDelegate = self
        web.allowsMagnification = false
        web.autoresizingMask = [.width, .height]
        if #available(macOS 13.3, *) { web.isInspectable = true }
        web.onFolders = { [weak self] paths in self?.js("window.FR && FR.drop(\(jsonString(paths)))") }
        web.onDragging = { [weak self] on in self?.js("window.FR && FR.dragging(\(on))") }

        window = MainWindow(contentRect: NSRect(x: 0, y: 0, width: 1320, height: 860),
                            styleMask: [.titled, .closable, .miniaturizable, .resizable, .fullSizeContentView],
                            backing: .buffered, defer: false)
        window.title = "FINDEROAD"
        window.titleVisibility = .hidden
        window.titlebarAppearsTransparent = true
        window.isMovableByWindowBackground = false
        window.minSize = NSSize(width: 1040, height: 640)
        window.isReleasedWhenClosed = false
        window.tabbingMode = .disallowed
        window.collectionBehavior.insert(.fullScreenPrimary)
        window.delegate = self
        window.backgroundColor = NSColor(calibratedRed: 0.925, green: 0.925, blue: 0.906, alpha: 1)
        let container = NSView(frame: NSRect(x: 0, y: 0, width: 1320, height: 860))
        container.autoresizingMask = [.width, .height]
        web.frame = container.bounds
        container.addSubview(web)
        window.contentView = container
        if !window.setFrameUsingName("FINDEROADMain") { window.center() }
        window.setFrameAutosaveName("FINDEROADMain")
        window.makeKeyAndOrderFront(nil)
        layoutTrafficLights()
        NSApp.activate(ignoringOtherApps: true)
    }

    /// 신호등 버튼(닫기 · 최소화 · 확대)을 화면 위쪽 막대(52pt) 가운데 높이에 맞춰요
    func layoutTrafficLights() {
        guard !window.styleMask.contains(.fullScreen),
              let close = window.standardWindowButton(.closeButton),
              let mini = window.standardWindowButton(.miniaturizeButton),
              let zoom = window.standardWindowButton(.zoomButton),
              let titlebar = close.superview,
              let container = titlebar.superview else { return }
        var cf = container.frame
        cf.size.height = headerHeight
        cf.origin.y = window.frame.height - headerHeight
        container.frame = cf
        titlebar.frame = NSRect(x: 0, y: 0, width: cf.width, height: headerHeight)
        for (i, b) in [close, mini, zoom].enumerated() {
            let h = b.frame.height
            b.setFrameOrigin(NSPoint(x: 20 + CGFloat(i) * 20, y: (headerHeight - h) / 2))
        }
    }

    func windowDidResize(_ notification: Notification) { layoutTrafficLights() }
    func windowDidEndLiveResize(_ notification: Notification) { layoutTrafficLights() }
    func windowDidBecomeKey(_ notification: Notification) { layoutTrafficLights() }
    func windowDidEnterFullScreen(_ notification: Notification) { js("window.FR && FR.fullscreen(true)") }
    func windowDidExitFullScreen(_ notification: Notification) {
        layoutTrafficLights()
        js("window.FR && FR.fullscreen(false)")
    }

    /// 화면이 알려 준 '창을 잡고 옮길 수 있는 곳'에 투명한 손잡이를 깔아요
    func layoutDragViews(_ rects: [[String: Any]]) {
        dragViews.forEach { $0.removeFromSuperview() }
        dragViews = []
        guard let container = window.contentView else { return }
        let H = container.bounds.height
        for r in rects {
            let x = (r["x"] as? NSNumber)?.doubleValue ?? 0
            let y = (r["y"] as? NSNumber)?.doubleValue ?? 0
            let w = (r["w"] as? NSNumber)?.doubleValue ?? 0
            let h = (r["h"] as? NSNumber)?.doubleValue ?? 0
            if w < 4 || h < 4 { continue }
            let v = DragView(frame: NSRect(x: x, y: Double(H) - y - h, width: w, height: h))
            v.autoresizingMask = []
            container.addSubview(v, positioned: .above, relativeTo: web)
            dragViews.append(v)
        }
    }

    func showLoading() {
        let html = """
        <!doctype html><meta charset="utf-8"><style>
        :root{color-scheme:light dark}
        body{margin:0;height:100vh;display:grid;place-items:center;font:600 13px -apple-system,"Apple SD Gothic Neo",sans-serif;
             background:#F4F4F1;color:#5E5E66}
        @media (prefers-color-scheme: dark){body{background:#111113;color:#A1A1AA}}
        .w{display:flex;flex-direction:column;align-items:center;gap:14px}
        .s{width:22px;height:22px;border-radius:50%;border:2.5px solid rgba(128,128,128,.3);border-top-color:#C2410C;animation:r .9s linear infinite}
        @keyframes r{to{transform:rotate(360deg)}}
        </style><div class="w"><div class="s"></div>FINDEROAD 준비 중…</div>
        """
        web.loadHTMLString(html, baseURL: nil)
    }

    func js(_ code: String) {
        web?.evaluateJavaScript(code, completionHandler: nil)
    }

    @objc func showMain() {
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }

    @objc func reloadPage() {
        if let u = pageURL { web.load(URLRequest(url: u)) }
    }

    @objc func checkForUpdates() {
        showMain()
        js("document.getElementById('updBtn') && (document.getElementById('updPop').hidden ? document.getElementById('updBtn').click() : 0)")
    }

    @objc func focusSearch() {
        showMain()
        js("document.getElementById('qName') && document.getElementById('qName').focus()")
    }

    // MARK: 화면 ↔ 맥 기능

    func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage,
                               replyHandler: @escaping (Any?, String?) -> Void) {
        // 우리 엔진 화면에서 온 부탁만 들어요
        let origin = message.frameInfo.securityOrigin
        guard origin.host == "127.0.0.1", origin.port == engine.port, engine.port > 0 else {
            replyHandler(nil, "not allowed")
            return
        }
        guard let body = message.body as? [String: Any], let cmd = body["cmd"] as? String else {
            replyHandler(nil, "bad request")
            return
        }
        let paths = (body["paths"] as? [String]) ?? []
        switch cmd {
        case "ready":
            pageReady = true
            replyHandler(["ok": true], nil)
            if selftest { DispatchQueue.main.asyncAfter(deadline: .now() + 2) { self.runSelftest() } }

        case "appearance":
            let mode = body["mode"] as? String ?? "system"
            NSApp.appearance = mode == "dark" ? NSAppearance(named: .darkAqua) : (mode == "light" ? NSAppearance(named: .aqua) : nil)
            if let hex = body["color"] as? String, let c = NSColor(hex: hex) { window.backgroundColor = c }
            replyHandler(["ok": true], nil)

        case "dragRegions":
            layoutDragViews((body["rects"] as? [[String: Any]]) ?? [])
            replyHandler(["ok": true, "count": dragViews.count], nil)

        case "chooseFolders":
            if let t = ProcessInfo.processInfo.environment["FINDEROAD_TEST_CHOOSE"], !t.isEmpty {
                replyHandler(["paths": t.split(separator: "|").map(String.init)], nil)
                return
            }
            let panel = NSOpenPanel()
            panel.canChooseDirectories = true
            panel.canChooseFiles = false
            panel.canCreateDirectories = true
            panel.allowsMultipleSelection = (body["multiple"] as? Bool) ?? false
            panel.message = body["prompt"] as? String ?? "폴더를 골라 주세요"
            panel.prompt = body["button"] as? String ?? "선택"
            panel.beginSheetModal(for: window) { r in
                replyHandler(["paths": r == .OK ? panel.urls.map { $0.path } : []], nil)
            }

        case "trash":
            var items: [[String: String]] = []
            var errors: [[String: String]] = []
            for p in paths {
                if let why = protectedPath(p) { errors.append(["path": p, "error": why]); continue }
                var res: NSURL?
                do {
                    try FileManager.default.trashItem(at: URL(fileURLWithPath: p), resultingItemURL: &res)
                    items.append(["from": p, "to": res?.path ?? ""])
                } catch {
                    let ns = error as NSError
                    let msg = ns.code == NSFileNoSuchFileError ? "파일을 찾을 수 없어요" : (ns.code == NSFileWriteNoPermissionError ? "권한이 없어요" : ns.localizedDescription)
                    errors.append(["path": p, "error": msg])
                }
            }
            Log.write("휴지통으로 \(items.count)개" + (errors.isEmpty ? "" : " · 실패 \(errors.count)개"))
            replyHandler(["ok": errors.isEmpty, "items": items, "errors": errors], nil)

        case "pbWriteFiles":
            let pb = NSPasteboard.general
            pb.clearContents()
            pb.writeObjects(paths.map { URL(fileURLWithPath: $0) as NSURL })
            replyHandler(["ok": true], nil)

        case "pbReadFiles":
            let urls = (NSPasteboard.general.readObjects(forClasses: [NSURL.self], options: [.urlReadingFileURLsOnly: true]) as? [URL]) ?? []
            replyHandler(["paths": urls.map { $0.path }], nil)

        case "pbWriteText":
            let pb = NSPasteboard.general
            pb.clearContents()
            pb.setString(body["text"] as? String ?? "", forType: .string)
            replyHandler(["ok": true], nil)

        case "quicklook":
            let urls = paths.map { URL(fileURLWithPath: $0) }
            guard !urls.isEmpty, let panel = QLPreviewPanel.shared() else { replyHandler(["ok": false], nil); return }
            if QLPreviewPanel.sharedPreviewPanelExists() && panel.isVisible && urls == window.qlItems {
                panel.orderOut(nil)
                replyHandler(["ok": true, "closed": true], nil)
                return
            }
            window.qlItems = urls
            window.makeFirstResponder(web)
            panel.updateController()
            panel.reloadData()
            let idx = (body["index"] as? Int) ?? 0
            panel.currentPreviewItemIndex = max(0, min(idx, urls.count - 1))
            panel.makeKeyAndOrderFront(nil)
            replyHandler(["ok": true], nil)

        default:
            replyHandler(nil, "unknown command")
        }
    }

    // MARK: WKWebView

    func webView(_ webView: WKWebView, decidePolicyFor action: WKNavigationAction,
                 decisionHandler: @escaping (WKNavigationActionPolicy) -> Void) {
        guard let u = action.request.url else { decisionHandler(.allow); return }
        let scheme = u.scheme ?? ""
        if scheme == "about" || scheme == "data" || scheme == "blob" || u.host == "127.0.0.1" {
            decisionHandler(.allow)
            return
        }
        NSWorkspace.shared.open(u)
        decisionHandler(.cancel)
    }

    func webView(_ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration,
                 for action: WKNavigationAction, windowFeatures: WKWindowFeatures) -> WKWebView? {
        if let u = action.request.url { NSWorkspace.shared.open(u) }
        return nil
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        guard pageURL != nil, !quitting else { return }
        Log.write("⚠️ 화면을 불러오지 못함: \(error.localizedDescription) → 다시 시도")
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) { [weak self] in self?.reloadPage() }
    }

    func webViewWebContentProcessDidTerminate(_ webView: WKWebView) {
        Log.write("⚠️ 화면 프로세스가 멈춤 → 다시 불러와요")
        reloadPage()
    }

    func webView(_ webView: WKWebView, runJavaScriptAlertPanelWithMessage message: String,
                 initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping () -> Void) {
        let a = NSAlert()
        a.messageText = message
        a.addButton(withTitle: "확인")
        a.beginSheetModal(for: window) { _ in completionHandler() }
    }

    func webView(_ webView: WKWebView, runJavaScriptConfirmPanelWithMessage message: String,
                 initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping (Bool) -> Void) {
        let a = NSAlert()
        a.messageText = message
        a.addButton(withTitle: "확인")
        a.addButton(withTitle: "취소")
        a.beginSheetModal(for: window) { r in completionHandler(r == .alertFirstButtonReturn) }
    }

    // MARK: 메뉴

    func buildMenu() {
        let main = NSMenu()

        let appItem = NSMenuItem()
        main.addItem(appItem)
        let appMenu = NSMenu()
        appMenu.addItem(withTitle: "FINDEROAD에 관하여", action: #selector(NSApplication.orderFrontStandardAboutPanel(_:)), keyEquivalent: "")
        let upd = appMenu.addItem(withTitle: "업데이트 확인…", action: #selector(checkForUpdates), keyEquivalent: "")
        upd.target = self
        appMenu.addItem(NSMenuItem.separator())
        appMenu.addItem(withTitle: "FINDEROAD 가리기", action: #selector(NSApplication.hide(_:)), keyEquivalent: "h")
        let others = appMenu.addItem(withTitle: "기타 가리기", action: #selector(NSApplication.hideOtherApplications(_:)), keyEquivalent: "h")
        others.keyEquivalentModifierMask = [.command, .option]
        appMenu.addItem(withTitle: "모두 보기", action: #selector(NSApplication.unhideAllApplications(_:)), keyEquivalent: "")
        appMenu.addItem(NSMenuItem.separator())
        appMenu.addItem(withTitle: "FINDEROAD 종료", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        appItem.submenu = appMenu

        let editItem = NSMenuItem()
        main.addItem(editItem)
        let edit = NSMenu(title: "편집")
        edit.addItem(withTitle: "실행 취소", action: Selector(("undo:")), keyEquivalent: "z")
        let redo = edit.addItem(withTitle: "실행 복귀", action: Selector(("redo:")), keyEquivalent: "z")
        redo.keyEquivalentModifierMask = [.command, .shift]
        edit.addItem(NSMenuItem.separator())
        edit.addItem(withTitle: "오려두기", action: #selector(NSText.cut(_:)), keyEquivalent: "x")
        edit.addItem(withTitle: "복사하기", action: #selector(NSText.copy(_:)), keyEquivalent: "c")
        edit.addItem(withTitle: "붙여넣기", action: #selector(NSText.paste(_:)), keyEquivalent: "v")
        edit.addItem(withTitle: "모두 선택", action: #selector(NSText.selectAll(_:)), keyEquivalent: "a")
        edit.addItem(NSMenuItem.separator())
        let find = edit.addItem(withTitle: "찾기", action: #selector(focusSearch), keyEquivalent: "f")
        find.target = self
        editItem.submenu = edit

        let viewItem = NSMenuItem()
        main.addItem(viewItem)
        let view = NSMenu(title: "보기")
        let fs = view.addItem(withTitle: "전체 화면", action: #selector(NSWindow.toggleFullScreen(_:)), keyEquivalent: "f")
        fs.keyEquivalentModifierMask = [.command, .control]
        viewItem.submenu = view

        let winItem = NSMenuItem()
        main.addItem(winItem)
        let win = NSMenu(title: "윈도우")
        win.addItem(withTitle: "최소화", action: #selector(NSWindow.performMiniaturize(_:)), keyEquivalent: "m")
        win.addItem(withTitle: "확대/축소", action: #selector(NSWindow.performZoom(_:)), keyEquivalent: "")
        win.addItem(withTitle: "닫기", action: #selector(NSWindow.performClose(_:)), keyEquivalent: "w")
        win.addItem(NSMenuItem.separator())
        let show = win.addItem(withTitle: "FINDEROAD 창 보기", action: #selector(showMain), keyEquivalent: "")
        show.target = self
        winItem.submenu = win

        NSApp.mainMenu = main
        NSApp.windowsMenu = win
    }

    // MARK: 시험 (맥 빌드 서버에서만: FINDEROAD_SELFTEST=1)

    func evalJS(_ code: String, _ done: @escaping (Any?) -> Void) {
        web.evaluateJavaScript(code) { v, _ in done(v) }
    }

    func runSelftest() {
        var r: [String: Any] = ["version": engine.build]
        let fm = FileManager.default
        let tmpDir = Paths.support.appendingPathComponent("selftest-tmp", isDirectory: true)
        try? fm.removeItem(at: tmpDir)
        try? fm.createDirectory(at: tmpDir, withIntermediateDirectories: true)
        let sample = tmpDir.appendingPathComponent("시험 파일.txt")
        try? "FINDEROAD".write(to: sample, atomically: true, encoding: .utf8)

        // 1) 신호등 버튼이 위쪽 막대 가운데에 있는지
        if let close = window.standardWindowButton(.closeButton) {
            let f = close.convert(close.bounds, to: nil)
            let topGap = window.frame.height - f.maxY
            r["trafficLightTop"] = Double(topGap)
            r["trafficLightCentered"] = abs((topGap + f.height / 2) - headerHeight / 2) < 3
        }
        r["dragViews"] = dragViews.count

        // 2) 휴지통 (macOS 휴지통 기능) → 원래 자리로 되돌리기
        var res: NSURL?
        do {
            try fm.trashItem(at: sample, resultingItemURL: &res)
            let inTrash = res?.path ?? ""
            let ok = !inTrash.isEmpty && fm.fileExists(atPath: inTrash) && !fm.fileExists(atPath: sample.path)
            if ok { try? fm.moveItem(atPath: inTrash, toPath: sample.path) }
            r["trash"] = ok && fm.fileExists(atPath: sample.path)
        } catch {
            r["trash"] = false
            r["trashError"] = error.localizedDescription
        }

        // 3) 클립보드: 파일 쓰고 읽기 (Finder 와 같은 형식)
        let pb = NSPasteboard.general
        pb.clearContents()
        pb.writeObjects([sample as NSURL])
        let back = (pb.readObjects(forClasses: [NSURL.self], options: [.urlReadingFileURLsOnly: true]) as? [URL]) ?? []
        r["pasteboard"] = back.first?.path == sample.path

        // 4) 위쪽 막대의 버튼이 눌리는지 (신호등 · 끌기 영역이 가로막지 않는지)
        evalJS("(()=>{const b=document.querySelector('[data-theme-pick=\"dark\"]').getBoundingClientRect();return [b.left+b.width/2,b.top+b.height/2]})()") { v in
            if let a = v as? [NSNumber], a.count == 2, let container = self.window.contentView {
                let p = NSPoint(x: a[0].doubleValue, y: Double(container.bounds.height) - a[1].doubleValue)
                self.click(at: p)
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.8) {
                self.evalJS("document.documentElement.dataset.theme + '|' + document.querySelector('[data-theme-pick=\"dark\"]').getAttribute('aria-pressed')") { v in
                    r["headerClick"] = (v as? String) == "dark|true"
                    r["headerClickRaw"] = v as? String ?? ""
                    // 5) 키보드 단축키(⌘C)가 화면까지 가는지
                    self.evalJS("window.__k=[];document.addEventListener('keydown',e=>window.__k.push(e.code+(e.metaKey?'+m':'')),true);document.activeElement&&document.activeElement.blur();1") { _ in
                        self.window.makeKeyAndOrderFront(nil)
                        self.key("c", code: 8, flags: .command)
                        DispatchQueue.main.asyncAfter(deadline: .now() + 0.8) {
                            self.evalJS("window.__k.join(',')") { v in
                                r["keyToPage"] = (v as? String ?? "").contains("KeyC+m")
                                r["keyRaw"] = v as? String ?? ""
                                // 6) Quick Look 창
                                self.window.qlItems = [sample]
                                if let panel = QLPreviewPanel.shared() {
                                    panel.updateController()
                                    panel.reloadData()
                                    panel.makeKeyAndOrderFront(nil)
                                }
                                DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) {
                                    r["quicklook"] = QLPreviewPanel.sharedPreviewPanelExists() && (QLPreviewPanel.shared()?.isVisible ?? false)
                                    QLPreviewPanel.shared()?.orderOut(nil)
                                    self.js("document.querySelector('[data-theme-pick=\"system\"]').click()")
                                    try? fm.removeItem(at: tmpDir)
                                    let out = Paths.file("selftest.json")
                                    try? jsonString(r).write(to: out, atomically: true, encoding: .utf8)
                                    Log.write("자체 시험 결과: \(jsonString(r))")
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    func click(at p: NSPoint) {
        for type in [NSEvent.EventType.leftMouseDown, NSEvent.EventType.leftMouseUp] {
            if let e = NSEvent.mouseEvent(with: type, location: p, modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
                                          windowNumber: window.windowNumber, context: nil, eventNumber: 0, clickCount: 1,
                                          pressure: type == .leftMouseDown ? 1 : 0) {
                window.sendEvent(e)
            }
        }
    }

    func key(_ chars: String, code: UInt16, flags: NSEvent.ModifierFlags) {
        for type in [NSEvent.EventType.keyDown, NSEvent.EventType.keyUp] {
            if let e = NSEvent.keyEvent(with: type, location: .zero, modifierFlags: flags, timestamp: ProcessInfo.processInfo.systemUptime,
                                        windowNumber: window.windowNumber, context: nil, characters: chars,
                                        charactersIgnoringModifiers: chars, isARepeat: false, keyCode: code) {
                NSApp.sendEvent(e)
            }
        }
    }
}

extension NSColor {
    convenience init?(hex: String) {
        var s = hex.trimmingCharacters(in: .whitespaces)
        if s.hasPrefix("#") { s.removeFirst() }
        guard s.count == 6, let v = UInt32(s, radix: 16) else { return nil }
        self.init(srgbRed: CGFloat((v >> 16) & 0xff) / 255, green: CGFloat((v >> 8) & 0xff) / 255, blue: CGFloat(v & 0xff) / 255, alpha: 1)
    }
}

// MARK: - 시작

let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.setActivationPolicy(.regular)
app.run()
