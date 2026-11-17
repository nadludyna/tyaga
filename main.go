// ТЯГА (TYAGA) downloads video and audio from YouTube with yt-dlp, through a page in your browser.
//
// How it works: the program starts a tiny web server on your computer and opens a page
// in the browser. The page sends links to the server, the server runs yt-dlp for them one
// at a time and reads its output, and the page asks for progress every 0.7 s.
// On Windows there's no console window: TYAGA quits by the Quit button on the page,
// or by itself once the page is closed and the downloads are finished (see app.go).
//
// Only this computer can reach the server, and only the page that knows the token.
package main

import (
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//go:embed index.html
var page []byte

// version is set when a release is built (see .github/workflows/release.yml).
var version = "dev"

var (
	appDir      string // folder the program lives in
	binDir      string // yt-dlp, ffmpeg and Deno get downloaded here
	cookiesFile string // optional youtube-cookies.txt next to the program
	outDir      string // where downloads are saved
)

func main() {
	consoleUTF8()
	home, _ := os.UserHomeDir()
	out := flag.String("out", filepath.Join(home, "Downloads", "YouTube"), "folder for downloads")
	noBrowser := flag.Bool("no-browser", false, "don't open the browser on start")
	flag.Parse()

	appDir = programDir()
	binDir = filepath.Join(appDir, "bin")
	cookiesFile = filepath.Join(appDir, "youtube-cookies.txt")
	outDir = *out

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
	logf("To quit: the Quit button on the page, or just close the page.")

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
	os.Remove(instanceFile())
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
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.Write(page)
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
				mu.Lock()
				defer mu.Unlock()
				writeJSON(w, map[string]any{
					"version": version, "ready": ready, "setup": status, "problem": problem,
					"out": outDir, "jobs": jobs,
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
			o := body.Options // only known values get through
			if o.Mode != "audio" {
				o.Mode = "video"
			}
			if o.Format != "mp3" {
				o.Format = "m4a"
			}
			if o.Quality != 1440 && o.Quality != 1080 && o.Quality != 720 {
				o.Quality = 0
			}
			if _, _, ok := cutRange(o.From, o.To); !ok {
				writeJSON(w, map[string]string{"error": "The fragment times look wrong. Examples: 1:20, 0:45, 1:02:03; \"to\" after \"from\"."})
				return
			}
			addJobs(links, o)
		case "/api/cancel":
			cancelJob(body.ID)
		case "/api/retry":
			retryJob(body.ID)
		case "/api/clear":
			clearFinished()
		case "/api/bye":
			pageClosed()
		case "/api/quit":
			writeJSON(w, map[string]bool{"ok": true})
			go func() { time.Sleep(200 * time.Millisecond); quit() }() // let the answer reach the page first
			return
		case "/api/open":
			os.MkdirAll(outDir, 0o755)
			openFolder(outDir)
		case "/api/reveal":
			if path, ok := jobFile(body.ID, body.N); ok && fileExists(path) {
				revealFile(path)
			}
		default:
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	})
}
