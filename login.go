package main

// Signing in to YouTube from ТЯГА itself, so nobody has to export cookies by hand.
//
// "Sign in to YouTube" on the page, in this order:
//  1. The browser you use (whatever opens links): if yt-dlp can read its cookies (Brave,
//     Firefox, most others) and you're signed in to YouTube there, ТЯГА remembers that
//     browser and reads its sign-in afresh before every download. A one-off copy would go
//     stale: YouTube keeps changing the cookies of an open browser, and an old copy stops
//     working after a while.
//  2. Not signed in there: Google's sign-in page opens in that browser (your saved password is
//     right there), and ТЯГА checks every few seconds until you're in. Nothing to close.
//  3. A browser yt-dlp can't read (Chrome and Edge lock their cookies on Windows since 2024):
//     ТЯГА's own sign-in window (runLogin, -login), a web view with its own data folder. It
//     closes by itself once YouTube opens with an account; then yt-dlp reads that folder (the
//     same Chromium format as Chrome's) into youtube-cookies.txt, which downloads use. The
//     window has to be gone first: while it's open, its cookie database is locked.
// A youtube-cookies.txt put next to the program by hand works too, as before.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Google's sign-in page, going on to YouTube once you're in.
const signInURL = "https://accounts.google.com/ServiceLogin?service=youtube&continue=https%3A%2F%2Fwww.youtube.com%2F"

var login struct {
	sync.Mutex
	open bool   // signing in is under way (a window, or waiting for the browser)
	msg  string // what came of the last try, for the page ("" = nothing to say)
	win  bool   // ТЯГА's own sign-in window is open
	step string // while open: "check" (reading the browser's sign-in), "page" (Google's page is open in the browser), "window" (ТЯГА's own window)
}

func setStep(s string) {
	login.Lock()
	login.step = s
	login.Unlock()
}

// startLogin signs in (see the top of the file). Clicked again while that's under way, it only
// brings ТЯГА's own sign-in window up again if that's what is open. It never opens YouTube in
// the browser a second time: a click while ТЯГА is still checking the browser's sign-in opened
// YouTube for someone who was signed in already (the owner, 22.11).
func startLogin() {
	if !canLogin {
		return
	}
	login.Lock()
	already, win := login.open, login.win
	if !already {
		login.open, login.msg = true, ""
	}
	login.Unlock()
	if already {
		if win {
			startLoginWindow() // the second copy finds the open one and brings it up
		}
		return
	}
	go func() {
		msg := signIn()
		login.Lock()
		login.open, login.msg = false, msg
		login.Unlock()
	}()
}

// signIn does the work of startLogin and returns what to tell the person ("" = signed in).
func signIn() string {
	if !tools.ready() {
		return "ТЯГА is still getting ready. Sign in again once it's done."
	}
	if b := defaultBrowser(); b != "" && browserReadable(b) {
		setStep("check")
		in, ok := browserSignedIn(b)
		if ok && !in {
			// Readable, but not signed in to YouTube there: the sign-in page in that browser.
			setStep("page")
			openURL(signInURL)
			for end := time.Now().Add(5 * time.Minute); time.Now().Before(end); {
				time.Sleep(3 * time.Second)
				if in, _ = browserSignedIn(b); in {
					break
				}
			}
			if !in {
				return "Didn't see you sign in to YouTube in " + browserName(b) + ". Sign in there, then click Sign in again."
			}
		}
		if in {
			os.WriteFile(signinFile(), []byte(b+"\n"), 0o644)
			logf("Signed in to YouTube: the sign-in comes from %s.", browserName(b))
			return ""
		}
		// Couldn't read it after all: ТЯГА's own window below.
	}
	cmd := startLoginWindow()
	if cmd == nil {
		return "Couldn't open the sign-in window."
	}
	login.Lock()
	login.win, login.step = true, "window"
	login.Unlock()
	cmd.Wait()
	login.Lock()
	login.win = false
	login.Unlock()
	return takeCookies()
}

func startLoginWindow() *exec.Cmd {
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	cmd := exec.Command(exe, "-login")
	cmd.Dir = appDir
	if err := cmd.Start(); err != nil {
		logf("Sign-in window: %v", err)
		return nil
	}
	return cmd
}

// browserReadable: Chrome and Edge are left out: on Windows their cookies can only be read by
// the browser itself (yt-dlp gets nothing), so trying would just wait for nothing.
func browserReadable(b string) bool { return b != "chrome" && b != "edge" }

// browserSignedIn reads the browser's cookies with yt-dlp (into a temporary file, removed
// right away) and says whether YouTube is signed in there. ok is false when they couldn't
// be read at all.
func browserSignedIn(b string) (in, ok bool) {
	tmp := filepath.Join(os.TempDir(), "tyaga-signin-check.txt")
	defer os.Remove(tmp)
	// No link: yt-dlp complains that a link is missing, but it still reads the cookies and
	// saves them to the --cookies file on the way out.
	cmd := exec.Command(tools.ytdlp, append(cacheArgs(), "--cookies-from-browser", b, "--cookies", tmp)...)
	cmd.Env = toolEnv()
	hideWindow(cmd)
	out, _ := cmd.CombinedOutput()
	s := string(out)
	if !strings.Contains(s, "Extracted ") || strings.Contains(s, "could not be decrypted") {
		logf("Sign-in: yt-dlp couldn't read %s's cookies:\n%s", b, s)
		return false, false
	}
	data, _ := os.ReadFile(tmp)
	return hasSignIn(data), true
}

// browserSource is the browser the sign-in comes from ("" = none, see signinFile).
func browserSource() string {
	b, _ := os.ReadFile(signinFile())
	return strings.TrimSpace(string(b))
}

// signinFile remembers which browser to read the sign-in from.
func signinFile() string { return filepath.Join(dataDir, "youtube-signin.txt") }

func browserName(b string) string {
	if n, ok := map[string]string{"brave": "Brave", "firefox": "Firefox", "chrome": "Chrome", "edge": "Edge",
		"vivaldi": "Vivaldi", "opera": "Opera", "chromium": "Chromium", "whale": "Whale"}[b]; ok {
		return n
	}
	return b
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
	cmd := exec.Command(tools.ytdlp, append(cacheArgs(), "--cookies-from-browser", "chrome:"+loginProfile(), "--cookies", tmp)...)
	cmd.Env = toolEnv()
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

// signedIn: is the sign-in taken from a browser, or is there a cookies file with a signed-in YouTube session? Read again only when the
// file changes (yt-dlp rewrites it after every download).
var signedCache struct {
	sync.Mutex
	stamp string
	in    bool
}

func signedIn() bool {
	if browserSource() != "" {
		return true
	}
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

// logout forgets the sign-in: which browser it came from, the cookies file and the sign-in
// window's data. (Your browser stays signed in, of course.)
func logout() {
	login.Lock()
	open := login.open
	login.msg = ""
	login.Unlock()
	os.Remove(signinFile())
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
	via := ""
	if b := browserSource(); b != "" {
		via = browserName(b)
	}
	return map[string]any{"can": canLogin, "open": login.open, "step": login.step, "in": signedIn(), "via": via, "msg": login.msg}
}
