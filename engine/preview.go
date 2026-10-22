package main

// 미리보기: 파일 그대로 보내기(이미지 · 동영상 · 음악 · PDF), Quick Look 썸네일(qlmanage), 글 미리보기, 자세한 정보

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func queryPath(r *http.Request) (string, bool) {
	p := r.URL.Query().Get("path")
	if p == "" || !filepath.IsAbs(p) {
		return "", false
	}
	return filepath.Clean(p), true
}

// GET ?path= — 파일 내용 (동영상은 구간 요청을 지원해서 앞뒤로 넘길 수 있어요)
func (a *App) handleFile(w http.ResponseWriter, r *http.Request) {
	p, ok := queryPath(r)
	if !ok {
		http.Error(w, "bad path", 400)
		return
	}
	f, err := os.Open(p)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.Error(w, "not a file", 400)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, filepath.Base(p), st.ModTime(), f)
}

// GET ?path= — 글 파일 앞부분 (최대 48KB)
func (a *App) handleText(w http.ResponseWriter, r *http.Request) {
	p, ok := queryPath(r)
	if !ok {
		jsonError(w, 400, "bad path")
		return
	}
	f, err := os.Open(p)
	if err != nil {
		jsonError(w, 404, "파일을 열 수 없어요")
		return
	}
	defer f.Close()
	st, _ := f.Stat()
	buf := make([]byte, 48<<10)
	n, _ := io.ReadFull(f, buf)
	b := buf[:n]
	// 끝에서 잘린 글자는 버려요
	for i := 0; i < 3 && len(b) > 0 && !utf8.Valid(b); i++ {
		b = b[:len(b)-1]
	}
	binary := !utf8.Valid(b) || strings.ContainsRune(string(b), 0)
	text := ""
	if !binary {
		text = strings.TrimPrefix(string(b), "\ufeff")
	}
	writeJSON(w, map[string]any{"ok": true, "binary": binary, "text": text, "truncated": st != nil && st.Size() > int64(n)})
}

// GET ?path= — 자세한 정보
func (a *App) handleInfo(w http.ResponseWriter, r *http.Request) {
	p, ok := queryPath(r)
	if !ok {
		jsonError(w, 400, "bad path")
		return
	}
	st, err := os.Lstat(p)
	if err != nil {
		jsonError(w, 404, "파일을 찾을 수 없어요 (옮겨졌거나 지워졌을 수 있어요)")
		return
	}
	it := makeItem(p, st)
	out := map[string]any{"ok": true, "item": it, "created": birthTime(st).UnixMilli(), "symlink": st.Mode()&os.ModeSymlink != 0}
	if st.IsDir() {
		if ents, err := os.ReadDir(p); err == nil {
			n := 0
			for _, e := range ents {
				if !strings.HasPrefix(e.Name(), ".") {
					n++
				}
			}
			out["count"] = n
		} else {
			out["count"] = -1
		}
	}
	if m := mdlsInfo(p); m != nil {
		out["meta"] = m
	}
	writeJSON(w, out)
}

// mdlsInfo: Spotlight 가 아는 종류 이름 · 길이 · 크기(가로×세로)
func mdlsInfo(p string) map[string]any {
	if runtime.GOOS != "darwin" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	names := []string{"kMDItemKind", "kMDItemDurationSeconds", "kMDItemPixelWidth", "kMDItemPixelHeight"}
	args := []string{"-raw"}
	for _, n := range names {
		args = append(args, "-name", n)
	}
	args = append(args, p)
	out, err := exec.CommandContext(ctx, "/usr/bin/mdls", args...).Output()
	if err != nil {
		return nil
	}
	vals := strings.Split(string(out), "\x00")
	m := map[string]any{}
	get := func(i int) string {
		if i < len(vals) {
			v := strings.TrimSpace(vals[i])
			if v != "(null)" {
				return v
			}
		}
		return ""
	}
	if v := get(0); v != "" {
		m["kind"] = v
	}
	if v, err := strconv.ParseFloat(get(1), 64); err == nil && v > 0 {
		m["duration"] = v
	}
	wd, _ := strconv.Atoi(get(2))
	ht, _ := strconv.Atoi(get(3))
	if wd > 0 && ht > 0 {
		m["width"], m["height"] = wd, ht
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// ---------------------------------------------------------------- Quick Look 썸네일

type Thumbs struct {
	dir string
	sem chan struct{}
}

func NewThumbs(support string) *Thumbs {
	t := &Thumbs{dir: filepath.Join(support, "cache", "thumb"), sem: make(chan struct{}, 2)}
	os.MkdirAll(t.dir, 0o755)
	go t.prune()
	return t
}

// prune: 30일 넘게 안 쓴 썸네일은 지워요
func (t *Thumbs) prune() {
	defer recoverLog("thumb prune")
	ents, _ := os.ReadDir(t.dir)
	for _, e := range ents {
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > 30*24*time.Hour {
			os.Remove(filepath.Join(t.dir, e.Name()))
		}
	}
}

// GET ?path=&s=640 — PNG 썸네일 (macOS Quick Look 이 만든 것). 못 만들면 404
func (a *App) handleThumb(w http.ResponseWriter, r *http.Request) {
	p, ok := queryPath(r)
	if !ok {
		http.Error(w, "bad path", 400)
		return
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("s"))
	if size < 64 || size > 1600 {
		size = 640
	}
	st, err := os.Stat(p)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	if runtime.GOOS != "darwin" {
		http.Error(w, "no quick look", 404)
		return
	}
	t := a.thumbs
	h := sha1.Sum([]byte(fmt.Sprintf("%s|%d|%d|%d", p, size, st.Size(), st.ModTime().UnixNano())))
	cached := filepath.Join(t.dir, hex.EncodeToString(h[:])+".png")
	if fileExists(cached) {
		now := time.Now()
		os.Chtimes(cached, now, now)
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "private, max-age=600")
		http.ServeFile(w, r, cached)
		return
	}
	select {
	case t.sem <- struct{}{}:
	case <-r.Context().Done():
		return
	}
	defer func() { <-t.sem }()
	tmp, err := os.MkdirTemp(t.dir, "ql-")
	if err != nil {
		http.Error(w, "tmp", 500)
		return
	}
	defer os.RemoveAll(tmp)
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	exec.CommandContext(ctx, "/usr/bin/qlmanage", "-t", "-s", strconv.Itoa(size), "-o", tmp, p).Run()
	ents, _ := os.ReadDir(tmp)
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".png") {
			if os.Rename(filepath.Join(tmp, e.Name()), cached) == nil {
				w.Header().Set("Content-Type", "image/png")
				w.Header().Set("Cache-Control", "private, max-age=600")
				http.ServeFile(w, r, cached)
				return
			}
		}
	}
	http.Error(w, "no thumbnail", 404)
}
