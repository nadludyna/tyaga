package main

// Signing in to YouTube from ТЯГА itself, so nobody has to export cookies by hand.
//
// "Sign in to YouTube" on the page starts a second copy of ТЯГА with -login: just a window
// with Google's sign-in page, in a web view with its own data folder (runLogin, Windows
// only). It closes by itself once YouTube opens with an account (or when you close it); then
// yt-dlp reads the cookies from that data folder (it's
// the same Chromium format as Chrome's) and writes them out as youtube-cookies.txt, which
// every download already uses. The window has to be gone first: while it's open, the web
// view keeps its cookie database locked.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var login struct {
	sync.Mutex
	open bool   // the sign-in window is open
	msg  string // what came of the last try, for the page ("" = nothing to say)
}

// startLogin opens the sign-in window, or brings it up if it's already open
// (the second copy finds the first one and exits, see runLogin).
func startLogin() {
	if !canLogin {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, "-login")
	cmd.Dir = appDir
	if err := cmd.Start(); err != nil {
		logf("Sign-in window: %v", err)
		setLoginMsg("Couldn't open the sign-in window.")
		return
	}
	login.Lock()
	already := login.open
	login.open, login.msg = true, ""
	login.Unlock()
	if already {
		go cmd.Wait()
		return
	}
	go func() {
		cmd.Wait()
		msg := takeCookies()
		login.Lock()
		login.open, login.msg = false, msg
		login.Unlock()
	}()
}

func setLoginMsg(s string) {
	login.Lock()
	login.msg = s
	login.Unlock()
}

// takeCookies turns the sign-in window's cookies into youtube-cookies.txt.
// It returns what to tell the person, or "" when it worked.
func takeCookies() string {
	if !tools.ready() {
		return "ТЯГА is still getting ready. Sign in again once it's done."
	}
	if !fileExists(loginProfile()) {
		return "" // the window was closed before anything loaded: nothing to say
	}
	tmp := cookiesFile + ".new"
	os.Remove(tmp)
	// No link: yt-dlp complains that a link is missing, but it still reads the cookies and
	// saves them to the --cookies file on the way out.
	cmd := exec.Command(tools.ytdlp, "--cookies-from-browser", "chrome:"+loginProfile(), "--cookies", tmp)
	hideWindow(cmd)
	out, _ := cmd.CombinedOutput()
	data, _ := os.ReadFile(tmp)
	if !hasSignIn(data) {
		os.Remove(tmp)
		if s := string(out); strings.Contains(s, "ERROR") && !strings.Contains(s, "provide at least one URL") {
			logf("Sign-in: yt-dlp couldn't read the cookies:\n%s", s)
			return "Couldn't take the sign-in from that window. Try once more."
		}
		return "Not signed in: the window was closed before YouTube opened with your account."
	}
	if err := os.Rename(tmp, cookiesFile); err != nil {
		os.Remove(tmp)
		logf("Sign-in: %v", err)
		return "Couldn't save the sign-in next to the program."
	}
	logf("Signed in to YouTube.")
	return ""
}

// hasSignIn: does a cookies.txt hold a signed-in YouTube session (not just a visit)?
func hasSignIn(data []byte) bool {
	for _, l := range strings.Split(string(data), "\n") {
		f := strings.Split(strings.TrimPrefix(l, "#HttpOnly_"), "\t")
		if len(f) == 7 && strings.HasSuffix(f[0], "youtube.com") &&
			(f[5] == "SAPISID" || f[5] == "__Secure-3PSID" || f[5] == "LOGIN_INFO") {
			return true
		}
	}
	return false
}

// signedIn: is there a cookies file with a signed-in YouTube session? Read again only when the
// file changes (yt-dlp rewrites it after every download).
var signedCache struct {
	sync.Mutex
	stamp string
	in    bool
}

func signedIn() bool {
	st, err := os.Stat(cookiesFile)
	if err != nil {
		return false
	}
	stamp := st.ModTime().String() + "|" + filepath.Base(cookiesFile)
	signedCache.Lock()
	defer signedCache.Unlock()
	if signedCache.stamp != stamp {
		data, _ := os.ReadFile(cookiesFile)
		signedCache.stamp, signedCache.in = stamp, hasSignIn(data)
	}
	return signedCache.in
}

// logout forgets the sign-in: the cookies file and the sign-in window's data.
func logout() {
	login.Lock()
	open := login.open
	login.msg = ""
	login.Unlock()
	os.Remove(cookiesFile)
	if canLogin && !open {
		os.RemoveAll(loginDir())
	}
	logf("Signed out of YouTube.")
}

// loginProfile is the sign-in window's browser profile, where yt-dlp finds the cookies.
func loginProfile() string { return filepath.Join(loginDir(), "EBWebView", "Default") }

// loginState is what the page gets about signing in.
func loginState() map[string]any {
	login.Lock()
	defer login.Unlock()
	return map[string]any{"can": canLogin, "open": login.open, "in": signedIn(), "msg": login.msg}
}
