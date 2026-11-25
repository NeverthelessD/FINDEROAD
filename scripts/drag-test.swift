// ROAD 앱 창 옮기기 시험 (맥 빌드 서버 smoke-mac.sh 가 불러요)
// 창 맨 위 잡는 줄을 마우스로 끌어서 창이 실제로 움직이는지 봐요. 움직였으면 "moved dx dy" 를 찍고 0 으로 끝나요.
import Cocoa

let appName = CommandLine.arguments.count > 1 ? CommandLine.arguments[1] : "FINDEROAD"

func bounds() -> CGRect? {
    let list = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] ?? []
    for w in list where (w[kCGWindowOwnerName as String] as? String)?.lowercased() == appName.lowercased() && (w[kCGWindowLayer as String] as? Int) == 0 {
        if let d = w[kCGWindowBounds as String] as? NSDictionary, let r = CGRect(dictionaryRepresentation: d), r.width > 300 { return r }
    }
    return nil
}

func post(_ type: CGEventType, _ p: CGPoint) {
    CGEvent(mouseEventSource: nil, mouseType: type, mouseCursorPosition: p, mouseButton: .left)?.post(tap: .cghidEventTap)
    usleep(30_000)
}

guard let b0 = bounds() else { print("no-window"); exit(2) }
let start = CGPoint(x: b0.midX, y: b0.minY + 18)
post(.mouseMoved, start)
post(.leftMouseDown, start)
for i in 1...12 {
    post(.leftMouseDragged, CGPoint(x: start.x + CGFloat(i) * 10, y: start.y + CGFloat(i) * 6))
}
post(.leftMouseUp, CGPoint(x: start.x + 120, y: start.y + 72))
usleep(400_000)
guard let b1 = bounds() else { print("no-window-after"); exit(2) }
let dx = b1.minX - b0.minX, dy = b1.minY - b0.minY
print(abs(dx) > 40 || abs(dy) > 30 ? "moved \(Int(dx)) \(Int(dy))" : "not-moved \(Int(dx)) \(Int(dy)) at \(b0)")
exit(abs(dx) > 40 || abs(dy) > 30 ? 0 : 1)
