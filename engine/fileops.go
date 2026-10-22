package main

// 파일 작업: 복사(붙여넣기) · 이동 · 되돌리기 · 휴지통(맥 앱에서는 Swift 가 해요) · 즉시 삭제

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

type moved struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type opErr struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// protectedPath: 지우거나 옮기면 큰일 나는 곳(디스크 맨 위, 시스템 폴더, 홈 폴더와 그 기본 폴더)은 막아요
func protectedPath(p string) error {
	if !filepath.IsAbs(p) {
		return errors.New("경로가 올바르지 않아요")
	}
	p = filepath.Clean(p)
	home, _ := os.UserHomeDir()
	blocked := []string{"/", "/System", "/Library", "/Applications", "/Users", "/Volumes", "/private", "/usr", "/bin",
		"/sbin", "/etc", "/var", "/opt", "/cores", "/dev", "/tmp", "/Network"}
	for _, b := range blocked {
		if p == b {
			return errors.New("시스템에 꼭 필요한 폴더라서 할 수 없어요")
		}
	}
	if strings.HasPrefix(p, "/System/") {
		return errors.New("시스템 파일이라서 할 수 없어요")
	}
	if strings.HasPrefix(p, "/Volumes/") && !strings.Contains(p[len("/Volumes/"):], "/") {
		return errors.New("디스크 전체는 다룰 수 없어요")
	}
	if home != "" {
		if p == home || filepath.Dir(p) == "/Users" {
			return errors.New("사용자 폴더 전체는 다룰 수 없어요")
		}
		for _, n := range []string{"Desktop", "Documents", "Downloads", "Library", "Movies", "Music", "Pictures", "Public", "Applications", ".Trash"} {
			if p == filepath.Join(home, n) {
				return errors.New("맥의 기본 폴더라서 할 수 없어요")
			}
		}
	}
	return nil
}

// uniqueDest: 같은 이름이 있으면 Finder 처럼 '이름 2.확장자' 로 둘 다 남겨요
func uniqueDest(dir, name string) string {
	p := filepath.Join(dir, name)
	if !fileExists(p) {
		return p
	}
	base, ext := name, ""
	if e := filepath.Ext(name); e != "" && e != name {
		base, ext = strings.TrimSuffix(name, e), e
	}
	for i := 2; i < 10000; i++ {
		p = filepath.Join(dir, fmt.Sprintf("%s %d%s", base, i, ext))
		if !fileExists(p) {
			return p
		}
	}
	return filepath.Join(dir, fmt.Sprintf("%s %d%s", base, time.Now().UnixNano(), ext))
}

// copyItem: 파일이나 폴더를 그대로(만든 날짜 · 꼬리표 등 포함) 복사해요
func copyItem(src, dst string) error {
	var out []byte
	var err error
	if runtime.GOOS == "darwin" {
		out, err = exec.Command("/usr/bin/ditto", src, dst).CombinedOutput()
	} else {
		out, err = exec.Command("cp", "-a", "--", src, dst).CombinedOutput()
	}
	if err != nil {
		os.RemoveAll(dst)
		msg := lastLine(string(out))
		if strings.Contains(msg, "Permission denied") || strings.Contains(msg, "Operation not permitted") {
			return errors.New("권한이 없어서 복사하지 못했어요")
		}
		if strings.Contains(msg, "No space") {
			return errors.New("디스크 공간이 모자라요")
		}
		if msg == "" {
			msg = err.Error()
		}
		return errors.New("복사하지 못했어요: " + msg)
	}
	return nil
}

// moveItem: 같은 디스크면 이름만 바꾸고, 다른 디스크면 복사한 뒤 원본을 지워요
func moveItem(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.EXDEV) {
		if err := copyItem(src, dst); err != nil {
			return err
		}
		if err := os.RemoveAll(src); err != nil {
			return errors.New("복사는 했지만 원래 파일을 지우지 못했어요")
		}
		return nil
	}
	return friendlyErr(err)
}

func friendlyErr(err error) error {
	switch {
	case errors.Is(err, os.ErrPermission):
		return errors.New("권한이 없어요")
	case errors.Is(err, os.ErrNotExist):
		return errors.New("파일을 찾을 수 없어요 (옮겨졌거나 지워졌을 수 있어요)")
	case errors.Is(err, syscall.ENOSPC):
		return errors.New("디스크 공간이 모자라요")
	}
	return err
}

func cleanPaths(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, p := range in {
		if p == "" || !filepath.IsAbs(p) {
			continue
		}
		p = filepath.Clean(p)
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// POST {mode: copy|move, paths, dest}
func (a *App) handleTransfer(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Mode  string   `json:"mode"`
		Paths []string `json:"paths"`
		Dest  string   `json:"dest"`
	}
	if err := readJSON(r, &in); err != nil {
		jsonError(w, 400, "bad request")
		return
	}
	dest := filepath.Clean(in.Dest)
	if !filepath.IsAbs(dest) || !dirExists(dest) {
		jsonError(w, 400, "대상 폴더를 찾을 수 없어요")
		return
	}
	paths := cleanPaths(in.Paths)
	if len(paths) == 0 {
		jsonError(w, 400, "고른 파일이 없어요")
		return
	}
	items := []moved{}
	errs := []opErr{}
	skipped := 0
	for _, src := range paths {
		name := filepath.Base(src)
		if _, err := os.Lstat(src); err != nil {
			errs = append(errs, opErr{src, friendlyErr(err).Error()})
			continue
		}
		if dest == src || strings.HasPrefix(dest+"/", src+"/") {
			errs = append(errs, opErr{src, "폴더를 자기 자신 안으로 넣을 수 없어요"})
			continue
		}
		if in.Mode == "move" {
			if err := protectedPath(src); err != nil {
				errs = append(errs, opErr{src, err.Error()})
				continue
			}
			if filepath.Dir(src) == dest {
				skipped++
				continue
			}
			to := uniqueDest(dest, name)
			if err := moveItem(src, to); err != nil {
				errs = append(errs, opErr{src, err.Error()})
				continue
			}
			items = append(items, moved{src, to})
		} else {
			to := uniqueDest(dest, name)
			if err := copyItem(src, to); err != nil {
				errs = append(errs, opErr{src, err.Error()})
				continue
			}
			items = append(items, moved{src, to})
		}
	}
	writeJSON(w, map[string]any{"ok": len(errs) == 0, "items": items, "errors": errs, "skipped": skipped})
}

// POST {items:[{from,to}]} — 되돌리기: to 에 있는 것을 from 으로 다시 옮겨요
func (a *App) handleMoveBack(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Items []moved `json:"items"`
	}
	if err := readJSON(r, &in); err != nil {
		jsonError(w, 400, "bad request")
		return
	}
	done := []moved{}
	errs := []opErr{}
	for _, m := range in.Items {
		from, to := filepath.Clean(m.From), filepath.Clean(m.To)
		if !filepath.IsAbs(from) || !filepath.IsAbs(to) {
			continue
		}
		if _, err := os.Lstat(to); err != nil {
			errs = append(errs, opErr{from, "되돌릴 파일을 찾을 수 없어요"})
			continue
		}
		os.MkdirAll(filepath.Dir(from), 0o755)
		back := from
		if fileExists(back) {
			back = uniqueDest(filepath.Dir(from), filepath.Base(from))
		}
		if err := moveItem(to, back); err != nil {
			errs = append(errs, opErr{from, err.Error()})
			continue
		}
		done = append(done, moved{to, back})
	}
	writeJSON(w, map[string]any{"ok": len(errs) == 0, "items": done, "errors": errs})
}

// POST {paths} — 휴지통으로 (맥 앱에서는 Swift 가 macOS 휴지통 기능으로 해요. 이건 브라우저 · 시험용)
func (a *App) handleTrash(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Paths []string `json:"paths"`
	}
	if err := readJSON(r, &in); err != nil {
		jsonError(w, 400, "bad request")
		return
	}
	home, _ := os.UserHomeDir()
	trash := filepath.Join(home, ".Trash")
	if runtime.GOOS != "darwin" {
		trash = filepath.Join(a.support, "trash")
	}
	os.MkdirAll(trash, 0o755)
	items := []moved{}
	errs := []opErr{}
	for _, p := range cleanPaths(in.Paths) {
		if err := protectedPath(p); err != nil {
			errs = append(errs, opErr{p, err.Error()})
			continue
		}
		if _, err := os.Lstat(p); err != nil {
			errs = append(errs, opErr{p, friendlyErr(err).Error()})
			continue
		}
		to := uniqueDest(trash, filepath.Base(p))
		if err := os.Rename(p, to); err != nil {
			if errors.Is(err, syscall.EXDEV) {
				errs = append(errs, opErr{p, "이 디스크의 파일은 여기서 휴지통으로 옮길 수 없어요"})
			} else {
				errs = append(errs, opErr{p, friendlyErr(err).Error()})
			}
			continue
		}
		items = append(items, moved{p, to})
	}
	writeJSON(w, map[string]any{"ok": len(errs) == 0, "items": items, "errors": errs})
}

// POST {paths} — 즉시 삭제 (되돌릴 수 없어요. 화면에서 확인을 받은 뒤에만 불러요)
func (a *App) handleDelete(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Paths []string `json:"paths"`
	}
	if err := readJSON(r, &in); err != nil {
		jsonError(w, 400, "bad request")
		return
	}
	deleted := []string{}
	errs := []opErr{}
	for _, p := range cleanPaths(in.Paths) {
		if err := protectedPath(p); err != nil {
			errs = append(errs, opErr{p, err.Error()})
			continue
		}
		if _, err := os.Lstat(p); err != nil {
			errs = append(errs, opErr{p, friendlyErr(err).Error()})
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			errs = append(errs, opErr{p, friendlyErr(err).Error()})
			continue
		}
		log.Printf("deleted permanently: %s", p)
		deleted = append(deleted, p)
	}
	writeJSON(w, map[string]any{"ok": len(errs) == 0, "deleted": deleted, "errors": errs})
}

// GET → {paths} / POST {paths} — 앱 안 클립보드 (맥 앱에서는 macOS 클립보드를 써서 Finder 와도 주고받아요)
func (a *App) handleClipboard(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var in struct {
			Paths []string `json:"paths"`
		}
		readJSON(r, &in)
		a.mu.Lock()
		a.clip = cleanPaths(in.Paths)
		a.mu.Unlock()
	}
	a.mu.Lock()
	paths := append([]string{}, a.clip...)
	a.mu.Unlock()
	writeJSON(w, map[string]any{"ok": true, "paths": paths})
}

// POST {prompt, multiple} → {paths} — 폴더 고르기 (맥 앱에서는 Swift 가 macOS 창으로 해요. 이건 대체용)
func (a *App) handleChooseFolder(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Prompt   string `json:"prompt"`
		Multiple bool   `json:"multiple"`
	}
	readJSON(r, &in)
	if t := env("TEST_CHOOSE"); t != "" {
		paths := strings.Split(t, "|")
		if !in.Multiple {
			paths = paths[:1]
		}
		writeJSON(w, map[string]any{"ok": true, "paths": paths})
		return
	}
	if runtime.GOOS != "darwin" {
		jsonError(w, 501, "macOS에서만 지원해요")
		return
	}
	prompt := in.Prompt
	if prompt == "" {
		prompt = "폴더를 골라 주세요"
	}
	multi := ""
	if in.Multiple {
		multi = " with multiple selections allowed"
	}
	script := []string{
		"on run argv",
		"activate",
		"try",
		"set fs to choose folder with prompt (item 1 of argv)" + multi,
		"if class of fs is not list then set fs to {fs}",
		"set out to \"\"",
		"repeat with f in fs",
		"set out to out & POSIX path of f & linefeed",
		"end repeat",
		"return out",
		"on error number -128",
		"return \"\"",
		"end try",
		"end run",
	}
	args := []string{}
	for _, l := range script {
		args = append(args, "-e", l)
	}
	args = append(args, prompt)
	out, err := exec.Command("/usr/bin/osascript", args...).Output()
	if err != nil {
		jsonError(w, 500, "폴더 선택 창을 열지 못했어요")
		return
	}
	paths := []string{}
	for _, l := range strings.Split(string(out), "\n") {
		l = strings.TrimSpace(l)
		if l != "" && l != "/" {
			l = strings.TrimSuffix(l, "/")
		}
		if l != "" {
			paths = append(paths, l)
		}
	}
	writeJSON(w, map[string]any{"ok": true, "paths": paths})
}

func (a *App) handleOpen(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Paths []string `json:"paths"`
	}
	readJSON(r, &in)
	paths := cleanPaths(in.Paths)
	if runtime.GOOS == "darwin" && len(paths) > 0 {
		exec.Command("/usr/bin/open", paths...).Start() // 경로는 모두 / 로 시작해서 옵션으로 읽히지 않아요
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (a *App) handleReveal(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Paths []string `json:"paths"`
	}
	readJSON(r, &in)
	paths := cleanPaths(in.Paths)
	if runtime.GOOS == "darwin" && len(paths) > 0 {
		exec.Command("/usr/bin/open", append([]string{"-R"}, paths...)...).Start()
	}
	writeJSON(w, map[string]any{"ok": true})
}
