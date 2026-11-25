> **이 파일을 읽은 Claude는 작업이 끝날 때마다 이 파일을 업데이트할 것.**
> (구조가 바뀌면 「구조」, 새로 정한 것은 「결정 사항」, 한 일·남은 일은 「진행 사항」에 적고 커밋·푸시까지 할 것)

# FINDEROAD — 프로젝트 노트

파일 검색 · 미리보기 · 복사 · 붙여넣기 · 이동 · 삭제 macOS 앱 (Apple Silicon). made by. Nevertheless_D
사용자는 개발자가 아니에요. 한국어로 쉽게 설명하고, 터미널 사용을 요구하지 말 것. 결과물은 더블클릭으로 쓸 수 있게.
참고한 구조: 같은 사용자의 MSROAD 저장소(1.1.0~ 의 Dock 앱 구조 · 맥 빌드 서버 · 자동 업데이트).

---

## 진행 사항

- **2026-11-25 v2.0.1 '그만 찾기'가 안 되던 문제 고침 (사용자 보고)**
  - 원인: 카드 보기가 결과를 다시 그릴 때마다(0.3~0.7초) 최대 400개 썸네일 `<img src=/api/thumb>` 를 새로 만들었고, 맥 Quick Look 은 2개씩만 만들어서
    요청이 쌓여 WebKit 의 같은 주소 연결을 다 차지 → `/api/search/stop` · poll 이 그 뒤에 줄을 섬(리눅스 재현: 멈추는 데 6.9초 → 고친 뒤 0.3초).
  - 고침(`app.js` `thumbs`): 찾는 동안엔 썸네일을 부르지 않음, 화면에 보이는 카드만(IntersectionObserver), 한 번에 2개, 새 검색이면 AbortController 로 취소,
    받은 그림은 경로별로 기억(blob URL, 600개)해서 다시 그려도 재요청 없음. 카드 그림은 index.html 의 `<symbol id="art-…">` 를 `<use>` 로(행마다 SVG 복사 안 함).
  - 엔진 시험용 `FINDEROAD_TEST_THUMB_MS`(리눅스에서 느린 썸네일 흉내, 2개씩).
  - 전체 기능 재점검(Playwright): 보낼 곳 추가 · 복사/붙여넣기/이동/휴지통 + ⌘Z 되돌리기 · 미리보기 · 오른쪽 메뉴 · 테마 · 업데이트 창 · 확장자 고르기 ·
    폴더 추가 · 범위 창 · 단축키 창 · ⌫ 이전 검색 · 카드 방향키 · 스크롤한 자리 썸네일 → 모두 통과. 상태 문구의 '을(를)'은 받침에 맞게(`eul()`).
- **2026-11-25 v2.0.0 새 디자인 "초록 서랍" · 이름 FindeROAD (사용자 요청, 디자인 캔버스 "FINDEROAD 2.0 디자인 시안"의 E 확정)**
  - 도로 느낌(밤의 고속도로 · 차선 점선 · 호박색)을 모두 빼고 "Finder 를 대신하는 파일 찾기 앱"이 주인공이 되게. YTROAD 의 형제처럼
    색 하나(초록 #0B8A5F / 다크 #34D399), 아주 큰 제목, 그림, 큼직한 선택 카드. 글꼴 **Pretendard**(본문, OFL) + **Unbounded 800**(로고, OFL),
    `engine/web/fonts/fonts.css` 하나로. Gothic A1 · Orbit · IBM Plex Mono 는 뺌.
  - 화면 이름은 **FindeROAD**(창 제목 · 메뉴 · Info.plist CFBundleName/DisplayName). 앱 파일 · 실행 파일 · 지원 폴더 · 번들 ID · 업데이트 주소는
    그대로 FINDEROAD (업데이트 · 시험 · AppleScript 가 그대로 돌게).
  - 아이콘: 초록 바탕, 열린 서랍에서 파일 카드 한 장이 나온 그림(app/AppIcon.svg → scripts/icon.py).
  - 화면 두 상태: **첫 화면**(body.mode-start: 서랍 그림 + "찾고 싶은 파일, 이름만 적어 보세요" + 큰 검색 칸 + "어디서 찾을까요?" 카드 +
    "무엇을 찾을까요?" 분류 4×2 칸, 8번째 칸 = 직접 고르기) ↔ **결과 화면**(body.mode-results: 얇은 검색 막대 · 분류 알약 · 진행 표시 ·
    결과 · 아래 "여기로 끌어다 놓기"(보낼 곳) · 오른쪽 미리보기). 검색이 시작되면 결과 화면, 결과를 지우면(검색어 비움 등) 첫 화면.
    같은 DOM(id 그대로)을 CSS 로 옮기는 방식이라 기존 기능 · 맥 자체 시험의 선택자가 그대로 맞아요.
  - 결과 **카드 / 목록** 보기(설정 ui.view 에 기억). 카드: 종류별 그림 + 그림 · 동영상 · 문서는 Quick Look 썸네일(/api/thumb, 보일 때만),
    ← → 한 칸, ↑ ↓ 한 줄. 카드에서는 '정렬 ▾' 메뉴, 목록에서는 표 머리로 정렬.
  - 검색 범위는 결과 화면에서 검색 막대의 '범위 ▾' 아래 떠 있는 카드(같은 #scopeCard). 왼쪽 막대 · 사이드바 단축키 목록은 없어지고
    맨 위 **?** 단추 = 단축키 창(12개), 맨 위 **‹** 단추 = 이전 검색(⌫ 와 같음).
  - **⌫ 규칙 보완**(road-mac-app 두 갈래 규칙): 지나온 검색이 없을 때 ⌫ = 첫 화면으로(검색어 · 확장자 비움). 첫 화면에서는 아무 일도 안 함.
    범위 창 · 단축키 창 · 확장자 창 · 메뉴 · 확인 창이 열려 있으면 ⌫ 는 건드리지 않음.
  - 창 이름이 FindeROAD 로 바뀌어 맥 시험 M1(scripts/drag-test.swift)이 창을 못 찾던 것 → 앱 이름을 대소문자 상관없이 비교.
  - 맥 빌드 서버 시험 55개 모두 통과 → 2.0.0 업데이트로 내보냄 (releases/latest.json).
  - 결과가 없을 때 같은 말이 두 번 나오던 것 고침(아래 줄은 '찾기를 마쳤어요 · n초').
  - 가이드 이미지 · guide-screen.png 를 새 디자인으로 다시 만듦. 리눅스 Playwright 로 첫 화면 · 결과(카드 · 목록) · 범위 창 · 확장자 창 ·
    단축키 창 · 결과 없음 · 우클릭 메뉴 · 즉시 삭제 확인 · 가장 작은 창(1040×640) · 라이트/다크 확인, ⌫ · 방향키 · 정렬 · 보기 기억 시험 통과.

- **2026-11-23 v1.1.0 새 디자인 · 확장자 고르기 · 끌어서 옮기기 (사용자 요청, 시안 B 확정)**
  - 디자인: 디자인 캔버스 "FINDEROAD 1.1 디자인 시안"의 **B · 밤의 고속도로**. 다크 #0D1016 바탕 + 호박색 #FFB020(라이트는 #B45309) · 주황 #FF7A1A
    그라데이션(진행 막대), 위쪽 막대 아래 차선 점선, 왼쪽 막대 경계도 점선. 글꼴 **Gothic A1**(본문, OFL) + **Orbit**(로고 · 제목, OFL) + IBM Plex Mono(숫자 · 경로),
    `engine/web/fonts/ga1 · orbit` 에 내장(fontsource npm 패키지에서 woff2). IBM Plex Sans KR 은 뺌. 종류별 색 아이콘(문서 파랑 · 동영상 보라 · 이미지 청록 ·
    음악 분홍 · 압축 호박 · 코드 회색), '이름 일치' 표시(정확도 1000 이상).
  - 확장자: 칸이 조각(토큰) 칸으로 — 쉼표 · 띄어쓰기 · Enter 로 조각, 빈 칸에서 ⌫ 는 마지막 조각 빼기. **고르기** 창(종류별 체크 · 모두 고르기 · 직접 넣기 ·
    이걸로 찾기). 분류 버튼은 여러 개 함께 켜짐(일부만 켜지면 2/16 처럼 표시, aria-pressed=mixed).
  - 보낼 곳: 설정 `dests`. 왼쪽에 목록 · 추가 · 빼기, 누르면 '고른 파일을 여기로 이동 · 복사 · Finder에서 열기' 메뉴. Finder 에서 폴더를 그 자리에 끌어다 놓아도 추가.
  - 끌어서 옮기기: 화면이 줄을 누른 채 5px 움직이면 `dragFiles` → Swift `FRWebView.startFileDrag`(마지막 마우스 움직임을 NSEvent 로컬 감시로 기억해 두었다가
    `beginDraggingSession`). Finder · 데스크탑 · 다른 앱에 놓으면 macOS 규칙(같은 디스크 이동 · 다른 디스크 복사 · ⌥ 복사), 끝나면 `/api/exists` 로 사라진 줄을 뺌.
    이 창의 '보낼 곳'이나 결과 속 폴더에 놓으면 `FR.dragOver · dragDrop` → `sendTo(move|copy)`(⌥ = 복사, 되돌리기 됨).
  - 진행 표시: 검색 칸 아래 막대(Spotlight ✓ · 폴더 직접 확인 중 · 확인한 폴더 수 · 지금 보는 폴더 · 그만 찾기 Esc).
  - 사용 가이드 이미지도 밤의 고속도로 분위기로 다시 만듦(guide-screen.png 는 리눅스 Playwright 다크 화면).
  - 맥 시험 N1 추가: 자체 시험이 첫 줄 · 첫 보낼 곳의 화면 위치를 남기고, `scripts/drag-file-test.swift` 가 ⌥ 를 누른 채 끌어다 놓아 복사되는지 확인 → 통과.
- **2026-11-23 v1.0.3 검색 정확도 (사용자 요청)**: ① 'Mac 전체'에서 '선택한 폴더'로는 나오는 파일이 빠짐 ② 'hwp, hwpx' 를 함께 넣으면 한쪽이 안 나옴.
  원인: Mac 전체는 Spotlight + 홈 폴더를 **한 줄로 8초만** 훑었고(큰 홈이면 깊은 곳까지 못 감), 확장자만으로 찾을 때는 그마저 안 했어요 → Spotlight 가 모르는
  파일(낯선 확장자 hwpx, 색인에서 빠진 폴더)은 영영 안 나왔어요. 고친 것: 어느 범위든 **Spotlight 답을 먼저 보여 주고, 동시에 폴더를 끝까지 직접 훑음**
  (`engine/walker.go`: 일꾼 6~16명이 나눠 훑기, 자주 쓰는 곳 먼저 — 데스크탑 · 문서 · 다운로드 · iCloud Drive · 클라우드 폴더 · 사진/동영상/음악 → 홈 나머지
  → /Users/Shared → 응용 프로그램 → 외장 디스크(네트워크 · 타임머신 디스크 제외), 시간 Mac 전체 2분 · 선택한 폴더 3분). 화면은 `/api/search/start` →
  `/api/search/poll` 로 찾는 중에도 결과를 계속 받고, 아래 막대에 '그만 찾기'(Esc). **정확도순**: 이름으로 찾으면 이름이 검색어와 똑같은 파일 → 검색어로 시작
  → 단어 첫머리 → 가운데 순(같으면 짧은 이름 · 최근 수정), 표 머리를 누르면 그 정렬로. node_modules 같은 개발 폴더는 숨김 파일 켤 때만.
  **공통 기능(road-mac-app)도 함께**: ⌫ / ⌘[ = 이전 검색으로(입력 중 한 글자씩 바뀐 검색은 하나로 묶음, 글 칸 · 한글 조합 중 · 떠 있는 창에서는 안 함),
  맥 시험 M1(창이 앞에 있을 때 맨 위를 끌어 옮기기, `scripts/drag-test.swift`). FINDEROAD 는 처음부터 DragView(화면이 알려 준 빈 곳에 깔림)라서 창 옮기기는 이미 됨.
  맥 시험 추가: B6(폴더에서 hwp, hwpx) · B7(Mac 전체에서 hwp, hwpx) · B8(Mac 전체에서 깊은 폴더) · B9(그만 찾기).
  빌드 서버에서 찾은 버그: Spotlight 는 /private/var/… , 직접 훑기는 /var/… 로 같은 파일을 줘서 두 번 나옴 → dedupeKey · mdfind 경로 되돌리기. 맥 시험 54개 통과 → 출시.
  디자인 시안: 디자인 캔버스 "FINDEROAD 1.1 디자인 시안"(A 오렌지 길 · B 밤의 고속도로 · C 도로 표지판) → 사용자가 B 로 확정 → 1.1.0.
- **2026-10-31 v1.0.1 이름 일부로 찾기 (사용자 요청)**: '호랑이'로 '호랑이님 증명사진'이 안 나오던 문제. 엔진의 이름 비교는 원래 '포함'이었지만
  Mac 전체 검색은 Spotlight 답에만 기대서 Spotlight 가 놓치면 끝이었음. 고친 것: ① Spotlight 질의를 여러 모양으로(FSName · DisplayName, NFC · NFD,
  'cd' · 'c') ② 이름으로 찾을 때는 Spotlight 와 함께 홈 폴더를 8초 동안 직접 훑어 빠진 파일을 채움(선택한 폴더도 Spotlight 가 찾은 뒤 8초 더 훑음)
  ③ 띄어쓰기 · 밑줄 · 하이픈은 있든 없든 같게('호랑이님증명사진' = '호랑이님 증명사진'). 시험: Go TestPartialHangulMatch, 맥 B2+ · B2*. Spotlight · 직접 훑기가 같은 파일을 NFC/NFD 로 다르게 줘서 생기던 중복은 경로를 합친 모양으로 비교해 없앰.
  맥 빌드 서버 48개 시험 모두 통과 → 릴리스.
- **2026-10-29 내려받기용 zip (사용자 요청)**: 저장소 `download/` 폴더에 `FINDEROAD.zip`(앱 + 사용 가이드)를 올리고, README 맨 위 '앱 받기' 링크(`raw/main/download/FINDEROAD.zip`)로 바로 받게 했어요. 예전 버전 zip 은 지웠어요 (저장소 `releases/` 에도 업데이트용 최신판만).
작업이 중간에 끊기면 아래 체크리스트에서 **처음으로 체크 안 된 항목부터** 이어서 하면 돼요.
각 단계가 끝날 때마다 커밋 · 푸시해 둡니다.

- [x] 0. 디자인 시안 확정 (디자인 캔버스 "FINDEROAD UI": 라이트/다크 화면, 앱 아이콘 · 워드마크)
- [x] 1. 저장소 뼈대 (이 파일, CLAUDE.md, VERSION, .gitignore, README)
- [x] 2. 엔진: 검색(Spotlight · 폴더 직접) · 파일 작업 · 미리보기 · 설정 · 업데이트
- [x] 3. 화면 (web UI)
- [x] 4. Swift 앱 껍데기 · Info.plist · 아이콘
- [x] 5. 빌드 · 시험 스크립트, 맥 빌드 서버(워크플로)
- [x] 6. 리눅스에서 Go 테스트 + Playwright 화면 시험
- [x] 7. 사용 가이드 이미지 (docs/FINDEROAD_사용가이드.png)
- [x] 8. 맥 빌드 서버 결과 확인 → v1.0.0 릴리스 (releases/latest.json) — 실제 맥 시험 43개 모두 통과
- [x] 9. 설치 파일 · 가이드 이미지 전달

### 다음에 할 수 있는 일
- [ ] 사용자 맥에서 v1.0.0 실행 확인 (특히 Mac 전체 검색 — 빌드 서버의 Spotlight 는 새 파일을 색인하지 않아 실제 Spotlight 결과는 시험하지 못함)
- [ ] (사용자가 원하면) Apple 개발자 서명 · 공증 → 처음 열 때 경고 없애기

### 작업 기록
- 2026-11-02 v1.0.2 — 사용자 제보(CUTROAD 에서): Spotlight 에 같은 앱이 두 개(응용 프로그램 + 백업 폴더) 보임. 업데이트 때 이전 버전을 보관하는
  폴더를 `Support/backup.noindex/FINDEROAD.app`(.noindex = Spotlight 가 안 찾음)로 바꾸고, 바꾸기 · 되돌리기 뒤 `lsregister -u` 로 보관본을 macOS 목록에서 지움.
  엔진이 켜질 때 예전 `Support/backup/FINDEROAD.app` 이 있으면 새 폴더로 옮김(migrateBackup). 맥 시험 A1b 추가. Windows 경로는 그대로.
- 2026-10-23 **v1.0.0 릴리스.** 맥 빌드 서버(macOS 15.7, Apple Silicon)에서 43개 시험 통과: 켜기 · 엔진이 앱의 자식 · 자체 시험(위쪽 막대 클릭,
  ⌘C 가 화면까지 감, 휴지통, 클립보드, Quick Look, 신호등 위치, 끌기 영역, 동영상 분류 검색) · 검색(확장자만, NFD 한글 이름, 숨김) ·
  파일 작업(복사 · 같은 이름 · 이동 · 되돌리기 · 즉시 삭제 · 기본 폴더 보호 · 폴더 복사) · 미리보기(썸네일 · 구간 전송 · 글 · 정보) ·
  다시 열기 · 엔진 고장 복구 · ⌘Q · 앱 강제 종료 시 엔진도 끝남 · 업데이트 재시작 · **실제 업데이트(가짜 9.9.9 를 받아 설치 → 다시 열기 → 백업 · 서명 유지)**.
  업데이트 시험이 한 번 실패했던 건 시험용 서버가 아직 안 켜졌던 것 → 시험 스크립트가 서버 준비를 기다리게 함. 업데이트 확인 실패 원인은 이제 engine.log 에 남음.
- 2026-10-23 맥 빌드 서버 1차 결과: Swift 빌드 통과, 자체 시험(위쪽 막대 클릭 · ⌘C 전달 · 휴지통 · 클립보드 · Quick Look · 신호등 위치 · 끌기 영역) 모두 통과.
  고친 것: macOS 기본 bash 3.2 는 `$N개` 처럼 변수 뒤에 한글이 붙으면 변수 이름으로 읽음 → 스크립트에서 `${N}개` 로.
  좁은 창에서 작업 버튼이 두 줄이 되던 것 → 1380px 아래는 단축키 숨김, 1180px 아래는 아이콘만. 재생 못 하는 동영상은 4초 뒤 썸네일로.
- 2026-10-23 시작. 디자인 확정 후 개발 착수.

## 구조

```
app/shell/main.swift   FINDEROAD.app 본체(Swift/AppKit): Dock 아이콘 · 메뉴 · WKWebView 창 · 엔진(자식) 실행/종료 ·
                       네이티브 기능 다리(Quick Look · 휴지통 · 클립보드 · 폴더 고르기 · 폴더 끌어다 놓기 · 화면 모드)
app/Info.plist         Dock 에 보임, 127.0.0.1 만 허용
app/AppIcon.svg/png/icns
engine/                Go 엔진(표준 라이브러리만). 화면(web/)을 내장. 앱이 켜져 있는 동안만 앱의 자식으로 돎
  main.go              라우트 · 토큰 · 설정 · 수명(앱이 사라지면 종료)
  search.go            검색 조건(이름 · 확장자 · 숨김) · 정확도 점수 · Spotlight(mdfind) · 결과 모으기
  walker.go            폴더 직접 훑기(여러 일꾼 · 자주 쓰는 곳 먼저 · Mac 전체에서 훑을 곳 macRoots)
  searchjob.go         검색 요청: /api/search(끝까지 기다림, 시험용) · /api/search/start · poll · stop(화면)
  fileops.go           복사 · 이동 · 휴지통(대체) · 즉시 삭제 · 되돌리기
  preview.go           파일 내보내기(동영상 · 음악 · 이미지), Quick Look 썸네일(qlmanage), 글 미리보기
  hangul.go            한글 NFC/NFD 변환
  update.go            자동 업데이트(releases/latest.json)
  web/                 index.html · css · js · fonts(Pretendard · Unbounded, fonts.css)
scripts/               build-mac.sh · smoke-mac.sh · ci-mac.sh · release.sh · feed.py · guide.py · icon.py
releases/              FINDEROAD-x.y.z.zip · latest.json
docs/                  사용 가이드 이미지와 원본(guide.html · guide-screen.png). 고친 뒤 scripts/guide.py
.github/workflows/build-mac.yml   맥 빌드 서버 (app/ · engine/ · scripts/ · VERSION · 가이드 이미지가 바뀌면 자동 실행)
```
실행 사슬: FINDEROAD.app(Swift, Dock) → finderoad-engine(앱 묶음 안의 Go, 자식 하나). 주소는 127.0.0.1 의 빈 포트(엔진이 표준 출력으로
`FINDEROAD_PORT=` 한 줄을 알려 줌) + 실행마다 새 토큰. 엔진은 표준 입력이 닫히거나(앱이 사라짐) 부모가 바뀌면 끝나고, 종료 신호는 언제나 따름.
앱은 엔진이 고장으로 꺼지면 2분에 3번까지 다시 켬. 엔진이 코드 0 으로 끝나면 앱도 끝남(restart-requested 가 있으면 새 버전으로 다시 열기).

화면 ↔ 맥 기능: `window.webkit.messageHandlers.fr.postMessage({cmd, …})` (답을 돌려받는 방식). cmd: ready · appearance · dragRegions ·
chooseFolders · trash · pbWriteFiles · pbReadFiles · pbWriteText · quicklook. 맥 앱이 화면에 알리는 것: `FR.drop(paths)` · `FR.dragging(on)` · `FR.fullscreen(on)`.
브라우저(시험)에서는 같은 일을 엔진의 대체 API(/api/clipboard · /api/trash · /api/choose-folder)가 해요.
창: 제목 막대를 투명하게 하고 화면을 끝까지 채움(fullSizeContentView). 신호등 버튼은 52pt 위쪽 막대 가운데로 옮기고,
화면이 알려 준 빈 곳(로고 · 가운데 빈칸)에 투명한 DragView 를 깔아 창을 끌 수 있게 함.

### 시험하는 법
- 리눅스: `cd engine && go test ./...`, 엔진을 `FINDEROAD_SUPPORT · FINDEROAD_TOKEN · FINDEROAD_PORT · FINDEROAD_TEST_CHOOSE(대상 폴더) ·
  FINDEROAD_UPDATE_URL` 로 켜고 Playwright 로 http://127.0.0.1:포트/?t=토큰 을 열어 시험 (Quick Look · 휴지통은 대체 API).
- 맥: 맥 빌드 서버(scripts/smoke-mac.sh). 앱을 `FINDEROAD_SELFTEST=1` 로 켜면 Swift 가 스스로 맥 기능을 시험하고
  Support/selftest.json · shot-light.png · shot-dark.png 를 남김.
- 빌드 서버의 Spotlight 는 새로 만든 파일을 색인하지 않아요 → Mac 전체 검색은 'Spotlight 에 물어봤는지'까지만 확인.

## 결정 사항

- **디자인(2.0, 초록 서랍)**: 색 · 글꼴 · 간격 · 모서리는 `engine/web/css/app.css` 맨 위 변수에서만. 초록 #0B8A5F(라이트) / #34D399(다크), 바탕 #F3F7F4 / #0D110F, 카드 #FFFFFF / #151B17. 글자 12 · 13 · 15 · 18(+ 첫 화면 제목 42), 모서리 10 · 16 · 24 + 알약. 종류 색: 동영상 빨강 · 이미지 초록 · 문서 파랑 · 음악 보라 · 압축 주황 · 코드 회색. 앱 이름 표기는 **FindeROAD**. 시안 원본: 디자인 캔버스 "FINDEROAD 2.0 디자인 시안"(2차 페이지 E).
- **GitHub 에는 늘 최신판 zip 1개만**: 업데이트를 내보낼 때마다 `scripts/publish-download.sh` 가 앱 + 가이드 zip 을 `download/` 에 정해진 이름으로 바꿔 넣고 다른 파일은 지워요(release.sh 가 자동으로 불러요. GitHub 릴리스 페이지는 이 작업 환경에서 만들 수 없어 저장소 폴더를 써요). `releases/` 의 업데이트용 zip 도 현재 버전만 남겨요 — 설치된 앱은 latest.json 이 가리키는 zip 만 받고, '되돌리기'는 맥 안의 백업을 써서 예전 zip 이 필요 없어요.
- 방식: MSROAD 1.1.0 과 같은 **Dock 에 보이는 정식 맥 앱**. 본체는 Swift 껍데기, 화면은 앱 자체 창(WKWebView), Chrome 안 씀.
  엔진(Go)은 앱의 **자식으로만**, 앱이 켜져 있는 동안만 돈다. ⌘Q 하면 엔진도 끈다. 앱이 강제로 꺼져도 엔진은 스스로 끝난다.
  숨은 도우미 앱 · 종료 신호 무시 · 실행기가 엔진만 남기고 먼저 끝나는 방식은 쓰지 않는다 (macOS '백그라운드 앱 활동' 규칙을 따름).
- Apple Silicon 전용(arm64), macOS 12 이상.
- 빌드 · 서명 · 시험은 **맥 빌드 서버**(.github/workflows/build-mac.yml, macos-15)에서 실제 맥으로 하고, 통과한 결과만 릴리스한다.
- 업데이트: 이 저장소의 `releases/latest.json` 을 켤 때 + 6시간마다 확인, 자동 설치(기본 켬). 수동 '업데이트 확인' 버튼, 버전 표시.
- 사용자에게 보이는 곳(앱 화면 · 가이드 이미지)에는 GitHub 나 저장소 이름을 쓰지 않는다 (MSROAD 와 같은 원칙).
- 화면 모드: 시스템 설정 따르기(기본) / 라이트 / 다크.
- 화면 아래 `made by. Nevertheless_D`.
- 디자인(1.1.0~): **B · 밤의 고속도로** — 다크 #0D1016 / 라이트 #EEF1F6 바탕, 포인트 호박색(다크 #FFB020, 라이트 #B45309), 차선 점선 장식. 글꼴 Gothic A1(본문) + Orbit(로고 · 제목) + IBM Plex Mono, 앱에 내장. (1.0.x 는 무채색 + 주황 · IBM Plex Sans KR)
  아이콘: 짙은/밝은 주황으로 나뉜 정사각형(경계선 = 굽은 길), 왼쪽 눈, 오른쪽 눈에 돋보기. 워드마크 FINDE + R(반반) + OAD.
- 검색어는 이름 어디에든 들어 있으면 찾음(앞 · 가운데 · 뒤). 띄어쓰기 · _ · - 는 무시하고도 비교. Spotlight 를 믿지 않고 **언제나 끝까지 직접 훑기**(1.0.3~). 이름으로 찾으면 기본 정렬은 정확도순.
- 검색: 이름 칸과 확장자 칸이 따로. 이름을 비우고 확장자만으로도 검색. 확장자는 조각(토큰)으로 여러 개, '고르기' 창에서 체크. 분류 버튼은 여러 개 함께 켜짐(누르면 그 분류 확장자 더하기 · 다시 누르면 빼기).
- 보낼 곳 · 끌어서 옮기기: 결과를 끌어서 Finder · 데스크탑 · 보낼 곳 · 결과 속 폴더에 놓기. 앱 안에 놓으면 이동(⌥ 복사), 되돌리기 됨.
  범위: Mac 전체 또는 선택한 폴더(여러 개, 하위 폴더 포함). 초성 검색은 하지 않음(사용자 결정).
- 삭제: 기본은 휴지통으로(⌘⌫, ⌘Z 로 되돌리기). **즉시 삭제**(⌘⌥⌫)는 확인 창을 거친 뒤 되돌릴 수 없게 지움.
- 붙여넣기 · 이동: 검색 결과에는 '현재 폴더'가 없으므로 대상 폴더를 고르는 창을 띄운다. 같은 이름이 있으면 '파일 2.mp4' 처럼 둘 다 남긴다.
- 즉시 삭제 · 이동 · 휴지통은 시스템 폴더 · 디스크 맨 위 · 홈 폴더와 그 기본 폴더(문서 · 데스크탑 등) 자체를 막음 (엔진 protectedPath · Swift 같은 규칙).
- Mac 전체 검색은 숨김 · 시스템 · ~/Library 를 빼되 iCloud Drive(~/Library/Mobile Documents) · 클라우드 폴더(~/Library/CloudStorage)는 넣음.
  앱 · 사진 보관함 같은 묶음 폴더는 하나의 항목으로만 보고 안을 들여다보지 않음. 결과는 최대 150,000개를 모아 정확도 → 최근 수정 순 2,000개만 보여 줌.
- 되돌리기는 마지막 작업 하나(붙여넣기 → 붙여넣은 것을 휴지통으로, 이동 · 휴지통 → 원래 자리로).

## 업데이트 내보내는 법 (Claude용)

1. 수정 → `VERSION` 올리기 → 커밋 · 푸시 (app/ · engine/ · scripts/ · VERSION 이 바뀌면 맥 빌드 서버가 자동으로 돎, 5~10분)
2. `git fetch origin +refs/heads/ci-results:refs/remotes/origin/ci-results` 로 결과가 새 커밋인지 확인 →
   `git archive origin/ci-results | tar -x -C <임시폴더>` → status.json(build/smoke 모두 ok), results.txt, shot-*.png 확인
3. `scripts/release.sh --notes "바뀐 점 1" "바뀐 점 2"` (사용자가 읽는 한국어 문구, GitHub 언급 없이) → releases/ 커밋 · 푸시.
   처음 설치용은 dist/FINDEROAD.zip (가이드 이미지 포함). releases/ 에는 최신 zip 3개만 남김.

- 커밋 작성자: `Nevertheless_D <NeverthelessD@users.noreply.github.com>`
