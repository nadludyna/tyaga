package main

// Portable: everything ТЯГА keeps lives in one hidden folder next to the program, tyaga-data.
// Copy the program somewhere and it starts clean; delete the folder and nothing of ТЯГА is
// left on the computer. What's in it, and why:
//
//	bin\              yt-dlp, ffmpeg and Deno, downloaded once (only if they aren't installed)
//	webview\          the window's browser data: the switches you picked (Windows; WebView2
//	                  can't run without such a folder; its caches are deleted at every start)
//	login\            ТЯГА's own sign-in window, only for Chrome/Edge users (Windows)
//	tyaga.log         what happened in the last run, for when something goes wrong
//	youtube-signin.txt  which browser the YouTube sign-in is read from
//
// Downloads go to Downloads\YouTube, of course. Only when the program's folder can't be
// written to (Program Files) is the data kept in the user's own app data folder instead.

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

func makeDataDir() string {
	d := filepath.Join(appDir, "tyaga-data")
	if err := os.MkdirAll(d, 0o755); err != nil || !writable(d) {
		base, err := os.UserCacheDir() // %LOCALAPPDATA% on Windows
		if err != nil {
			base = os.TempDir()
		}
		d = filepath.Join(base, "TYAGA", folderID())
		os.MkdirAll(d, 0o755)
	}
	if filepath.Dir(d) == appDir {
		hideDataDir(d)
	}
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
