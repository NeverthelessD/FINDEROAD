#!/bin/bash
# FINDEROAD 맥 빌드 (맥에서만 돌아요 — 맥 빌드 서버 scripts/ci-mac.sh 가 불러요)
#   Go 엔진(darwin/arm64) + Swift 앱 껍데기(app/shell/main.swift) → dist/FINDEROAD.app
#   → 앱 전체에 임시 서명(ad-hoc) → dist/FINDEROAD-<버전>.zip (업데이트용) · dist/FINDEROAD.zip (처음 설치용, 사용 가이드 포함)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="$(tr -d ' \n' < "$ROOT/VERSION")"
DIST="$ROOT/dist"
APP="$DIST/FINDEROAD.app"
rm -rf "$DIST"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

echo "▶ 엔진 테스트 · 빌드 (darwin/arm64, v$VERSION)"
( cd "$ROOT/engine" && go vet ./... && go test ./... )
( cd "$ROOT/engine" && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
    go build -trimpath -ldflags "-s -w -X main.build=$VERSION" -o "$APP/Contents/Resources/finderoad-engine" . )
chmod 755 "$APP/Contents/Resources/finderoad-engine"

echo "▶ 앱 껍데기 빌드 (Swift)"
xcrun swiftc --version
xcrun swiftc -O -target arm64-apple-macos12.0 \
  -o "$APP/Contents/MacOS/FINDEROAD" "$ROOT/app/shell/main.swift" \
  -framework AppKit -framework WebKit -framework Quartz
chmod 755 "$APP/Contents/MacOS/FINDEROAD"

sed "s/__VERSION__/$VERSION/g" "$ROOT/app/Info.plist" > "$APP/Contents/Info.plist"
plutil -lint "$APP/Contents/Info.plist"
cp "$ROOT/app/AppIcon.icns" "$APP/Contents/Resources/AppIcon.icns"
printf '%s' "$VERSION" > "$APP/Contents/Resources/version.txt"
printf 'APPL????' > "$APP/Contents/PkgInfo"

echo "▶ 임시 서명 (ad-hoc) · 확인"
codesign --force --sign - --timestamp=none "$APP/Contents/Resources/finderoad-engine"
codesign --force --deep --sign - --timestamp=none "$APP"
codesign --verify --deep --strict --verbose=2 "$APP"
codesign -dv "$APP" 2>&1 | sed 's/^/  /'
lipo -archs "$APP/Contents/MacOS/FINDEROAD" "$APP/Contents/Resources/finderoad-engine"

echo "▶ 압축"
ditto -c -k --norsrc --keepParent "$APP" "$DIST/FINDEROAD-$VERSION.zip"
mkdir -p "$DIST/first/FINDEROAD"
ditto "$APP" "$DIST/first/FINDEROAD/FINDEROAD.app"
[ -f "$ROOT/docs/FINDEROAD_사용가이드.png" ] && cp "$ROOT/docs/FINDEROAD_사용가이드.png" "$DIST/first/FINDEROAD/"
ditto -c -k --norsrc --keepParent "$DIST/first/FINDEROAD" "$DIST/FINDEROAD.zip"

echo "▶ 업데이트 파일을 풀어도 서명이 그대로인지 확인 (앱의 업데이트 기능처럼 unzip)"
CHECK="$DIST/check"; rm -rf "$CHECK"; mkdir -p "$CHECK"
( cd "$CHECK" && unzip -q "$DIST/FINDEROAD-$VERSION.zip" )
codesign --verify --deep --strict "$CHECK/FINDEROAD.app"
[ "$(cat "$CHECK/FINDEROAD.app/Contents/Resources/version.txt")" = "$VERSION" ]
rm -rf "$CHECK"
ls -la "$DIST"
echo "✔ dist/FINDEROAD-$VERSION.zip · dist/FINDEROAD.zip"
