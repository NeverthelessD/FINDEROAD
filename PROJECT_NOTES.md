> **이 파일을 읽은 Claude는 작업이 끝날 때마다 이 파일을 업데이트할 것.**
> (구조가 바뀌면 「구조」, 새로 정한 것은 「결정 사항」, 한 일·남은 일은 「진행 사항」에 적고 커밋·푸시까지 할 것)

# FINDEROAD — 프로젝트 노트

파일 검색 · 미리보기 · 복사 · 붙여넣기 · 이동 · 삭제 macOS 앱 (Apple Silicon). made by. Nevertheless_D
사용자는 개발자가 아니에요. 한국어로 쉽게 설명하고, 터미널 사용을 요구하지 말 것. 결과물은 더블클릭으로 쓸 수 있게.
참고한 구조: 같은 사용자의 MSROAD 저장소(1.1.0~ 의 Dock 앱 구조 · 맥 빌드 서버 · 자동 업데이트).

---

## 진행 사항

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
  search.go            검색: Spotlight(mdfind) 또는 폴더 직접 훑기, 이름 · 확장자 조건, 정렬
  fileops.go           복사 · 이동 · 휴지통(대체) · 즉시 삭제 · 되돌리기
  preview.go           파일 내보내기(동영상 · 음악 · 이미지), Quick Look 썸네일(qlmanage), 글 미리보기
  hangul.go            한글 NFC/NFD 변환
  update.go            자동 업데이트(releases/latest.json)
  web/                 index.html · css · js · fonts(IBM Plex Sans KR · IBM Plex Mono)
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
- 디자인: 무채색 바탕 + 주황 포인트(라이트 #C2410C, 다크 #FB923C). 글꼴 IBM Plex Sans KR(한글 UI) + IBM Plex Mono(확장자 · 경로 · 크기 · 버전), 앱에 내장.
  아이콘: 짙은/밝은 주황으로 나뉜 정사각형(경계선 = 굽은 길), 왼쪽 눈, 오른쪽 눈에 돋보기. 워드마크 FINDE + R(반반) + OAD.
- 검색어는 이름 어디에든 들어 있으면 찾음(앞 · 가운데 · 뒤). 띄어쓰기 · _ · - 는 무시하고도 비교. Spotlight 를 믿지 않고 직접 훑기로 보충(8초).
- 검색: 이름 칸과 확장자 칸이 따로. 이름을 비우고 확장자만으로도 검색. 분류 버튼(전체 · 동영상 · 이미지 · 문서 · 음악 · 압축 · 코드)을 누르면 확장자 칸이 채워짐.
  범위: Mac 전체 또는 선택한 폴더(여러 개, 하위 폴더 포함). 초성 검색은 하지 않음(사용자 결정).
- 삭제: 기본은 휴지통으로(⌘⌫, ⌘Z 로 되돌리기). **즉시 삭제**(⌘⌥⌫)는 확인 창을 거친 뒤 되돌릴 수 없게 지움.
- 붙여넣기 · 이동: 검색 결과에는 '현재 폴더'가 없으므로 대상 폴더를 고르는 창을 띄운다. 같은 이름이 있으면 '파일 2.mp4' 처럼 둘 다 남긴다.
- 즉시 삭제 · 이동 · 휴지통은 시스템 폴더 · 디스크 맨 위 · 홈 폴더와 그 기본 폴더(문서 · 데스크탑 등) 자체를 막음 (엔진 protectedPath · Swift 같은 규칙).
- Mac 전체 검색은 숨김 · 시스템 · ~/Library 를 빼되 iCloud Drive(~/Library/Mobile Documents) · 클라우드 폴더(~/Library/CloudStorage)는 넣음.
  앱 · 사진 보관함 같은 묶음 폴더는 하나의 항목으로만 보고 안을 들여다보지 않음. 결과는 최대 30,000개를 모아 최근 수정 순 2,000개만 보여 줌.
- 되돌리기는 마지막 작업 하나(붙여넣기 → 붙여넣은 것을 휴지통으로, 이동 · 휴지통 → 원래 자리로).

## 업데이트 내보내는 법 (Claude용)

1. 수정 → `VERSION` 올리기 → 커밋 · 푸시 (app/ · engine/ · scripts/ · VERSION 이 바뀌면 맥 빌드 서버가 자동으로 돎, 5~10분)
2. `git fetch origin +refs/heads/ci-results:refs/remotes/origin/ci-results` 로 결과가 새 커밋인지 확인 →
   `git archive origin/ci-results | tar -x -C <임시폴더>` → status.json(build/smoke 모두 ok), results.txt, shot-*.png 확인
3. `scripts/release.sh --notes "바뀐 점 1" "바뀐 점 2"` (사용자가 읽는 한국어 문구, GitHub 언급 없이) → releases/ 커밋 · 푸시.
   처음 설치용은 dist/FINDEROAD.zip (가이드 이미지 포함). releases/ 에는 최신 zip 3개만 남김.

- 커밋 작성자: `Nevertheless_D <NeverthelessD@users.noreply.github.com>`
