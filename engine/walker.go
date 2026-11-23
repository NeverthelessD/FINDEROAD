package main

// 폴더 직접 훑기 (여러 일꾼이 함께)
//
// 예전에는 한 줄로 차례차례 8초만 훑어서, 홈 폴더가 크면 문서 · 다운로드 깊은 곳까지 닿기 전에 시간이 끝났어요.
// 그래서 'Mac 전체'로 찾으면 '선택한 폴더'로는 나오는 파일이 빠졌어요. 이제는
//   ① 일꾼 여러 명이 폴더를 나눠 맡아 동시에 훑고
//   ② 사용자가 자주 쓰는 곳(데스크탑 · 문서 · 다운로드 · iCloud Drive …)부터 먼저 보고
//   ③ 시간이 넉넉해서(Mac 전체 2분) 보통은 끝까지 다 훑어요. 찾는 중에도 결과가 계속 화면에 들어가요.

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type walkItem struct{ dir, root string }

type walker struct {
	ctx      context.Context
	q        *query
	c        *collector
	deadline time.Time
	roots    map[string]bool // 따로 훑는 시작 폴더 (다른 시작 폴더 안에서 만나면 건너뛰어요 — 두 번 보지 않게)

	mu      sync.Mutex
	cond    *sync.Cond
	stack   []walkItem
	active  int
	stopped atomic.Bool
	timeout atomic.Bool

	dirs atomic.Int64 // 확인한 폴더 수
	cur  atomic.Value // 지금 보고 있는 폴더 (화면 표시용)
}

func newWalker(ctx context.Context, c *collector, deadline time.Time, roots []string) *walker {
	w := &walker{ctx: ctx, q: c.q, c: c, deadline: deadline, roots: map[string]bool{}}
	w.cond = sync.NewCond(&w.mu)
	w.cur.Store("")
	for _, r := range roots {
		w.roots[filepath.Clean(r)] = true
	}
	// 앞에 있는 시작 폴더를 먼저 보도록 거꾸로 쌓아요 (맨 위에서 꺼내요)
	for i := len(roots) - 1; i >= 0; i-- {
		r := filepath.Clean(roots[i])
		w.stack = append(w.stack, walkItem{dir: r, root: r})
	}
	return w
}

func (w *walker) stop() {
	w.stopped.Store(true)
	w.mu.Lock()
	w.cond.Broadcast()
	w.mu.Unlock()
}

// run: 다 훑으면 true, 시간이 다 되거나 멈추면 false
func (w *walker) run() bool {
	n := runtime.NumCPU() * 2
	if n < 6 {
		n = 6
	}
	if n > 16 {
		n = 16
	}
	for i := 0; i < n; i++ {
		go func() {
			defer recoverLog("walker")
			w.work()
		}()
	}
	done := make(chan struct{})
	go func() {
		w.mu.Lock()
		for !(len(w.stack) == 0 && w.active == 0) && !w.stopped.Load() {
			w.cond.Wait()
		}
		w.mu.Unlock()
		close(done)
	}()
	timer := time.NewTimer(time.Until(w.deadline))
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		w.timeout.Store(true)
		w.stop()
	case <-w.ctx.Done():
		w.stop()
	}
	// 응답이 없는 디스크에 걸린 일꾼은 기다리지 않아요 (나중에 혼자 끝나요)
	return !w.stopped.Load()
}

func (w *walker) work() {
	for {
		w.mu.Lock()
		for len(w.stack) == 0 && !w.stopped.Load() {
			if w.active == 0 {
				w.cond.Broadcast()
				w.mu.Unlock()
				return
			}
			w.cond.Wait()
		}
		if w.stopped.Load() {
			w.mu.Unlock()
			return
		}
		it := w.stack[len(w.stack)-1]
		w.stack = w.stack[:len(w.stack)-1]
		w.active++
		w.mu.Unlock()

		kids := w.scan(it)

		w.mu.Lock()
		for i := len(kids) - 1; i >= 0; i-- {
			w.stack = append(w.stack, kids[i])
		}
		w.active--
		w.cond.Broadcast()
		w.mu.Unlock()
	}
}

// 개발 도구가 만드는 아주 큰 폴더는 사용자가 찾는 파일이 아니라서 건너뛰어요 ('숨김 · 시스템 파일도 찾기'를 켜면 들어가요)
var bulkyDirs = map[string]bool{"node_modules": true, "__pycache__": true, "DerivedData": true, "Pods": true}

// 타임머신 백업 디스크 (같은 파일이 수없이 나와서 Mac 전체 검색에서는 빼요)
var tmBackupName = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-\d{6}\.(backup|previous|inprogress)$`)

func (w *walker) scan(it walkItem) (kids []walkItem) {
	if w.ctx.Err() != nil || w.stopped.Load() {
		return nil
	}
	entries, err := os.ReadDir(it.dir)
	if n := w.dirs.Add(1); n%64 == 1 {
		w.cur.Store(it.dir)
	}
	if err != nil {
		return nil
	}
	q := w.q
	home := q.home
	for _, d := range entries {
		if w.stopped.Load() {
			return kids
		}
		name := d.Name()
		p := it.dir + "/" + name
		if it.dir == "/" {
			p = "/" + name
		}
		if d.IsDir() {
			if !q.hidden && strings.HasPrefix(name, ".") {
				continue
			}
			if w.roots[p] && p != it.root {
				continue // 따로 훑는 시작 폴더
			}
			if !q.hidden && q.inHiddenLibrary(it.root, p) && p != home+"/Library" {
				continue
			}
			if name == "Backups.backupdb" || tmBackupName.MatchString(name) {
				continue
			}
			ext := extOf(name)
			if packageExts[ext] {
				if q.nameMatches(name, true) {
					if info, err := d.Info(); err == nil && !w.c.add(p, info) {
						w.stop()
						return nil
					}
				}
				continue
			}
			if p == home+"/Library" && !q.hidden {
				kids = append(kids, walkItem{p, it.root}) // 안쪽의 iCloud Drive · 클라우드 폴더만 들어가요
				continue
			}
			if q.nameMatches(name, true) {
				if info, err := d.Info(); err == nil && !w.c.add(p, info) {
					w.stop()
					return nil
				}
			}
			if !q.hidden && bulkyDirs[name] {
				continue
			}
			kids = append(kids, walkItem{p, it.root})
			continue
		}
		if !q.hidden && strings.HasPrefix(name, ".") {
			continue
		}
		if !q.hidden && q.inHiddenLibrary(it.root, p) {
			continue
		}
		if q.nameMatches(name, false) {
			info, err := d.Info()
			if err != nil {
				continue
			}
			if !w.c.add(p, info) {
				w.stop()
				return nil
			}
		}
	}
	return kids
}

// macRoots: 'Mac 전체'에서 직접 훑을 곳. 자주 쓰는 곳을 먼저, 그다음 홈 폴더 나머지, 함께 쓰는 폴더, 응용 프로그램, 외장 디스크.
func macRoots(q *query) []string {
	out := []string{}
	add := func(p string) {
		if p != "" && dirExists(p) {
			out = append(out, filepath.Clean(p))
		}
	}
	if h := q.home; h != "" {
		for _, sub := range []string{"Desktop", "Documents", "Downloads", "Library/Mobile Documents", "Library/CloudStorage", "Pictures", "Movies", "Music"} {
			add(filepath.Join(h, sub))
		}
		add(h)
	}
	add("/Users/Shared")
	add("/Applications")
	if ents, err := os.ReadDir("/Volumes"); err == nil {
		for _, d := range ents {
			if !d.IsDir() || (!q.hidden && strings.HasPrefix(d.Name(), ".")) {
				continue // 'Macintosh HD' 는 / 로 가는 바로 가기라서 여기서 걸러져요
			}
			p := "/Volumes/" + d.Name()
			if isNetworkFS(p) || dirExists(p+"/Backups.backupdb") || isTimeMachineDisk(p) {
				continue
			}
			add(p)
		}
	}
	return out
}

func isTimeMachineDisk(p string) bool {
	ents, err := os.ReadDir(p)
	if err != nil {
		return false
	}
	for i, d := range ents {
		if i > 50 {
			break
		}
		if tmBackupName.MatchString(d.Name()) {
			return true
		}
	}
	return false
}
