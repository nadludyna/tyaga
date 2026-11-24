package main

// Portable: everything ТЯГА keeps lives in one hidden folder next to the program, tyaga-data.
// Copy the program somewhere and it starts clean; delete the folder and nothing of ТЯГА is
// left on the computer. What's in it, and why:
//
//	bin\              ffmpeg and Deno, downloaded once (only if they aren't installed);
//	bin\yt-dlp\       yt-dlp unpacked (it starts faster than the one-file one), kept up to
//	                  date at each start (ytdlp_unpacked.go)
//	webview\          the window's browser data: the switches you picked (Windows; WebView2
//	                  can't run without such a folder; its caches are deleted at every start)
//	login\            ТЯГА's own sign-in window, only for Chrome/Edge users (Windows)
//	tyaga.log         what happened in the last run, for when something goes wrong
//	youtube-signin.txt  which browser the YouTube sign-in is read from
//	queue.json        what was still downloading or waiting when ТЯГА closed: it's back in
//	                  the list next time, to resume (queue.go); gone once nothing is left
//	settings.json     what has to outlive the window: the downloads folder picked on the
//	                  page (the switches live in the window's own storage), and that the
//	                  "ТЯГА is in the tray" notice was shown (once is enough: every time annoys)
//
// Downloads go to Windows' Music and Videos unless another folder was picked (dirFor in jobs.go).
// The tools ТЯГА runs keep their own caches here too, not in the user's profile:
//	cache\yt-dlp\      yt-dlp's (YouTube's player code it worked out), --cache-dir
//	cache\deno\        Deno's (yt-dlp runs it for YouTube's JavaScript), DENO_DIR
// (Without them yt-dlp wrote to %USERPROFILE%\.cache\yt-dlp and Deno to %LOCALAPPDATA%\deno.)

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// makeDataDir makes tyaga-data next to the program. Nowhere else: a folder ТЯГА can't write to
// (Program Files) is said plainly at start, instead of keeping things in AppData (the owner:
// nothing of ТЯГА outside its own folder). See fail in main.go.
func makeDataDir() string {
	d := filepath.Join(appDir, "tyaga-data")
	if err := os.MkdirAll(d, 0o755); err != nil || !writable(d) {
		fail("ТЯГА can't write in its own folder:\n" + appDir + "\n\nMove tyaga.exe into a folder of your own (Desktop, Documents, a folder on another drive) and start it from there.")
	}
	hideDataDir(d)
	moveOldData(d)
	return d
}

func writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".write-test-*")
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(f.Name())
	return true
}

// moveOldData: builds before tyaga-data kept these next to the program (and, on Windows, in
// %LOCALAPPDATA%\TYAGA): move them in, so nothing is downloaded again and nothing stays behind.
func moveOldData(d string) {
	moveIn(filepath.Join(appDir, "bin"), filepath.Join(d, "bin"))
	moveIn(filepath.Join(appDir, "youtube-signin.txt"), filepath.Join(d, "youtube-signin.txt"))
	os.Remove(filepath.Join(appDir, "tyaga.log"))
	os.Remove(filepath.Join(appDir, ".tyaga-running"))
	moveOldWindowData(d)
}

// moveIn moves from to to, unless to is already there.
func moveIn(from, to string) {
	if _, err := os.Stat(to); err == nil {
		return
	}
	if _, err := os.Stat(from); err != nil {
		return
	}
	os.Rename(from, to)
}

// folderID tells copies of ТЯГА in different folders apart.
func folderID() string {
	sum := sha256.Sum256([]byte(strings.ToLower(appDir)))
	return hex.EncodeToString(sum[:8])
}

// settings are kept in tyaga-data/settings.json.
type settings struct {
	Out      string `json:"out,omitempty"`      // the downloads folder, if one was picked
	TrayTold bool   `json:"trayTold,omitempty"` // the "ТЯГА is in the tray" notice was shown once
}

var settingsMu sync.Mutex

// changeSettings reads settings.json, changes it and writes it back, so one setting saved
// doesn't wipe the others.
func changeSettings(f func(*settings)) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	s := loadSettings()
	f(&s)
	saveSettings(s)
}

func settingsFile() string { return filepath.Join(dataDir, "settings.json") }

func loadSettings() settings {
	var s settings
	if b, err := os.ReadFile(settingsFile()); err == nil {
		json.Unmarshal(b, &s)
	}
	return s
}

func saveSettings(s settings) {
	if b, err := json.MarshalIndent(s, "", "  "); err == nil {
		os.WriteFile(settingsFile(), b, 0o644)
	}
}
