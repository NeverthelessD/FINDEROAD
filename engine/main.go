// FINDEROAD — 파일 검색 · 미리보기 · 복사 · 이동 · 삭제 (macOS, Apple Silicon)
// made by. Nevertheless_D
//
// 이 엔진은 FINDEROAD.app(Dock 에 보이는 Swift 앱)이 자식으로 켜요. 앱이 켜져 있는 동안만 돌고,
// 앱이 끝나거나 사라지면(표준 입력이 닫히거나 부모가 바뀌면) 스스로 끝나요. 종료 신호는 언제나 따라요.
package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// build is the app version, set with -ldflags "-X main.build=1.2.3".
var build = "0.0.0-dev"

//go:embed web
var webFiles embed.FS

type Settings struct {
	Appearance string         `json:"appearance"` // system | light | dark
	AutoUpdate bool           `json:"autoUpdate"`
	Scope      string         `json:"scope"`   // all | folders
	Folders    []string       `json:"folders"` // 선택한 폴더들
	ShowHidden bool           `json:"showHidden"`
	UI         map[string]any `json:"ui"`
}

type App struct {
	support string
	token   string
	port    int

	mu       sync.Mutex
	settings Settings

	upd    *Updater
	thumbs *Thumbs
	clip   []string // 앱 안 클립보드 (맥 앱에서는 macOS 클립보드를 써요)
}

var app *App

func env(k string) string { return os.Getenv("FINDEROAD_" + k) }

func main() {
	home, _ := os.UserHomeDir()
	support := env("SUPPORT")
	if support == "" {
		if runtime.GOOS == "darwin" {
			support = filepath.Join(home, "Library", "Application Support", "FINDEROAD")
		} else {
			support = filepath.Join(home, ".finderoad")
		}
	}
	os.MkdirAll(support, 0o755)
	token := env("TOKEN")
	if token == "" {
		token = randID(16)
	}
	app = &App{support: support, token: token}

	os.Rename(filepath.Join(support, "engine.log"), filepath.Join(support, "engine.prev.log"))
	if f, err := os.OpenFile(filepath.Join(support, "engine.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
		log.SetOutput(io.MultiWriter(os.Stderr, f))
	}
	log.SetFlags(log.LstdFlags)
	app.loadSettings()
	app.upd = NewUpdater(support)
	app.thumbs = NewThumbs(support)

	ln := listen(env("PORT"))
	app.port = ln.Addr().(*net.TCPAddr).Port
	writeFileAtomic(filepath.Join(support, "port"), []byte(strconv.Itoa(app.port)), 0o644)
	writeFileAtomic(filepath.Join(support, "token"), []byte(token), 0o600)
	// 앱(Swift)은 이 한 줄로 주소를 알아요
	fmt.Printf("FINDEROAD_PORT=%d\n", app.port)
	log.Printf("FINDEROAD %s listening on 127.0.0.1:%d (pid %d, parent %d, %s/%s)", build, app.port, os.Getpid(), os.Getppid(), runtime.GOOS, runtime.GOARCH)

	app.watchLifecycle()
	go func() { defer recoverLog("autoLoop"); app.upd.autoLoop() }()

	srv := &http.Server{Handler: hostGuard(app.routes()), ErrorLog: log.Default(), ReadHeaderTimeout: 10 * time.Second}
	if err := srv.Serve(ln); err != nil {
		log.Printf("server stopped: %v", err)
	}
}

func listen(pref string) net.Listener {
	if p, err := strconv.Atoi(pref); err == nil && p > 0 {
		for i := 0; i < 20; i++ {
			if ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p)); err == nil {
				return ln
			}
			time.Sleep(150 * time.Millisecond)
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	return ln
}

// ---------------------------------------------------------------- 수명

// watchLifecycle: 엔진은 앱이 켜져 있는 동안만 돌아요.
//   - 종료 신호(TERM/INT/HUP)를 받으면 정리하고 바로 끝나요 (무시하지 않아요)
//   - 앱이 넘겨 준 표준 입력이 닫히면(= 앱이 끝났거나 사라짐) 끝나요
//   - 부모 프로세스가 바뀌면(= 앱이 사라짐) 끝나요
func (a *App) watchLifecycle() {
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	go func() {
		s := <-sig
		a.shutdownWhy(fmt.Sprintf("종료 신호(%v)", s))
	}()
	if env("STDIN_WATCH") == "1" {
		go func() {
			io.Copy(io.Discard, os.Stdin)
			a.shutdownWhy("앱과의 연결(표준 입력)이 닫힘")
		}()
	}
	if ppid, err := strconv.Atoi(env("PARENT_PID")); err == nil && ppid > 1 {
		go func() {
			for range time.Tick(time.Second) {
				if os.Getppid() != ppid {
					a.shutdownWhy("FINDEROAD 앱이 사라짐")
				}
			}
		}()
	}
}

var shutdownOnce sync.Once

func (a *App) shutdownWhy(why string) {
	shutdownOnce.Do(func() {
		log.Printf("shutdown: %s", why)
		if b, err := os.ReadFile(filepath.Join(a.support, "port")); err == nil && strings.TrimSpace(string(b)) == strconv.Itoa(a.port) {
			os.Remove(filepath.Join(a.support, "port"))
		}
		os.Exit(0)
	})
}

// nativeShell: Dock 에 보이는 맥 앱(app/shell/main.swift)이 엔진을 켰는지
func nativeShell() bool { return env("SHELL") == "native" }

// ---------------------------------------------------------------- 설정

func (a *App) settingsPath() string { return filepath.Join(a.support, "settings.json") }

func (a *App) loadSettings() {
	home, _ := os.UserHomeDir()
	a.settings = Settings{Appearance: "system", AutoUpdate: true, Scope: "all", UI: map[string]any{}}
	if b, err := os.ReadFile(a.settingsPath()); err == nil {
		_ = json.Unmarshal(b, &a.settings)
	} else if home != "" {
		// 처음 켤 때 고를 수 있는 폴더 몇 개를 미리 넣어 둬요 (선택한 폴더 모드일 때만 쓰여요)
		for _, n := range []string{"Desktop", "Documents", "Downloads"} {
			p := filepath.Join(home, n)
			if dirExists(p) {
				a.settings.Folders = append(a.settings.Folders, p)
			}
		}
	}
	if a.settings.UI == nil {
		a.settings.UI = map[string]any{}
	}
	if a.settings.Scope != "folders" {
		a.settings.Scope = "all"
	}
	switch a.settings.Appearance {
	case "light", "dark":
	default:
		a.settings.Appearance = "system"
	}
}

func (a *App) saveSettings() {
	b, _ := json.MarshalIndent(a.settings, "", "  ")
	writeFileAtomic(a.settingsPath(), b, 0o644)
}

func (a *App) Settings() Settings {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.settings
	s.Folders = append([]string(nil), a.settings.Folders...)
	s.UI = make(map[string]any, len(a.settings.UI))
	for k, v := range a.settings.UI {
		s.UI[k] = v
	}
	return s
}

func (a *App) settingsJSON() map[string]any {
	s := a.Settings()
	home, _ := os.UserHomeDir()
	folders := []map[string]any{}
	for _, f := range s.Folders {
		folders = append(folders, map[string]any{"path": f, "name": composeHangul(filepath.Base(f)), "display": displayPath(f), "exists": dirExists(f)})
	}
	return map[string]any{
		"appearance": s.Appearance,
		"autoUpdate": s.AutoUpdate,
		"scope":      s.Scope,
		"folders":    folders,
		"showHidden": s.ShowHidden,
		"ui":         s.UI,
		"home":       home,
		"build":      build,
		"platform":   runtime.GOOS + "/" + runtime.GOARCH,
		"shell":      env("SHELL"),
	}
}

func (a *App) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var in map[string]json.RawMessage
		if err := readJSON(r, &in); err != nil {
			jsonError(w, 400, "bad request")
			return
		}
		a.mu.Lock()
		for k, v := range in {
			switch k {
			case "appearance":
				var s string
				json.Unmarshal(v, &s)
				if s == "light" || s == "dark" || s == "system" {
					a.settings.Appearance = s
				}
			case "autoUpdate":
				json.Unmarshal(v, &a.settings.AutoUpdate)
			case "showHidden":
				json.Unmarshal(v, &a.settings.ShowHidden)
			case "scope":
				var s string
				json.Unmarshal(v, &s)
				if s == "all" || s == "folders" {
					a.settings.Scope = s
				}
			case "folders":
				var fs []string
				if json.Unmarshal(v, &fs) == nil {
					out := []string{}
					seen := map[string]bool{}
					for _, f := range fs {
						f = filepath.Clean(f)
						if filepath.IsAbs(f) && !seen[f] {
							seen[f] = true
							out = append(out, f)
						}
					}
					a.settings.Folders = out
				}
			case "ui":
				var ui map[string]any
				if json.Unmarshal(v, &ui) == nil {
					for kk, vv := range ui {
						a.settings.UI[kk] = vv
					}
				}
			}
		}
		a.saveSettings()
		a.mu.Unlock()
	}
	writeJSON(w, a.settingsJSON())
}

// ---------------------------------------------------------------- 라우트

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	sub, _ := fs.Sub(webFiles, "web")
	static := http.FileServer(http.FS(sub))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			if r.URL.Query().Get("t") != a.token {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusForbidden)
				io.WriteString(w, "<meta charset=utf-8><body style='font:15px -apple-system;padding:40px'>FINDEROAD 앱을 다시 실행해 주세요.</body>")
				return
			}
			b, _ := webFiles.ReadFile("web/index.html")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.Write(b)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/fonts/") {
			w.Header().Set("Cache-Control", "public, max-age=604800")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		static.ServeHTTP(w, r)
	})

	api := func(path string, h http.HandlerFunc) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			t := r.Header.Get("x-token")
			if t == "" {
				t = r.URL.Query().Get("t")
			}
			if t != a.token {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			defer func() {
				if v := recover(); v != nil {
					log.Printf("PANIC in %s: %v\n%s", path, v, debug.Stack())
					jsonError(w, 500, "앱 안에서 문제가 생겼어요. 다시 시도해 주세요")
				}
			}()
			h(w, r)
		})
	}

	api("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"ok": true, "build": build, "shell": env("SHELL"), "pid": os.Getpid(), "ppid": os.Getppid()})
	})
	api("/api/quit", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"ok": true})
		go func() { time.Sleep(200 * time.Millisecond); a.shutdownWhy("종료 요청") }()
	})
	api("/api/clientlog", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Items []string `json:"items"`
		}
		readJSON(r, &in)
		for i, x := range in.Items {
			if i >= 20 {
				break
			}
			if len(x) > 600 {
				x = x[:600]
			}
			log.Printf("client: %s", x)
		}
		writeJSON(w, map[string]any{"ok": true})
	})
	api("/api/state", func(w http.ResponseWriter, r *http.Request) {
		out := a.settingsJSON()
		out["update"] = a.upd.State()
		writeJSON(w, out)
	})
	api("/api/settings", a.handleSettings)

	api("/api/search", a.handleSearch)
	api("/api/info", a.handleInfo)
	api("/api/file", a.handleFile)
	api("/api/thumb", a.handleThumb)
	api("/api/text", a.handleText)

	api("/api/transfer", a.handleTransfer)
	api("/api/move-back", a.handleMoveBack)
	api("/api/trash", a.handleTrash)
	api("/api/delete", a.handleDelete)
	api("/api/clipboard", a.handleClipboard)
	api("/api/choose-folder", a.handleChooseFolder)
	api("/api/open", a.handleOpen)
	api("/api/reveal", a.handleReveal)

	api("/api/update/check", func(w http.ResponseWriter, r *http.Request) {
		a.upd.Check()
		writeJSON(w, a.upd.State())
	})
	api("/api/update/state", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.upd.State())
	})
	api("/api/update/install", func(w http.ResponseWriter, r *http.Request) {
		go func() { defer recoverLog("install"); a.upd.Install() }()
		writeJSON(w, map[string]any{"ok": true})
	})
	api("/api/update/restart", func(w http.ResponseWriter, r *http.Request) {
		if err := a.upd.Restart(); err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})
	return mux
}

// Only answer requests addressed to 127.0.0.1 / localhost (DNS rebinding protection).
func hostGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if host != "127.0.0.1" && host != "localhost" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------- utils

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": msg})
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(v)
}

func randID(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func writeFileAtomic(path string, b []byte, perm os.FileMode) {
	tmp := path + ".tmp"
	if os.WriteFile(tmp, b, perm) == nil {
		os.Rename(tmp, path)
	}
}

func fileExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func dirExists(p string) bool {
	if p == "" {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// displayPath: 홈 폴더는 ~ 로 줄여서 보여 줘요
func displayPath(p string) string {
	home, _ := os.UserHomeDir()
	p = composeHangul(p)
	if home != "" {
		if p == home {
			return "~"
		}
		if strings.HasPrefix(p, home+"/") {
			return "~" + p[len(home):]
		}
	}
	return p
}

func recoverLog(where string) {
	if v := recover(); v != nil {
		log.Printf("PANIC in %s: %v\n%s", where, v, debug.Stack())
	}
}

func init() {
	for ext, typ := range map[string]string{
		".js": "text/javascript", ".mjs": "text/javascript", ".css": "text/css; charset=utf-8", ".woff2": "font/woff2",
		".heic": "image/heic", ".heif": "image/heif", ".webp": "image/webp", ".avif": "image/avif", ".svg": "image/svg+xml",
		".mov": "video/quicktime", ".m4v": "video/x-m4v", ".mp4": "video/mp4", ".webm": "video/webm", ".mkv": "video/x-matroska",
		".m4a": "audio/mp4", ".aac": "audio/aac", ".mp3": "audio/mpeg", ".wav": "audio/wav", ".flac": "audio/flac",
		".aif": "audio/aiff", ".aiff": "audio/aiff", ".ogg": "audio/ogg", ".pdf": "application/pdf",
	} {
		mime.AddExtensionType(ext, typ)
	}
}
