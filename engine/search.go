package main

// 검색
//
// 범위가 'Mac 전체'면 Spotlight(mdfind)에 묻고, Spotlight 가 꺼진 맥이면 홈 폴더를 직접 훑어요.
// 범위가 '선택한 폴더'면 폴더마다 Spotlight 가 켜진 디스크인지 보고(mdutil -s), 꺼진 디스크(외장 드라이브 등)나
// Spotlight 가 아무것도 못 찾은 폴더는 직접 훑어요. 결과는 모두 다시 한 번 이름 · 확장자 조건으로 걸러요.

import (
	"bufio"
	"context"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type SearchReq struct {
	Name    string   `json:"name"`
	Ext     string   `json:"ext"`
	Scope   string   `json:"scope"` // all | folders
	Folders []string `json:"folders"`
	Hidden  bool     `json:"hidden"`
}

type Item struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Dir   string `json:"dir"`
	Ext   string `json:"ext"`
	Size  int64  `json:"size"`
	MTime int64  `json:"mtime"`
	IsDir bool   `json:"isDir"`
	Kind  string `json:"kind"`
	Score int    `json:"score"` // 이름이 검색어와 얼마나 맞는지 (클수록 위)
}

const (
	collectCap = 150000 // 이만큼 모으면 멈춰요 (보여 주는 건 정확도 · 최근 순으로 returnCap 개)
	returnCap  = 2000
	// 직접 훑기 시간. Spotlight 는 한글 이름 일부 · 색인에서 빠진 폴더 · 낯선 확장자(hwpx 등)를 놓칠 때가 있어서
	// 어느 범위든 Spotlight 답을 먼저 보여 주고, 그동안 폴더를 처음부터 끝까지 직접 훑어 채워요.
	walkBudgetAll     = 120 * time.Second // Mac 전체
	walkBudgetFolders = 180 * time.Second // 선택한 폴더
	walkBudgetSync    = 40 * time.Second  // 한 번에 답을 기다리는 /api/search (시험용)
)

// 안을 들여다보지 않는 묶음 폴더(앱 · 사진 보관함 등). 이름이 맞으면 하나의 항목으로 보여 줘요.
var packageExts = map[string]bool{
	"app": true, "bundle": true, "framework": true, "plugin": true, "kext": true, "appex": true, "xpc": true,
	"photoslibrary": true, "photolibrary": true, "musiclibrary": true, "tvlibrary": true, "imovielibrary": true,
	"fcpbundle": true, "logicx": true, "band": true, "xcodeproj": true, "xcworkspace": true, "xcarchive": true,
	"pages": true, "numbers": true, "key": true, "rtfd": true, "sparsebundle": true, "lrlibrary": true, "lrdata": true,
}

var kindExts = map[string][]string{
	"video":   {"mp4", "mov", "m4v", "mkv", "avi", "webm", "wmv", "flv", "mpg", "mpeg", "3gp", "mts", "m2ts"},
	"image":   {"jpg", "jpeg", "png", "heic", "heif", "gif", "webp", "bmp", "tif", "tiff", "svg", "avif", "raw", "cr2", "cr3", "nef", "arw", "dng", "psd", "ico"},
	"doc":     {"pdf", "doc", "docx", "hwp", "hwpx", "xls", "xlsx", "ppt", "pptx", "key", "pages", "numbers", "txt", "rtf", "md", "csv", "odt", "ods", "odp", "epub"},
	"audio":   {"mp3", "m4a", "wav", "aac", "flac", "aif", "aiff", "ogg", "wma", "opus"},
	"archive": {"zip", "dmg", "7z", "rar", "tar", "gz", "tgz", "bz2", "xz", "pkg", "iso"},
	"code":    {"swift", "js", "mjs", "ts", "jsx", "tsx", "py", "go", "java", "kt", "c", "cpp", "h", "hpp", "cs", "rb", "php", "rs", "html", "css", "scss", "json", "xml", "yml", "yaml", "sh", "zsh", "sql"},
	"app":     {"app"},
}

var extKind = func() map[string]string {
	m := map[string]string{}
	for k, list := range kindExts {
		for _, e := range list {
			if _, ok := m[e]; !ok {
				m[e] = k
			}
		}
	}
	return m
}()

func kindOf(ext string, isDir bool) string {
	if k, ok := extKind[ext]; ok && (!isDir || packageExts[ext] || ext == "app") {
		return k
	}
	if isDir {
		return "folder"
	}
	return "other"
}

// ---------------------------------------------------------------- 조건

type query struct {
	terms  []string // 소문자 · NFC
	exts   []string // 소문자, 점 없이
	hidden bool
	home   string
}

func newQuery(in SearchReq) *query {
	home, _ := os.UserHomeDir()
	return &query{terms: parseTerms(in.Name), exts: parseExts(in.Ext), hidden: in.Hidden, home: home}
}

// parseExts: "mp4, .mov *.MKV" → [mp4 mov mkv]
func parseExts(s string) []string {
	f := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';' || r == '\t' || r == '\n' || r == '、' || r == '，'
	})
	out := []string{}
	seen := map[string]bool{}
	for _, e := range f {
		e = strings.ToLower(strings.TrimSpace(e))
		e = strings.TrimLeft(e, "*")
		e = strings.TrimLeft(e, ".")
		e = cleanQueryText(e)
		if e == "" || strings.ContainsAny(e, "/") || seen[e] {
			continue
		}
		seen[e] = true
		out = append(out, e)
	}
	return out
}

func parseTerms(s string) []string {
	out := []string{}
	for _, t := range strings.Fields(strings.ToLower(composeHangul(s))) {
		if t = cleanQueryText(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// cleanQueryText: Spotlight 질의 문법에 쓰이는 글자(따옴표 · 별표 등)는 빼요
func cleanQueryText(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '"', '\\', '*', '?':
			return -1
		}
		return r
	}, s)
}

// compactName: 띄어쓰기 · 밑줄 · 하이픈을 뺀 이름 ('증명 사진' = '증명_사진' = '증명사진')
func compactName(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '_', '-', '\u3000', '\u00a0', '\t':
			return -1
		}
		return r
	}, s)
}

// nameMatches: 검색어가 이름 어디에든 들어 있으면 찾아요 ('호랑이' → '호랑이님 증명사진.jpg').
// 띄어쓰기 · 밑줄 · 하이픈은 있든 없든 같게 봐요 ('호랑이님증명사진' → '호랑이님 증명사진.jpg').
func (q *query) nameMatches(name string, isDir bool) bool {
	n := strings.ToLower(composeHangul(name))
	nc := ""
	for _, t := range q.terms {
		if strings.Contains(n, t) {
			continue
		}
		if nc == "" {
			nc = compactName(n)
		}
		if tc := compactName(t); tc == "" || !strings.Contains(nc, tc) {
			return false
		}
	}
	if len(q.exts) > 0 {
		for _, e := range q.exts {
			if strings.HasSuffix(n, "."+e) && len(n) > len(e)+1 {
				return true
			}
		}
		return false
	}
	if isDir && len(q.terms) == 0 {
		return false // 아무 조건 없이 폴더까지 늘어놓지는 않아요
	}
	return true
}

// score: 이름이 검색어와 얼마나 맞는지. 이름(확장자 뺀 것)이 검색어와 똑같으면 가장 위, 검색어로 시작하면 그다음,
// 단어 첫머리에서 맞으면 그다음, 가운데 어딘가에 들어 있으면 그다음. 같은 점수면 짧은 이름(군더더기가 적은 것)을 위로.
func (q *query) score(name string) int {
	if len(q.terms) == 0 {
		return 0
	}
	n := strings.ToLower(composeHangul(name))
	stem := n
	if e := extOf(n); e != "" {
		stem = strings.TrimSuffix(n, "."+e)
	}
	joined := strings.Join(q.terms, " ")
	cs, cj := compactName(stem), compactName(joined)
	s := 0
	switch {
	case stem == joined || cs == cj:
		s = 1000
	case strings.HasPrefix(stem, q.terms[0]) || strings.HasPrefix(cs, cj):
		s = 700
	default:
		s = 400
	}
	for _, t := range q.terms {
		if i := strings.Index(stem, t); i > 0 && strings.ContainsRune(" _-.()[]", rune(stem[i-1])) {
			s += 100 // 단어 첫머리에서 맞음
		}
		if strings.Contains(stem, t) {
			s += 20 // 띄어쓰기까지 그대로 맞음
		}
	}
	extra := len([]rune(cs)) - len([]rune(cj))
	if extra < 0 {
		extra = 0
	}
	if extra > 60 {
		extra = 60
	}
	return s + 60 - extra
}

var systemPrefixes = []string{"/System/", "/Library/", "/private/", "/usr/", "/bin/", "/sbin/", "/opt/", "/cores/", "/dev/", "/etc/", "/var/", "/tmp/"}

// skipRel: root 아래의 경로 p 를 결과에서 뺄지 (숨김 · 시스템 · 묶음 폴더 안쪽)
func (q *query) skipRel(root, p string) bool {
	rel := p
	if root != "/" && strings.HasPrefix(p, root+"/") {
		rel = p[len(root):]
	}
	parts := strings.Split(strings.Trim(rel, "/"), "/")
	for i, c := range parts {
		if c == "" {
			continue
		}
		if !q.hidden && strings.HasPrefix(c, ".") {
			return true
		}
		if i < len(parts)-1 {
			if ext := extOf(c); ext != "" && packageExts[ext] {
				return true
			}
		}
	}
	if q.hidden {
		return false
	}
	if root == "/" {
		for _, pre := range systemPrefixes {
			if strings.HasPrefix(p, pre) {
				return true
			}
		}
	}
	return q.inHiddenLibrary(root, p)
}

// ~/Library 는 앱들이 쓰는 곳이라 빼요. 단 iCloud Drive · 클라우드 폴더는 사용자가 쓰는 파일이라 넣어요.
func (q *query) inHiddenLibrary(root, p string) bool {
	if q.home == "" {
		return false
	}
	lib := q.home + "/Library"
	if strings.HasPrefix(root+"/", lib+"/") { // 사용자가 직접 Library 안의 폴더를 골랐으면 그대로
		return false
	}
	if p == lib {
		return true
	}
	if !strings.HasPrefix(p, lib+"/") {
		return false
	}
	return !strings.HasPrefix(p, lib+"/Mobile Documents/") && !strings.HasPrefix(p, lib+"/CloudStorage/")
}

func extOf(name string) string {
	i := strings.LastIndex(name, ".")
	if i <= 0 || i == len(name)-1 {
		return ""
	}
	return strings.ToLower(name[i+1:])
}

// ---------------------------------------------------------------- 결과 모으기

type collector struct {
	mu      sync.Mutex
	q       *query
	seen    map[string]bool
	items   []Item
	capped  bool
	version int64 // 결과가 늘 때마다 올라가요 (화면이 새로 받을지 판단)
}

func newCollector(q *query) *collector { return &collector{q: q, seen: map[string]bool{}} }

// add: 결과 하나를 넣어요. 더 받을 수 없으면 false.
func (c *collector) add(p string, info fs.FileInfo) bool {
	// Spotlight 와 직접 훑기가 같은 파일을 한글 모양(NFC/NFD)만 다르게 돌려줄 수 있어서 합친 모양으로 비교해요
	key := composeHangul(p)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.seen[key] {
		return true
	}
	if len(c.items) >= collectCap {
		c.capped = true
		return false
	}
	if info == nil {
		st, err := os.Lstat(p)
		if err != nil {
			return true
		}
		info = st
		// Spotlight 가 돌려준 폴더는 폴더 기준으로 다시 확인해요 (조건 없이 폴더까지 늘어놓지 않게)
		if info.IsDir() && !c.q.nameMatches(filepath.Base(p), true) {
			return true
		}
	}
	c.seen[key] = true
	it := makeItem(p, info)
	it.Score = c.q.score(it.Name)
	c.items = append(c.items, it)
	c.version++
	return true
}

func (c *collector) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// snapshot: 정확도 → 최근 수정 순으로 줄 세운 앞의 returnCap 개와 전체 수
func (c *collector) snapshot() (items []Item, total int, capped bool, version int64) {
	c.mu.Lock()
	all := append([]Item(nil), c.items...)
	capped, version = c.capped, c.version
	c.mu.Unlock()
	sortItems(all)
	total = len(all)
	if len(all) > returnCap {
		all = all[:returnCap]
	}
	if all == nil {
		all = []Item{}
	}
	return all, total, capped, version
}

func sortItems(items []Item) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		if items[i].MTime != items[j].MTime {
			return items[i].MTime > items[j].MTime
		}
		return items[i].Name < items[j].Name
	})
}

func (c *collector) markCapped() {
	c.mu.Lock()
	c.capped = true
	c.mu.Unlock()
}

func makeItem(p string, info fs.FileInfo) Item {
	name := composeHangul(filepath.Base(p))
	isDir := info.IsDir()
	ext := extOf(name)
	if isDir && !packageExts[ext] {
		ext = ""
	}
	size := info.Size()
	if isDir {
		size = -1
	}
	return Item{
		Path: p, Name: name, Dir: displayPath(filepath.Dir(p)), Ext: ext,
		Size: size, MTime: info.ModTime().UnixMilli(), IsDir: isDir, Kind: kindOf(ext, isDir),
	}
}

// ---------------------------------------------------------------- 폴더 직접 훑기

// walk: root 아래를 훑어요(여러 일꾼이 함께). 시간이 다 되면 멈추고 false 를 돌려줘요.
func walk(ctx context.Context, root string, c *collector, deadline time.Time) (finished bool) {
	return newWalker(ctx, c, deadline, []string{root}).run()
}

// ---------------------------------------------------------------- Spotlight

var (
	mdMu    sync.Mutex
	mdCache = map[string]mdState{}
)

type mdState struct {
	on bool
	at time.Time
}

func volumeOf(p string) string {
	if strings.HasPrefix(p, "/Volumes/") {
		rest := p[len("/Volumes/"):]
		if i := strings.Index(rest, "/"); i >= 0 {
			rest = rest[:i]
		}
		return "/Volumes/" + rest
	}
	return "/"
}

// spotlightOn: 이 경로가 있는 디스크에서 Spotlight 색인이 켜져 있는지 (5분 동안 기억)
func spotlightOn(p string) bool {
	if runtime.GOOS != "darwin" || env("NO_SPOTLIGHT") == "1" {
		return false
	}
	vol := volumeOf(p)
	mdMu.Lock()
	if s, ok := mdCache[vol]; ok && time.Since(s.at) < 5*time.Minute {
		mdMu.Unlock()
		return s.on
	}
	mdMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/usr/bin/mdutil", "-s", vol).CombinedOutput()
	on := err == nil && strings.Contains(string(out), "Indexing enabled")
	mdMu.Lock()
	mdCache[vol] = mdState{on: on, at: time.Now()}
	mdMu.Unlock()
	return on
}

func mdQuery(q *query) string {
	parts := []string{}
	for _, t := range q.terms {
		// 같은 조건을 여러 모양으로 물어요: 파일 이름 · 보이는 이름, 완성형(NFC) · 자모가 나뉜 형태(NFD),
		// 'd'(발음 구별 기호 무시) 있이 · 없이 — 'd' 가 한글 자모를 다르게 다뤄 일부만 맞는 이름을 놓치는 경우가 있어요.
		forms := []string{t}
		if nfd := decomposeHangul(t); nfd != t {
			forms = append(forms, nfd)
		}
		ors := []string{}
		for _, f := range forms {
			ors = append(ors,
				`kMDItemFSName == "*`+f+`*"cd`,
				`kMDItemFSName == "*`+f+`*"c`,
				`kMDItemDisplayName == "*`+f+`*"cd`)
		}
		parts = append(parts, "("+strings.Join(ors, " || ")+")")
	}
	if len(q.exts) > 0 {
		ors := []string{}
		for _, e := range q.exts {
			ors = append(ors, `kMDItemFSName == "*.`+e+`"c`)
		}
		parts = append(parts, "("+strings.Join(ors, " || ")+")")
	}
	if len(parts) == 0 {
		return `kMDItemFSName == "*"`
	}
	return strings.Join(parts, " && ")
}

// mdfind: Spotlight 에 물어서 찾은 경로를 하나씩 넘겨요. 찾은 수를 돌려줘요.
func mdfind(ctx context.Context, root string, c *collector) (int, error) {
	args := []string{"-0"}
	if root != "/" {
		args = append(args, "-onlyin", root)
	}
	args = append(args, mdQuery(c.q))
	cmd := exec.CommandContext(ctx, "/usr/bin/mdfind", args...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return 0, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	br := bufio.NewReaderSize(out, 256<<10)
	found := 0
	for {
		p, err := br.ReadString(0)
		if len(p) > 1 {
			p = strings.TrimSuffix(p, "\x00")
			if !c.q.skipRel(root, p) && c.q.nameMatches(filepath.Base(p), false) {
				if !c.add(p, nil) {
					cmd.Process.Kill()
					break
				}
				found++
			}
		}
		if err != nil {
			break
		}
	}
	cmd.Wait()
	return found, nil
}

// cleanRoots: 있는 폴더만, 다른 고른 폴더 안에 들어 있는 폴더는 빼요 (같은 파일이 두 번 나오지 않게)
func cleanRoots(in []string) []string {
	var roots []string
	for _, f := range in {
		f = filepath.Clean(f)
		if filepath.IsAbs(f) && dirExists(f) {
			roots = append(roots, f)
		}
	}
	sort.Strings(roots)
	out := []string{}
	for _, r := range roots {
		inside := false
		for _, o := range out {
			if o == "/" || r == o || strings.HasPrefix(r, o+"/") {
				inside = true
				break
			}
		}
		if !inside {
			out = append(out, r)
		}
	}
	return out
}
