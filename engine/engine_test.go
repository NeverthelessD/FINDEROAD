package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestHangul(t *testing.T) {
	nfd := decomposeHangul("제주여행_브이로그.mp4")
	if nfd == "제주여행_브이로그.mp4" {
		t.Fatal("decompose did nothing")
	}
	if got := composeHangul(nfd); got != "제주여행_브이로그.mp4" {
		t.Fatalf("roundtrip: %q", got)
	}
	if got := composeHangul("abc 한글"); got != "abc 한글" {
		t.Fatalf("nfc untouched: %q", got)
	}
	// 받침 있는 글자 · 겹받침
	for _, s := range []string{"닭갈비", "읽기", "값", "ㄱㄴ", "가나다라"} {
		if got := composeHangul(decomposeHangul(s)); got != s {
			t.Fatalf("%s → %q", s, got)
		}
	}
}

func TestParseExts(t *testing.T) {
	got := parseExts(" mp4, .MOV *.mkv;mp4  tar.gz ")
	want := []string{"mp4", "mov", "mkv", "tar.gz"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if len(parseExts(" , ; ")) != 0 {
		t.Fatal("empty")
	}
}

func TestNameMatches(t *testing.T) {
	q := newQuery(SearchReq{Name: "제주 VLOG", Ext: "mp4, mov"})
	cases := map[string]bool{
		"제주여행_vlog.mp4":                  true,
		decomposeHangul("제주여행_Vlog.MP4"): true,
		"제주여행_vlog.mkv":                  false,
		"서울_vlog.mp4":                    false,
		"제주vlog":                         false,
	}
	for name, want := range cases {
		if got := q.nameMatches(name, false); got != want {
			t.Errorf("%q: got %v want %v", name, got, want)
		}
	}
	q2 := newQuery(SearchReq{Ext: "app"})
	if !q2.nameMatches("Safari.app", true) {
		t.Error("app bundle should match by ext")
	}
	q3 := newQuery(SearchReq{})
	if q3.nameMatches("폴더", true) {
		t.Error("no-condition search should not list folders")
	}
	if !q3.nameMatches("a.txt", false) {
		t.Error("no-condition search lists files")
	}
}

func mk(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, f := range files {
		p := filepath.Join(root, f)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func names(items []Item) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, it.Name)
	}
	sort.Strings(out)
	return out
}

func TestWalk(t *testing.T) {
	root := t.TempDir()
	mk(t, root,
		"영상/"+decomposeHangul("제주여행.mp4"),
		"영상/회의.MOV",
		"영상/.숨김.mp4",
		".hidden/secret.mp4",
		"사진/a.jpg",
		"Safari.app/Contents/inner.mp4",
		"깊은/폴더/안/끝.mp4",
	)
	c := newCollector(newQuery(SearchReq{Ext: "mp4 mov"}))
	if !walk(context.Background(), root, c, time.Now().Add(10*time.Second)) {
		t.Fatal("walk did not finish")
	}
	got := names(c.items)
	want := []string{"끝.mp4", "제주여행.mp4", "회의.MOV"}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	// 숨김 파일 포함
	c = newCollector(newQuery(SearchReq{Ext: "mp4", Hidden: true}))
	walk(context.Background(), root, c, time.Now().Add(10*time.Second))
	if len(c.items) != 4 { // 제주여행 · 끝 · .숨김 · secret (앱 안쪽은 여전히 빼요)
		t.Fatalf("hidden: %v", names(c.items))
	}
	// 이름으로 폴더 찾기
	c = newCollector(newQuery(SearchReq{Name: "영상"}))
	walk(context.Background(), root, c, time.Now().Add(10*time.Second))
	if got := names(c.items); !reflect.DeepEqual(got, []string{"영상"}) || !c.items[0].IsDir || c.items[0].Kind != "folder" {
		t.Fatalf("folder by name: %+v", c.items)
	}
	// 앱 묶음은 하나의 항목
	c = newCollector(newQuery(SearchReq{Ext: "app"}))
	walk(context.Background(), root, c, time.Now().Add(10*time.Second))
	if got := names(c.items); !reflect.DeepEqual(got, []string{"Safari.app"}) || c.items[0].Kind != "app" {
		t.Fatalf("app: %+v", c.items)
	}
}

func TestLibraryRule(t *testing.T) {
	q := &query{home: "/Users/me"}
	if !q.skipRel("/", "/Users/me/Library/Caches/x.mp4") {
		t.Error("library should be skipped")
	}
	if q.skipRel("/", "/Users/me/Library/Mobile Documents/com~apple~CloudDocs/x.mp4") {
		t.Error("iCloud Drive should be kept")
	}
	if q.skipRel("/Users/me/Library/Caches", "/Users/me/Library/Caches/x.mp4") {
		t.Error("explicitly chosen Library folder should be searched")
	}
	if !q.skipRel("/", "/System/Library/x.mp4") || !q.skipRel("/", "/Applications/A.app/Contents/x.mp4") {
		t.Error("system / bundle insides should be skipped")
	}
	if q.skipRel("/", "/Applications/A.app") {
		t.Error("the app itself is fine")
	}
}

func TestCleanRoots(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(a, "sub"), 0o755)
	got := cleanRoots([]string{filepath.Join(a, "sub"), a, b, b, "/nope/xyz", "relative"})
	want := []string{a, b}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestMdQuery(t *testing.T) {
	q := newQuery(SearchReq{Name: `여행 "x*`, Ext: "mp4,mov"})
	s := mdQuery(q)
	d := decomposeHangul("여행")
	want := `(kMDItemFSName == "*여행*"cd || kMDItemFSName == "*여행*"c || kMDItemDisplayName == "*여행*"cd || ` +
		`kMDItemFSName == "*` + d + `*"cd || kMDItemFSName == "*` + d + `*"c || kMDItemDisplayName == "*` + d + `*"cd) && ` +
		`(kMDItemFSName == "*x*"cd || kMDItemFSName == "*x*"c || kMDItemDisplayName == "*x*"cd) && ` +
		`(kMDItemFSName == "*.mp4"c || kMDItemFSName == "*.mov"c)`
	if s != want {
		t.Fatalf("got  %s\nwant %s", s, want)
	}
}

// 이름의 일부만 써도 찾기 ('호랑이' → '호랑이님 증명사진.jpg'), 띄어쓰기는 있든 없든
func TestPartialHangulMatch(t *testing.T) {
	file := "호랑이님 증명사진.jpg"
	cases := map[string]bool{
		"호랑이": true, "호랑": true, "랑이님": true, "증명": true, "사진": true, "호랑이 사진": true,
		"호랑이님증명사진": true, "증명 사진": true, "호랑이님_증명사진": true, "님 증": true,
		"고양이": false, "호랑이 고양이": false, "사진호랑이": false,
	}
	for term, want := range cases {
		q := newQuery(SearchReq{Name: term})
		for _, name := range []string{file, decomposeHangul(file)} {
			if got := q.nameMatches(name, false); got != want {
				t.Errorf("검색어 %q, 이름 %q: got %v want %v", term, name, got, want)
			}
		}
	}
	// 같은 파일이 NFC · NFD 두 모양으로 들어와도 한 번만
	root0 := t.TempDir()
	mk(t, root0, file)
	c0 := newCollector(newQuery(SearchReq{Name: "호랑이"}))
	c0.add(filepath.Join(root0, file), nil)
	c0.add(decomposeHangul(filepath.Join(root0, file)), nil)
	if len(c0.items) != 1 {
		t.Fatalf("dup: %d items", len(c0.items))
	}
	// 직접 훑기로도 찾는지
	root := t.TempDir()
	mk(t, root, decomposeHangul(file), "다른 파일.jpg")
	c := newCollector(newQuery(SearchReq{Name: "호랑이"}))
	walk(context.Background(), root, c, time.Now().Add(time.Minute))
	if got := names(c.items); !reflect.DeepEqual(got, []string{file}) {
		t.Fatalf("walk got %v", got)
	}
}

func TestUniqueDestAndProtect(t *testing.T) {
	d := t.TempDir()
	mk(t, d, "a.mp4", "a 2.mp4", "폴더/x")
	if got := filepath.Base(uniqueDest(d, "a.mp4")); got != "a 3.mp4" {
		t.Fatalf("got %s", got)
	}
	if got := filepath.Base(uniqueDest(d, "폴더")); got != "폴더 2" {
		t.Fatalf("got %s", got)
	}
	if got := filepath.Base(uniqueDest(d, "new.txt")); got != "new.txt" {
		t.Fatalf("got %s", got)
	}
	home, _ := os.UserHomeDir()
	for _, p := range []string{"/", "/System", "/Users", home, filepath.Join(home, "Documents"), "/Volumes/Disk", "relative"} {
		if protectedPath(p) == nil {
			t.Errorf("%s should be protected", p)
		}
	}
	if err := protectedPath(filepath.Join(d, "a.mp4")); err != nil {
		t.Errorf("normal file blocked: %v", err)
	}
}

func TestCopyMove(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	mk(t, src, "a.txt", "dir/b.txt")
	mk(t, dst, "a.txt")
	to := uniqueDest(dst, "a.txt")
	if err := copyItem(filepath.Join(src, "a.txt"), to); err != nil {
		t.Fatal(err)
	}
	if filepath.Base(to) != "a 2.txt" || !fileExists(to) || !fileExists(filepath.Join(src, "a.txt")) {
		t.Fatal("copy result wrong")
	}
	if err := copyItem(filepath.Join(src, "dir"), filepath.Join(dst, "dir")); err != nil || !fileExists(filepath.Join(dst, "dir", "b.txt")) {
		t.Fatalf("dir copy: %v", err)
	}
	if err := moveItem(filepath.Join(src, "a.txt"), filepath.Join(dst, "moved.txt")); err != nil {
		t.Fatal(err)
	}
	if fileExists(filepath.Join(src, "a.txt")) || !fileExists(filepath.Join(dst, "moved.txt")) {
		t.Fatal("move result wrong")
	}
}

func TestCompareVersions(t *testing.T) {
	if compareVersions("1.0.10", "1.0.9") != 1 || compareVersions("1.0.0", "1.0") != 0 || compareVersions("v1.2.0", "1.10.0") != -1 {
		t.Fatal("compare")
	}
}

// 사용자 제보: 'hwp, hwpx' 를 함께 넣으면 둘 다 나와야 해요
func TestMultiExt(t *testing.T) {
	root := t.TempDir()
	mk(t, root, "가/보고서.hwp", "나/다/계획서.hwpx", "라/표.xlsx", "마/hwp설명.txt", "바/x.HWPX")
	for _, ext := range []string{"hwp, hwpx", "hwp hwpx", "*.hwp;*.hwpx", ".HWP，.hwpx", "hwp,hwpx,"} {
		c := newCollector(newQuery(SearchReq{Ext: ext}))
		walk(context.Background(), root, c, time.Now().Add(10*time.Second))
		items, _, _, _ := c.snapshot()
		names := []string{}
		for _, it := range items {
			names = append(names, it.Name)
		}
		sort.Strings(names)
		if !reflect.DeepEqual(names, []string{"x.HWPX", "계획서.hwpx", "보고서.hwp"}) {
			t.Errorf("%q → %v", ext, names)
		}
	}
}

// 사용자 제보: 'Mac 전체'에서는 깊은 폴더의 파일이 빠졌어요 → 큰 홈 폴더도 끝까지 훑는지
func TestDeepWalkLargeTree(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// 앞쪽(가나다순)에 큰 폴더를 많이 두고, 맨 뒤 깊은 곳에 찾는 파일
	for i := 0; i < 300; i++ {
		d := filepath.Join(home, "AAA큰폴더", fmt.Sprintf("%03d", i))
		os.MkdirAll(d, 0o755)
		for k := 0; k < 20; k++ {
			os.WriteFile(filepath.Join(d, fmt.Sprintf("f%02d.dat", k)), nil, 0o644)
		}
	}
	mk(t, home, "zz/깊은/곳/아주/깊은/호랑이님 증명사진.jpg", "Documents/호랑이.pdf",
		"Library/Mobile Documents/com~apple~CloudDocs/호랑이 iCloud.txt", "Library/Caches/호랑이 캐시.txt", "Library/CloudStorage/x/호랑이.hwpx")
	q := newQuery(SearchReq{Name: "호랑이"})
	roots := macRoots(q)
	if len(roots) < 3 || roots[0] != filepath.Join(home, "Documents") {
		t.Fatalf("roots %v", roots)
	}
	c := newCollector(q)
	w := newWalker(context.Background(), c, time.Now().Add(30*time.Second), roots)
	if !w.run() {
		t.Fatal("walk did not finish")
	}
	items, _, _, _ := c.snapshot()
	got := map[string]int{}
	for _, it := range items {
		got[it.Name]++
	}
	for _, want := range []string{"호랑이님 증명사진.jpg", "호랑이.pdf", "호랑이 iCloud.txt", "호랑이.hwpx"} {
		if got[want] != 1 {
			t.Errorf("%s: %d번 (%v)", want, got[want], got)
		}
	}
	if got["호랑이 캐시.txt"] != 0 {
		t.Error("~/Library/Caches 는 빼야 해요")
	}
	// 정확도: 이름이 검색어와 똑같은 '호랑이.pdf' · '호랑이.hwpx' 가 맨 위
	if items[0].Score < items[len(items)-1].Score || (items[0].Name != "호랑이.pdf" && items[0].Name != "호랑이.hwpx") {
		t.Errorf("order: %v", items)
	}
}

func TestScore(t *testing.T) {
	q := newQuery(SearchReq{Name: "증명사진"})
	a, b, c := q.score("증명사진.jpg"), q.score("증명사진_원본.jpg"), q.score("호랑이님 증명사진 최종본 진짜최종.jpg")
	if !(a > b && b > c) {
		t.Fatalf("%d %d %d", a, b, c)
	}
}

// 화면이 쓰는 방식: 시작 → 진행 중 결과 받기 → 끝
func TestSearchJobStream(t *testing.T) {
	root := t.TempDir()
	mk(t, root, "a/보고서.hwp", "b/계획서.hwpx", "c/메모.txt")
	app := &App{token: "tk"}
	h := app.routes()
	call := func(method, path, body string) map[string]any {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("x-token", "tk")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var m map[string]any
		json.Unmarshal(rec.Body.Bytes(), &m)
		return m
	}
	st := call("POST", "/api/search/start", fmt.Sprintf(`{"ext":"hwp, hwpx","scope":"folders","folders":[%q]}`, root))
	id, _ := st["id"].(string)
	if id == "" {
		t.Fatalf("start: %v", st)
	}
	var last map[string]any
	for i := 0; i < 100; i++ {
		last = call("GET", "/api/search/poll?id="+id+"&v=-1", "")
		if last["done"] == true {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if last["done"] != true || last["total"].(float64) != 2 {
		t.Fatalf("poll: %v", last)
	}
	again := call("GET", fmt.Sprintf("/api/search/poll?id=%s&v=%v", id, last["version"]), "")
	if _, has := again["items"]; has {
		t.Error("바뀐 게 없으면 목록은 보내지 않아요")
	}
	if e := call("POST", "/api/search/start", `{"scope":"all"}`); e["error"] == nil {
		t.Error("조건 없이 Mac 전체는 안내")
	}
}
