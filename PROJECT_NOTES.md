> **이 파일을 읽은 Claude는 작업이 끝날 때마다 이 파일을 업데이트할 것.**
> (구조가 바뀌면 「구조」, 새로 정한 것은 「결정 사항」, 한 일·남은 일은 「진행 사항」에 적고 커밋·푸시까지 할 것)

# FINDEROAD — 프로젝트 노트

파일 검색 · 미리보기 · 복사 · 붙여넣기 · 이동 · 삭제 macOS 앱 (Apple Silicon). made by. Nevertheless_D
사용자는 개발자가 아니에요. 한국어로 쉽게 설명하고, 터미널 사용을 요구하지 말 것. 결과물은 더블클릭으로 쓸 수 있게.
참고한 구조: 같은 사용자의 MSROAD 저장소(1.1.0~ 의 Dock 앱 구조 · 맥 빌드 서버 · 자동 업데이트).

---

## 진행 사항

작업이 중간에 끊기면 아래 체크리스트에서 **처음으로 체크 안 된 항목부터** 이어서 하면 돼요.
각 단계가 끝날 때마다 커밋 · 푸시해 둡니다.

- [x] 0. 디자인 시안 확정 (디자인 캔버스 "FINDEROAD UI": 라이트/다크 화면, 앱 아이콘 · 워드마크)
- [x] 1. 저장소 뼈대 (이 파일, CLAUDE.md, VERSION, .gitignore, README)
- [x] 2. 엔진: 검색(Spotlight · 폴더 직접) · 파일 작업 · 미리보기 · 설정 · 업데이트
- [x] 3. 화면 (web UI)
- [x] 4. Swift 앱 껍데기 · Info.plist · 아이콘
- [x] 5. 빌드 · 시험 스크립트, 맥 빌드 서버(워크플로)
- [ ] 6. 리눅스에서 Go 테스트 + Playwright 화면 시험
- [ ] 7. 사용 가이드 이미지 (docs/FINDEROAD_사용가이드.png)
- [ ] 8. 맥 빌드 서버 결과 확인 → v1.0.0 릴리스 (releases/latest.json)
- [ ] 9. 설치 파일 · 가이드 이미지 전달

### 작업 기록
- 2026-10-23 시작. 디자인 확정 후 개발 착수.

## 구조 (계획)

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
docs/                  사용 가이드 이미지와 원본(guide.html)
```

## 결정 사항

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
- 검색: 이름 칸과 확장자 칸이 따로. 이름을 비우고 확장자만으로도 검색. 분류 버튼(전체 · 동영상 · 이미지 · 문서 · 음악 · 압축 · 코드)을 누르면 확장자 칸이 채워짐.
  범위: Mac 전체 또는 선택한 폴더(여러 개, 하위 폴더 포함). 초성 검색은 하지 않음(사용자 결정).
- 삭제: 기본은 휴지통으로(⌘⌫, ⌘Z 로 되돌리기). **즉시 삭제**(⌘⌥⌫)는 확인 창을 거친 뒤 되돌릴 수 없게 지움.
- 붙여넣기 · 이동: 검색 결과에는 '현재 폴더'가 없으므로 대상 폴더를 고르는 창을 띄운다. 같은 이름이 있으면 '파일 2.mp4' 처럼 둘 다 남긴다.
- 커밋 작성자: `Nevertheless_D <NeverthelessD@users.noreply.github.com>`
