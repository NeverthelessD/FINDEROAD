package main

// 검색 요청
//
// 화면은 /api/search/start 로 검색을 시작하고 /api/search/poll 로 0.4초마다 지금까지 찾은 결과를 받아요.
// Spotlight 답은 1초 안에 들어오고, 직접 훑기로 찾은 파일이 그 뒤로 계속 채워져요. '그만'을 누르면 /api/search/stop.
// /api/search 는 끝날 때까지 기다렸다가 한 번에 답해요 (맥 시험 · 예전 화면용).

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type searchJob struct {
	id      string
	q       *query
	c       *collector
	cancel  context.CancelFunc
	start   time.Time
	folders bool

	mu       sync.Mutex
	w        *walker
	methods  map[string]bool
	notes    []string
	phase    string // spotlight · walk · done
	done     bool
	userStop bool
	partial  bool // 시간이 다 돼서 다 훑지 못함
	elapsed  time.Duration
}

var (
	jobsMu sync.Mutex
	jobs   = map[string]*searchJob{}
)

func newJobID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// prepareSearch: 요청을 확인하고 검색 일감을 만들어요 (아직 시작은 안 해요)
func prepareSearch(in SearchReq) (*searchJob, []string, error) {
	q := newQuery(in)
	folders := in.Scope == "folders"
	if !folders && len(q.terms) == 0 && len(q.exts) == 0 {
		return nil, nil, fmt.Errorf("파일 이름이나 확장자를 입력해 주세요")
	}
	var roots []string
	if folders {
		roots = cleanRoots(in.Folders)
		if len(roots) == 0 {
			return nil, nil, fmt.Errorf("검색할 폴더를 추가해 주세요")
		}
	}
	return &searchJob{id: newJobID(), q: q, c: newCollector(q), folders: folders, methods: map[string]bool{}, phase: "spotlight"}, roots, nil
}

func (j *searchJob) note(method, msg string) {
	j.mu.Lock()
	if method != "" {
		j.methods[method] = true
	}
	if msg != "" {
		j.notes = append(j.notes, msg)
	}
	j.mu.Unlock()
}

// run: Spotlight 와 직접 훑기를 동시에 해요. 끝나면 돌아와요.
func (j *searchJob) run(parent context.Context, roots []string, budget time.Duration) {
	ctx, cancel := context.WithTimeout(parent, budget+10*time.Second)
	j.mu.Lock()
	j.cancel = cancel
	j.start = time.Now()
	j.mu.Unlock()
	defer cancel()
	q, c := j.q, j.c

	var wg sync.WaitGroup
	spot := func(root string) {
		if !spotlightOn(root) {
			return
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer recoverLog("spotlight " + root)
			if _, err := mdfind(ctx, root, c); err == nil {
				j.note("spotlight", "")
			}
		}()
	}
	if j.folders {
		for _, r := range roots {
			spot(r)
		}
	} else {
		spot("/")
		roots = macRoots(q)
	}

	w := newWalker(ctx, c, time.Now().Add(budget), roots)
	j.mu.Lock()
	j.w = w
	j.phase = "walk"
	j.mu.Unlock()
	finished := w.run()
	j.note("walk", "")
	wg.Wait()

	j.mu.Lock()
	defer j.mu.Unlock()
	j.done = true
	j.phase = "done"
	j.elapsed = time.Since(j.start)
	if !finished {
		switch {
		case j.userStop:
			j.notes = append(j.notes, "찾기를 멈췄어요 · 지금까지 찾은 것만 보여 드려요")
		case w.timeout.Load():
			j.partial = true
			j.notes = append(j.notes, fmt.Sprintf("시간이 오래 걸려 폴더 %s개까지만 확인했어요", commaInt(w.dirs.Load())))
		}
	}
	if c.capped {
		j.notes = append(j.notes, "너무 많아서 일부만 모았어요 · 이름이나 확장자를 더 적어 보세요")
	}
}

func (j *searchJob) stop() {
	j.mu.Lock()
	j.userStop = true
	cancel := j.cancel
	w := j.w
	j.mu.Unlock()
	if w != nil {
		w.stop()
	}
	if cancel != nil {
		cancel()
	}
}

func (j *searchJob) method() string {
	switch {
	case j.methods["spotlight"] && j.methods["walk"]:
		return "mixed"
	case j.methods["spotlight"]:
		return "spotlight"
	}
	return "walk"
}

// state: 화면에 보낼 지금 상태. 결과가 since 이후로 바뀌었을 때만 목록을 넣어요.
func (j *searchJob) state(since int64, withItems bool) map[string]any {
	items, total, capped, version := j.c.snapshot()
	j.mu.Lock()
	defer j.mu.Unlock()
	var dirs int64
	cur := ""
	if j.w != nil {
		dirs = j.w.dirs.Load()
		if s, _ := j.w.cur.Load().(string); s != "" {
			cur = displayPath(s)
		}
	}
	ms := time.Since(j.start).Milliseconds()
	if j.done {
		ms = j.elapsed.Milliseconds()
	}
	notes := append([]string{}, j.notes...)
	if j.done && total == 0 && len(notes) == 0 {
		notes = append(notes, "찾은 파일이 없어요")
	}
	out := map[string]any{
		"ok": true, "id": j.id, "done": j.done, "phase": j.phase, "version": version,
		"total": total, "capped": capped, "partial": j.partial, "method": j.method(),
		"ms": ms, "dirs": dirs, "cur": cur, "notes": notes,
	}
	if withItems || version != since {
		out["items"] = items
	}
	return out
}

func commaInt(n int64) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// ---------------------------------------------------------------- 요청

// /api/search: 끝날 때까지 기다렸다가 한 번에 답해요
func (a *App) handleSearch(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SearchReq
		BudgetMs int `json:"budgetMs"`
	}
	if err := readJSON(r, &in); err != nil {
		jsonError(w, 400, "bad request")
		return
	}
	j, roots, err := prepareSearch(in.SearchReq)
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	budget := walkBudgetSync
	if in.BudgetMs > 0 && time.Duration(in.BudgetMs)*time.Millisecond < budget {
		budget = time.Duration(in.BudgetMs) * time.Millisecond
	}
	j.run(r.Context(), roots, budget)
	if r.Context().Err() != nil {
		return // 화면이 새 검색을 시작해서 이 요청은 버려졌어요
	}
	writeJSON(w, j.state(-1, true))
}

// /api/search/start: 검색을 시작하고 곧바로 번호를 돌려줘요. 앞서 하던 검색은 멈춰요 (화면은 하나라서)
func (a *App) handleSearchStart(w http.ResponseWriter, r *http.Request) {
	var in SearchReq
	if err := readJSON(r, &in); err != nil {
		jsonError(w, 400, "bad request")
		return
	}
	j, roots, err := prepareSearch(in)
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	j.start = time.Now()
	jobsMu.Lock()
	for id, old := range jobs {
		old.mu.Lock()
		done, at := old.done, old.start
		old.mu.Unlock()
		if !done {
			go old.stop()
		}
		if done && (time.Since(at) > 2*time.Minute || len(jobs) > 8) {
			delete(jobs, id)
		}
	}
	jobs[j.id] = j
	jobsMu.Unlock()
	budget := walkBudgetAll
	if j.folders {
		budget = walkBudgetFolders
	}
	go func() {
		defer recoverLog("search job")
		j.run(context.Background(), roots, budget)
	}()
	writeJSON(w, map[string]any{"ok": true, "id": j.id})
}

func findJob(id string) *searchJob {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	return jobs[id]
}

// /api/search/poll?id=…&v=…: 지금까지 찾은 결과 (v 이후로 바뀐 게 없으면 목록은 빼요)
func (a *App) handleSearchPoll(w http.ResponseWriter, r *http.Request) {
	j := findJob(r.URL.Query().Get("id"))
	if j == nil {
		jsonError(w, 404, "검색이 끝났어요. 다시 검색해 주세요")
		return
	}
	var since int64 = -1
	fmt.Sscan(r.URL.Query().Get("v"), &since)
	writeJSON(w, j.state(since, false))
}

// /api/search/stop {id}: 직접 훑기를 멈추고 지금까지 찾은 것으로 끝내요
func (a *App) handleSearchStop(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID string `json:"id"`
	}
	readJSON(r, &in)
	if j := findJob(in.ID); j != nil {
		j.stop()
	}
	writeJSON(w, map[string]any{"ok": true})
}
