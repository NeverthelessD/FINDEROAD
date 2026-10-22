#!/bin/bash
# FINDEROAD 실제 맥 시험 (맥 빌드 서버에서 dist/FINDEROAD.app 을 /Applications 에 깔고 여러 상황을 시험해요)
# 결과: $OUT/results.txt (PASS/FAIL 한 줄씩), 스크린샷, 자체 시험 결과, 기록 파일
set -uo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="${OUT:-$ROOT/ci-out}"
mkdir -p "$OUT"
APPDIR="/Applications/FINDEROAD.app"
SUP="$HOME/Library/Application Support/FINDEROAD"
RES="$OUT/results.txt"
: > "$RES"

pass() { echo "PASS  $*" | tee -a "$RES"; }
fail() { echo "FAIL  $*" | tee -a "$RES"; }
check() { local name="$1"; shift; if "$@"; then pass "$name"; else fail "$name"; fi; }

app_pid() { pgrep -x FINDEROAD | head -1; }
tok() { cat "$SUP/token" 2>/dev/null; }
port() { cat "$SUP/port" 2>/dev/null; }
get() { curl -fs -m 60 -H "x-token: $(tok)" "http://127.0.0.1:$(port)$1"; }
post() { curl -fs -m 60 -H "x-token: $(tok)" -H 'content-type: application/json' -X POST -d "$2" "http://127.0.0.1:$(port)$1"; }
ping_ok() { get /api/ping >/dev/null 2>&1; }
wait_ready() { for _ in $(seq 1 80); do [ -s "$SUP/port" ] && ping_ok && return 0; sleep 0.5; done; return 1; }
wait_gone() { for _ in $(seq 1 30); do [ -z "$(app_pid)" ] && [ -z "$(pgrep -x finderoad-engine)" ] && return 0; sleep 0.5; done; return 1; }
snap() { { echo "--- $1"; ps -axo pid,ppid,stat,comm | grep -E "FINDEROAD|finderoad-engine" | grep -v grep; } >> "$OUT/procs.txt"; }
launch() { rm -f "$SUP/port"; open "$@" "$APPDIR"; wait_ready; }
quit_app() { osascript -e 'tell application "FINDEROAD" to quit' >/dev/null 2>&1; wait_gone; }
js() { python3 -c "import json,sys; d=json.load(sys.stdin); print($1)"; }
eng_pid() { get /api/ping | js "d['pid']"; }

echo "▶ 준비: 앱 설치 · 시험용 폴더 · 설정"
rm -rf "$APPDIR"; ditto "$ROOT/dist/FINDEROAD.app" "$APPDIR"
T="$HOME/FINDEROAD-test"; D="$HOME/FINDEROAD-dest"; rm -rf "$T" "$D"; mkdir -p "$T/영상/깊은/폴더" "$T/사진" "$T/문서" "$D"
python3 - "$T" <<'PY'
import os, sys, unicodedata, shutil
t = sys.argv[1]
def w(p, data=b"x" * 2048):
    os.makedirs(os.path.dirname(p), exist_ok=True)
    open(p, "wb").write(data)
w(os.path.join(t, "영상", unicodedata.normalize("NFD", "제주여행_브이로그.mp4")))   # Finder 처럼 자모가 나뉜 이름
w(os.path.join(t, "영상", "회의녹화.MOV"))
w(os.path.join(t, "영상", "깊은", "폴더", "드론.mp4"))
w(os.path.join(t, "영상", ".숨김.mp4"))
w(os.path.join(t, "문서", "메모.txt"), "FINDEROAD 시험\n안녕하세요\n".encode())
w(os.path.join(t, "문서", "finderoad스모크고유.pdf"))
PY
cp "$ROOT/app/AppIcon.png" "$T/사진/아이콘.png"
mkdir -p "$SUP"
printf '{"appearance":"light","autoUpdate":false,"scope":"folders","folders":["%s"],"showHidden":false,"ui":{}}' "$T" > "$SUP/settings.json"
mdutil -s / > "$OUT/spotlight.txt" 2>&1

echo "▶ A. 켜기 · 자체 시험(Quick Look · 휴지통 · 클립보드 · 위쪽 막대 · 단축키)"
launch --env FINDEROAD_SELFTEST=1
check "A1 앱을 열면 엔진이 준비됨" ping_ok
check "A2 엔진이 Dock 앱 방식으로 켜짐(shell=native)" bash -c "curl -fs -H 'x-token: $(tok)' http://127.0.0.1:$(port)/api/ping | grep -q '\"shell\":\"native\"'"
APID=$(app_pid); EPPID=$(get /api/ping | js "d['ppid']"); EPID=$(eng_pid)
check "A3 엔진이 FINDEROAD 앱의 자식 (앱 $APID · 엔진 $EPID 의 부모 $EPPID)" test -n "$APID" -a "$EPPID" = "$APID"
check "A4 엔진은 하나만 (지금 $(pgrep -x finderoad-engine | wc -l | tr -d ' ')개)" test "$(pgrep -x finderoad-engine | wc -l | tr -d ' ')" = "1"
snap "A 실행 중"
for _ in $(seq 1 60); do [ -s "$SUP/selftest.json" ] && break; sleep 0.5; done
cp "$SUP/selftest.json" "$OUT/selftest.json" 2>/dev/null
ST="$(cat "$SUP/selftest.json" 2>/dev/null || echo '{}')"
stj() { echo "$ST" | js "$1"; }
check "S1 위쪽 막대 버튼이 눌림 (${ST:+$(stj "d.get('headerClickRaw')")})" test "$(stj "d.get('headerClick')")" = "True"
check "S2 ⌘C 같은 단축키가 화면까지 감 ($(stj "d.get('keyRaw')"))" test "$(stj "d.get('keyToPage')")" = "True"
check "S3 휴지통으로 옮기고 되돌리기" test "$(stj "d.get('trash')")" = "True"
check "S4 클립보드에 파일 쓰고 읽기 (Finder 형식)" test "$(stj "d.get('pasteboard')")" = "True"
check "S5 Quick Look 창이 열림" test "$(stj "d.get('quicklook')")" = "True"
check "S6 신호등 버튼이 위쪽 막대 가운데 (위 여백 $(stj "d.get('trafficLightTop')"))" test "$(stj "d.get('trafficLightCentered')")" = "True"
check "S7 창 끌기 영역이 깔림 ($(stj "d.get('dragViews')")개)" bash -c "[ \"$(stj "d.get('dragViews', 0)")\" -gt 0 ]"
check "S8 동영상 분류 버튼으로 찾은 결과가 화면에 나옴 ($(stj "d.get('videoRows')")줄)" bash -c "[ \"$(stj "d.get('videoRows', 0)")\" -ge 3 ]"
sleep 1
screencapture -x "$OUT/shot-screen.png"
cp "$SUP/shot-light.png" "$OUT/" 2>/dev/null; cp "$SUP/shot-dark.png" "$OUT/" 2>/dev/null

echo "▶ B. 검색"
q() { post /api/search "$1"; }
N=$(q "{\"ext\":\"mp4, mov\",\"scope\":\"folders\",\"folders\":[\"$T\"]}" | js "sorted(i['name'] for i in d['items'])")
check "B1 확장자만으로 찾기 (mp4, mov → $N)" test "$N" = "['드론.mp4', '제주여행_브이로그.mp4', '회의녹화.MOV']"
N=$(q "{\"name\":\"제주 브이로그\",\"scope\":\"folders\",\"folders\":[\"$T\"]}" | js "[i['name'] for i in d['items']]")
check "B2 한글 이름(자모가 나뉜 파일 이름)도 찾기 ($N)" test "$N" = "['제주여행_브이로그.mp4']"
N=$(q "{\"ext\":\"mp4\",\"scope\":\"folders\",\"folders\":[\"$T\"],\"hidden\":true}" | js "len(d['items'])")
check "B3 숨김 파일은 '숨김 파일도 찾기'일 때만 (${N}개)" test "$N" = "3"
FOUND=""; METHOD=""
for _ in $(seq 1 20); do
  R=$(q '{"name":"finderoad스모크고유","scope":"all"}')
  FOUND=$(echo "$R" | js "len(d['items'])"); METHOD=$(echo "$R" | js "d['method']")
  [ "${FOUND:-0}" -ge 1 ] && break; sleep 2
done
SHELLMD=$(mdfind -onlyin "$T" 'kMDItemFSName == "*finderoad*"c' 2>/dev/null | wc -l | tr -d ' ')
if [ "${FOUND:-0}" -ge 1 ]; then
  pass "B4 Mac 전체에서 찾기 (방법: $METHOD, ${FOUND}개)"
elif [ "$METHOD" = "spotlight" ] && [ "${SHELLMD:-0}" = "0" ]; then
  pass "B4 Mac 전체에서 찾기: 엔진이 Spotlight 에 물어봄 (이 빌드 서버의 Spotlight 는 시험 폴더를 색인하지 않아서 0개 — mdfind 직접 확인 ${SHELLMD}개)"
else
  fail "B4 Mac 전체에서 찾기 (방법: $METHOD, ${FOUND}개, mdfind 직접 ${SHELLMD}개)"
fi
ERR=$(curl -s -m 10 -H "x-token: $(tok)" -H 'content-type: application/json' -X POST -d '{"scope":"all"}' "http://127.0.0.1:$(port)/api/search" | js "d.get('error','')")
check "B5 조건 없이 Mac 전체를 찾으면 안내 ($ERR)" test -n "$ERR"

echo "▶ C. 파일 작업"
SRC="$T/문서/메모.txt"
R=$(post /api/transfer "{\"mode\":\"copy\",\"paths\":[\"$SRC\"],\"dest\":\"$D\"}")
check "C1 복사 → 붙여넣기" test -f "$D/메모.txt" -a -f "$SRC"
post /api/transfer "{\"mode\":\"copy\",\"paths\":[\"$SRC\"],\"dest\":\"$D\"}" >/dev/null
check "C2 같은 이름이면 '이름 2' 로 둘 다 남김" test -f "$D/메모 2.txt"
R=$(post /api/transfer "{\"mode\":\"move\",\"paths\":[\"$T/사진/아이콘.png\"],\"dest\":\"$D\"}")
check "C3 이동" test -f "$D/아이콘.png" -a ! -e "$T/사진/아이콘.png"
ITEMS=$(echo "$R" | js "json.dumps(d['items'], ensure_ascii=False)")
post /api/move-back "{\"items\":$ITEMS}" >/dev/null
check "C4 이동 되돌리기" test -f "$T/사진/아이콘.png" -a ! -e "$D/아이콘.png"
post /api/delete "{\"paths\":[\"$D/메모 2.txt\"]}" >/dev/null
check "C5 즉시 삭제" test ! -e "$D/메모 2.txt"
R=$(post /api/delete "{\"paths\":[\"$HOME/Documents\"]}")
check "C6 맥의 기본 폴더는 지우지 않음" bash -c "[ -d '$HOME/Documents' ] && echo '$R' | grep -q '\"error\":\"'"
R=$(post /api/transfer "{\"mode\":\"copy\",\"paths\":[\"$T/영상\"],\"dest\":\"$D\"}")
check "C7 폴더 통째로 복사" test -f "$D/영상/깊은/폴더/드론.mp4"

echo "▶ D. 미리보기"
CT=$(curl -s -o "$OUT/thumb.png" -w '%{http_code} %{content_type}' -H "x-token: $(tok)" "http://127.0.0.1:$(port)/api/thumb?s=320&path=$(python3 -c "import urllib.parse,sys;print(urllib.parse.quote(sys.argv[1]))" "$T/사진/아이콘.png")")
check "D1 Quick Look 썸네일 ($CT)" test "$CT" = "200 image/png"
CODE=$(curl -s -o /dev/null -w '%{http_code}' -H "x-token: $(tok)" -H 'Range: bytes=0-99' "http://127.0.0.1:$(port)/api/file?path=$(python3 -c "import urllib.parse,sys;print(urllib.parse.quote(sys.argv[1]))" "$T/영상/회의녹화.MOV")")
check "D2 동영상을 구간으로 보내기 (HTTP $CODE)" test "$CODE" = "206"
TXT=$(get "/api/text?path=$(python3 -c "import urllib.parse,sys;print(urllib.parse.quote(sys.argv[1]))" "$SRC")" | js "d['text'].splitlines()[1]")
check "D3 글 미리보기 ($TXT)" test "$TXT" = "안녕하세요"
INFO=$(get "/api/info?path=$(python3 -c "import urllib.parse,sys;print(urllib.parse.quote(sys.argv[1]))" "$T/사진/아이콘.png")")
echo "$INFO" > "$OUT/info-sample.json"
check "D4 자세한 정보 (만든 날짜)" bash -c "echo '$INFO' | grep -q '\"created\"'"

echo "▶ E. Dock 아이콘 다시 누르기(reopen)"
osascript -e 'tell application "FINDEROAD" to reopen' >/dev/null 2>&1; sleep 1
check "E1 다시 열기 뒤에도 앱 · 엔진 그대로" test "$(app_pid)" = "$APID" -a "$(eng_pid)" = "$EPID"

echo "▶ F. 엔진이 고장으로 꺼지면 앱이 다시 켬"
kill -SEGV "$EPID" 2>/dev/null; sleep 1
for _ in $(seq 1 40); do NEWE=$(eng_pid 2>/dev/null); [ -n "$NEWE" ] && [ "$NEWE" != "$EPID" ] && break; sleep 0.5; done
check "F1 엔진 다시 켜짐 ($EPID → ${NEWE:-없음})" test -n "${NEWE:-}" -a "${NEWE:-}" != "$EPID"
check "F2 앱은 그대로" test "$(app_pid)" = "$APID"
check "F3 새 엔진도 앱의 자식" test "$(get /api/ping | js "d['ppid']")" = "$APID"

echo "▶ G. 앱 종료(⌘Q) → 엔진까지 모두 꺼짐"
check "G1 앱 · 엔진 모두 꺼짐" quit_app
snap "G 종료 뒤"

echo "▶ H. 앱이 강제로 꺼져도 엔진이 혼자 남지 않음"
launch; snap "H 실행 중"
kill -9 "$(app_pid)"
check "H1 남은 엔진 없음" bash -c 'for i in $(seq 1 20); do [ -z "$(pgrep -x finderoad-engine)" ] && exit 0; sleep 0.5; done; exit 1'

echo "▶ I. 업데이트 뒤 다시 열기 (/api/update/restart)"
launch; OLD=$(app_pid)
post /api/update/restart '{}' >/dev/null
sleep 3
check "I1 새로 열린 앱이 엔진을 켬" wait_ready
NEW=$(app_pid)
check "I2 앱이 새로 열림 ($OLD → ${NEW:-없음})" test -n "$NEW" -a "$NEW" != "$OLD"
check "I3 앱은 하나만 ($(pgrep -x FINDEROAD | wc -l | tr -d ' ')개)" test "$(pgrep -x FINDEROAD | wc -l | tr -d ' ')" = "1"
quit_app

echo "▶ J. 실제 업데이트 (가짜 새 버전 9.9.9 를 이 맥 안에서 내려받아 설치 → 다시 열기)"
SRV="$OUT/srv"; rm -rf "$SRV" "$OUT/stage"; mkdir -p "$SRV" "$OUT/stage"
ditto "$ROOT/dist/FINDEROAD.app" "$OUT/stage/FINDEROAD.app"
printf '9.9.9' > "$OUT/stage/FINDEROAD.app/Contents/Resources/version.txt"
codesign --force --deep --sign - --timestamp=none "$OUT/stage/FINDEROAD.app" >/dev/null 2>&1
ditto -c -k --norsrc --keepParent "$OUT/stage/FINDEROAD.app" "$SRV/FINDEROAD-9.9.9.zip"
python3 "$ROOT/scripts/feed.py" "$SRV/latest.json" 9.9.9 "$SRV/FINDEROAD-9.9.9.zip" "시험용 새 버전" >/dev/null
( cd "$SRV" && exec python3 -m http.server 18998 --bind 127.0.0.1 >/dev/null 2>&1 ) &
HTTP=$!
for _ in $(seq 1 20); do curl -fs -m 2 http://127.0.0.1:18998/latest.json >/dev/null && break; sleep 0.5; done
curl -s -m 5 -o /dev/null -w 'feed server: HTTP %{http_code}\n' http://127.0.0.1:18998/latest.json | tee -a "$OUT/update-diag.txt"
curl -s -m 5 -o /dev/null -w 'feed server (query): HTTP %{http_code}\n' "http://127.0.0.1:18998/latest.json?ts=1" | tee -a "$OUT/update-diag.txt"
python3 - "$SUP/settings.json" <<'PY'
import json, sys
p = sys.argv[1]; s = json.load(open(p)); s["autoUpdate"] = True; json.dump(s, open(p, "w"))
PY
launch --env FINDEROAD_UPDATE_URL=http://127.0.0.1:18998/latest.json
ST=""
for _ in $(seq 1 60); do ST=$(get /api/update/state | js "d['status']"); [ "$ST" = "ready" ] || [ "$ST" = "error" ] && break; sleep 1; done
get /api/update/state > "$OUT/update-state.json" 2>/dev/null
grep -i "update" "$SUP/engine.log" >> "$OUT/update-diag.txt" 2>/dev/null
check "J1 자동 업데이트로 새 버전을 받아 설치함 (상태: $ST)" test "$ST" = "ready"
if [ "$ST" != "ready" ]; then
  # 진단용: 화면의 '업데이트 확인' → '지금 업데이트' 와 같은 수동 경로
  post /api/update/check '{}' >> "$OUT/update-diag.txt" 2>&1; echo >> "$OUT/update-diag.txt"
  post /api/update/install '{}' >/dev/null
  for _ in $(seq 1 60); do ST=$(get /api/update/state | js "d['status']"); [ "$ST" = "ready" ] || [ "$ST" = "error" ] && break; sleep 1; done
  check "J1b 수동 업데이트로 설치함 (상태: $ST)" test "$ST" = "ready"
  grep -i "update" "$SUP/engine.log" >> "$OUT/update-diag.txt" 2>/dev/null
fi
OLD=$(app_pid)
post /api/update/restart '{}' >/dev/null
sleep 3
check "J2 새 버전이 다시 열림" wait_ready
check "J3 응용 프로그램 폴더의 앱이 9.9.9 ($(cat "$APPDIR/Contents/Resources/version.txt"))" test "$(cat "$APPDIR/Contents/Resources/version.txt")" = "9.9.9"
check "J4 이전 버전은 백업에 보관" test -f "$SUP/backup/FINDEROAD.app/Contents/Resources/version.txt"
check "J5 업데이트한 앱의 서명이 그대로" codesign --verify --deep --strict "$APPDIR"
quit_app
kill "$HTTP" 2>/dev/null
rm -rf "$APPDIR"; ditto "$ROOT/dist/FINDEROAD.app" "$APPDIR"

echo "▶ 기록 모으기"
for f in launcher.log launcher.prev.log engine.log engine.prev.log engine.out engine.prev.out settings.json; do
  [ -f "$SUP/$f" ] && cp "$SUP/$f" "$OUT/$f"
done
rm -rf "$OUT/srv" "$OUT/stage"
check "Z1 끝나고 남은 프로세스 없음" test -z "$(pgrep -x FINDEROAD)$(pgrep -x finderoad-engine)"
sw_vers > "$OUT/macos.txt"
echo
echo "== 결과 =="; cat "$RES"
! grep -q '^FAIL' "$RES"
