// FINDEROAD 끌어서 옮기기 시험 (맥 빌드 서버 smoke-mac.sh 가 불러요)
// 결과 첫 줄(from)을 마우스로 눌러 끌어서 '보낼 곳'(to)에 ⌥(복사)를 누른 채 놓아요. 사용: swift drag-file-test.swift fx fy tx ty
import Cocoa

let a = CommandLine.arguments.dropFirst().compactMap { Double($0) }
guard a.count == 4 else { print("usage"); exit(2) }
let from = CGPoint(x: a[0], y: a[1]), to = CGPoint(x: a[2], y: a[3])

func post(_ t: CGEventType, _ p: CGPoint, _ flags: CGEventFlags = []) {
    guard let e = CGEvent(mouseEventSource: nil, mouseType: t, mouseCursorPosition: p, mouseButton: .left) else { return }
    e.flags = flags
    e.post(tap: .cghidEventTap)
    usleep(20_000)
}
func option(_ down: Bool) {
    guard let e = CGEvent(keyboardEventSource: nil, virtualKey: 0x3A, keyDown: down) else { return }
    e.flags = down ? .maskAlternate : []
    e.post(tap: .cghidEventTap)
    usleep(60_000)
}

post(.mouseMoved, from)
post(.leftMouseDown, from)
usleep(200_000)
for i in 1...8 { post(.leftMouseDragged, CGPoint(x: from.x + CGFloat(i) * 2, y: from.y + CGFloat(i))) }   // 화면이 '끌기 시작'을 알림
usleep(500_000)
option(true)
let steps = 40
for i in 1...steps {
    let t = CGFloat(i) / CGFloat(steps)
    post(.leftMouseDragged, CGPoint(x: from.x + (to.x - from.x) * t, y: from.y + (to.y - from.y) * t), .maskAlternate)
}
for _ in 0..<10 { post(.leftMouseDragged, to, .maskAlternate); usleep(40_000) }   // 놓을 곳을 알아보는 시간
post(.leftMouseUp, to, .maskAlternate)
option(false)
usleep(300_000)
print("dropped \(Int(from.x)),\(Int(from.y)) → \(Int(to.x)),\(Int(to.y))")
