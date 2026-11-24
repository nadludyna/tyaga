// ТЯГА (TYAGA) downloads video and audio from YouTube with yt-dlp.
//
// How it works: one page (index.html) and a small API behind it. The page sends links,
// the program runs yt-dlp for them one at a time and reads its output, and the page asks
// for progress every 0.7 s.
// On Windows the page lives in ТЯГА's own window with a tray icon (window_windows.go),
// served straight to the window with no network port. ТЯГА is a Windows app; on Linux
// (the test stand, stand/) the program starts a tiny web server on this computer and the
// page opens in a browser, and it quits by itself once the page is closed and the downloads
// are finished (see app.go).
//
// Only this computer can reach the server, and only the page that knows the token.
package main

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

//go:embed index.html
var page []byte

// The tray icon's menu on Windows, a little page of its own (window_windows.go).
//
//go:embed menu.html
var menuPage []byte

// The page's font, Onest (SIL Open Font License, fonts/OFL.txt): inside the program, so the
// window looks the same on every computer and needs no internet.
//
//go:embed fonts/*.woff2
var fontFiles embed.FS

// windowHidden: ТЯГА's window is in the tray or minimised (Windows). A hidden WebView2 page
// isn't told it's hidden, so the page asks, and stops its animations meanwhile.
var windowHidden = func() bool { return false }

// trayAction is what the tray menu's buttons do (Windows only).
var trayAction = func(string) any { return nil }

// pickFolder asks where downloads should go, with Windows' own folder dialog. Only ТЯГА's
// window has one (window_windows.go); "" = nothing picked.
var pickFolder func() string

// version is set when a release is built (see .github/workflows/release.yml).
var version = "dev"

var (
	appDir      string // folder the program lives in
	dataDir     string // everything ТЯГА keeps (data.go)
	binDir      string // yt-dlp, ffmpeg and Deno get downloaded here
	cacheDir    string // the caches of yt-dlp and Deno (toolEnv, --cache-dir): in tyaga-data, not the profile
	cookiesFile string // youtube-cookies.txt: from the sign-in window, or put next to the program by hand
	outDir      string // the downloads folder picked (or -out); see dirFor
	outPicked   bool   // a folder was picked (or -out given): else Windows' Music and Videos
	lastAudio   = true // what was downloaded last (the tray's "open the folder"), and in which language
	lastLang    = "ru"
)

func main() {
	consoleUTF8()
	home, _ := os.UserHomeDir()
	// The downloads folder when one is picked or given (sorted into Видео and Аудио inside it, see
	// typeDir). Not picked: Windows' own Music and Videos (dirFor). Until build 28 the default
	// was Downloads\YouTube, until build 45 Downloads\ТЯГА.
	out := flag.String("out", filepath.Join(home, "Downloads", "ТЯГА"), "folder for downloads")
	noBrowser := flag.Bool("no-browser", false, "no window and no browser: just the server, its address is in the log")
	loginWin := flag.Bool("login", false, "only the window for signing in to YouTube (the page opens it)")
	flag.Parse()

	appDir = programDir()
	dataDir = makeDataDir()
	binDir = filepath.Join(dataDir, "bin")
	cacheDir = filepath.Join(dataDir, "cache")
	cookiesFile = filepath.Join(dataDir, "youtube-cookies.txt")
	if byHand := filepath.Join(appDir, "youtube-cookies.txt"); fileExists(byHand) {
		cookiesFile = byHand
	}
	outDir = *out
	// A folder picked on the page (settings.json) counts, unless -out was given (the stand).
	outGiven := false
	flag.Visit(func(f *flag.Flag) { outGiven = outGiven || f.Name == "out" })
	// The old default (Downloads\YouTube) remembered as if picked counts as not picked: it moves
	// to the new one. What's in the old folder stays there.
	if s := loadSettings(); !outGiven && s.Out != "" && filepath.IsAbs(s.Out) && !strings.EqualFold(filepath.Clean(s.Out), filepath.Join(home, "Downloads", "YouTube")) {
		outDir, outPicked = s.Out, true
	}
	outPicked = outPicked || outGiven

	// The sign-in window (login.go): started by ТЯГА itself, closes when you close it.
	if *loginWin {
		runLogin()
		return
	}

	// Windows: ТЯГА's own window. It returns when ТЯГА quits.
	if !*noBrowser && runWindow(randomToken()) {
		return
	}

	// Launched again while already running: just bring up the page of the running one.
	if alreadyRunning() {
		return
	}
	openLog()

	// Local server: 127.0.0.1 only, the OS picks a free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fail("Could not start the server: " + err.Error())
	}
	localToken := randomToken()
	address := fmt.Sprintf("http://%s/?t=%s", ln.Addr(), localToken)

	os.WriteFile(instanceFile(), []byte(address), 0o600)

	logf("TYAGA %s is running. Files are saved to %s", version, outDir)
	logf("If the browser didn't open: %s", address)
	logf("To quit: just close the page.")

	loadQueue()
	go keepQueue()
	go setupTools()
	if !*noBrowser {
		openURL(address)
		go quitWhenAbandoned() // with -no-browser nobody promised to keep a page open
	}
	fail(http.Serve(ln, handler(localToken)).Error())
}

// programDir is the folder with the exe. Under "go run" the exe sits in a temp folder,
// so the current folder is used instead.
func programDir() string {
	exe, err := os.Executable()
	if err == nil {
		exe, _ = filepath.EvalSymlinks(exe)
		dir := filepath.Dir(exe)
		if !strings.Contains(dir, "go-build") {
			return dir
		}
	}
	dir, _ := os.Getwd()
	return dir
}

// fail shows the error (a message box on Windows, where there's no console) and exits.
func fail(msg string) {
	logf("%s", msg)
	if dataDir != "" { // (not made yet: a folder ТЯГА can't write to)
		os.Remove(instanceFile())
	}
	showError(msg)
	os.Exit(1)
}

func randomToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------------------------------------------------------------------------
//  Server
// ---------------------------------------------------------------------------

// handler serves the page and its API.
func handler(token string) http.Handler {
	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(v)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The page itself is served to anyone: there is nothing secret in it, the token comes in the URL.
		if r.URL.Path == "/" || r.URL.Path == "/menu" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			if r.URL.Path == "/menu" {
				w.Write(menuPage)
			} else {
				w.Write(page)
			}
			return
		}

		if strings.HasPrefix(r.URL.Path, "/fonts/") { // nothing secret either
			b, err := fontFiles.ReadFile(strings.TrimPrefix(r.URL.Path, "/"))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "font/woff2")
			w.Header().Set("Cache-Control", "max-age=31536000")
			w.Write(b)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") { // favicon and such don't count as break-in attempts
			http.NotFound(w, r)
			return
		}

		// Everything else only for the page that knows the token. Normal requests send it in a
		// header (another website can't do that); sendBeacon can't, so /api/bye has it in the URL.
		got := r.Header.Get("X-Token")
		if r.URL.Path == "/api/bye" || r.URL.Path == "/api/cover" { // sendBeacon and <img> can't send headers
			got = r.URL.Query().Get("t")
		}
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		if r.Method == http.MethodGet {
			switch r.URL.Path {
			case "/api/state":
				pageSeen()
				status, problem, ready := tools.state()
				hidden := windowHidden() // before mu: it asks the window's thread
				mu.Lock()
				defer mu.Unlock()
				writeJSON(w, map[string]any{
					"version": version, "ready": ready, "setup": status, "problem": problem,
					"out": outDir, "outName": folderName(outDir), "picked": outPicked, "canPick": pickFolder != nil,
					"musicName": folderName(dirFor(true, lastLang)), "videoName": folderName(dirFor(false, lastLang)),
					"jobs": jobs, "login": loginState(), "hidden": hidden,
				})
			case "/api/cover":
				id, _ := strconv.Atoi(r.URL.Query().Get("id"))
				pic := jobCover(id)
				if pic == nil {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", http.DetectContentType(pic))
				w.Header().Set("Cache-Control", "max-age=86400") // the page adds ?v= when it changes
				w.Write(pic)
			default:
				http.NotFound(w, r)
			}
			return
		}

		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var body struct {
			Text string
			Options
			ID int
			N  int // file number within a download
		}
		json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body)

		switch r.URL.Path {
		case "/api/add":
			links := strings.Fields(body.Text)
			if len(links) == 0 {
				writeJSON(w, map[string]string{"error": "Add at least one link."})
				return
			}
			for _, l := range links {
				u, err := url.Parse(l)
				if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
					writeJSON(w, map[string]string{"error": "This doesn't look like a link: " + l})
					return
				}
			}
			o := cleanOptions(body.Options) // only known values get through
			if _, _, ok := cutRange(o.From, o.To); !ok {
				writeJSON(w, map[string]string{"error": "The fragment times look wrong. Examples: 1:20, 0:45, 1:02:03; \"to\" after \"from\"."})
				return
			}
			addJobs(links, o)
		case "/api/inspect": // what a link is, before it's downloaded (inspect.go)
			u, err := url.Parse(strings.TrimSpace(body.Text))
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				writeJSON(w, Facts{Error: "not a link"})
				return
			}
			writeJSON(w, inspect(u.String()))
			return
		case "/api/peek": // a link's title and channel in a moment, before the full look (inspect.go)
			writeJSON(w, peek(strings.TrimSpace(body.Text)))
			return
		case "/api/cancel":
			cancelJob(body.ID)
		case "/api/retry":
			retryJob(body.ID)
		case "/api/login":
			startLogin()
		case "/api/logout":
			logout()
		case "/api/clear":
			writeJSON(w, map[string]int{"n": clearFinished()})
			return
		case "/api/unclear": // Undo after Clear finished
			unclear()
		case "/api/std": // back to Windows' own Music and Videos
			mu.Lock()
			outPicked = false
			mu.Unlock()
			changeSettings(func(s *settings) { s.Out = "" })
			logf("Downloads go to Music and Videos again")
		case "/api/folder": // pick another downloads folder
			writeJSON(w, map[string]string{"out": chooseFolder()})
			return
		case "/api/bye":
			pageClosed()
		case "/api/window": // the buttons of ТЯГА's own window, which has no system title bar
			windowControl(body.Text, body.N)
		case "/api/tray": // the tray menu's buttons
			writeJSON(w, trayAction(body.Text))
			return
		case "/api/quit":
			writeJSON(w, map[string]bool{"ok": true})
			go func() { time.Sleep(200 * time.Millisecond); quit() }() // let the answer reach the page first
			return
		case "/api/open": // the folder of the kind the page is set to (audio: Music, video: Videos)
			mu.Lock()
			dir := dirFor(body.Text == "audio", body.Lang)
			mu.Unlock()
			os.MkdirAll(dir, 0o755)
			openFolder(dir)
		case "/api/reveal":
			if path, ok := jobFile(body.ID, body.N); ok && fileExists(path) {
				revealFile(path)
			}
		case "/api/play": // double-click on a finished row: the file opens in its player
			// Only sound and video: yt-dlp also takes plain links to any file, and a "song" that's
			// really an .exe or a .bat would run (security check, build 47). Others: Show file.
			if path, ok := jobFile(body.ID, body.N); ok && fileExists(path) && isMedia(path) {
				openFile(path)
			}
		case "/api/copyall": // every finished file onto the clipboard at once
			paths := finishedFiles()
			writeJSON(w, map[string]any{"ok": len(paths) > 0 && copyFiles(paths), "n": len(paths)})
			return
		case "/api/copyfile": // the finished file onto the clipboard, to paste into a chat
			path, ok := jobFile(body.ID, body.N)
			writeJSON(w, map[string]bool{"ok": ok && fileExists(path) && copyFile(path)})
			return
		default:
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	})
}

// cleanOptions keeps only the values ТЯГА knows: what comes from the page (or from queue.json)
// can't put anything else on yt-dlp's command line. The fragment times are checked by cutRange.
func cleanOptions(o Options) Options {
	if o.Mode != "audio" {
		o.Mode = "video"
	}
	if o.Format != "mp3" {
		o.Format = "m4a"
	}
	if !slices.Contains([]int{4320, 2160, 1440, 1080, 720, 480, 360, 240, 144}, o.Quality) {
		o.Quality = 0
	}
	if o.Lang != "en" {
		o.Lang = "ru"
	}
	if _, _, ok := cutRange(o.From, o.To); !ok {
		o.From, o.To = "", ""
	}
	return o
}

// isMedia: a file a player opens (what yt-dlp makes of a video or a song), not a program.
func isMedia(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".m4a", ".mp3", ".webm", ".mkv", ".mov", ".opus", ".ogg", ".oga", ".flac", ".wav", ".aac", ".m4v":
		return true
	}
	return false
}

// downloadsDir is where downloads go now (it can change while ТЯГА runs).
func downloadsDir() string {
	mu.Lock()
	defer mu.Unlock()
	return outDir
}

// chooseFolder shows the folder dialog and, if a folder was picked, sends the next downloads
// there and remembers it. What's downloading now finishes where it started. Returns the
// folder's short name for the page, or "" if nothing changed.
func chooseFolder() string {
	if pickFolder == nil {
		return ""
	}
	dir := pickFolder()
	if dir == "" || !filepath.IsAbs(dir) {
		return ""
	}
	mu.Lock()
	outDir, outPicked = dir, true
	mu.Unlock()
	changeSettings(func(s *settings) { s.Out = dir })
	logf("Downloads now go to %s", dir)
	return folderName(dir)
}

// folderName is how the page names a folder: its last two parts ("Downloads › YouTube"),
// not the whole path.
func folderName(dir string) string {
	dir = filepath.Clean(dir)
	base, parent := filepath.Base(dir), filepath.Base(filepath.Dir(dir))
	if parent == "" || parent == "." || parent == string(filepath.Separator) || strings.HasSuffix(parent, ":\\") || parent == dir {
		return base
	}
	return parent + " › " + base
}
