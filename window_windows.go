//go:build windows

package main

// ТЯГА's own window on Windows (Wails, on the Edge WebView2 that comes with Windows 10/11).
// The page and its API are the same as in the browser; Wails just serves them to the window
// itself, with no network port at all. The window lives next to a tray icon:
//   - the window is the page's cards themselves: no frame and no background, the desktop
//     shows around them; the first card's top row (logo, buttons) and the gaps drag it;
//   - its size is fixed, so nothing jumps: the download list scrolls inside its card;
//   - the close button hides the window to the tray, downloads go on;
//   - a click on the tray icon brings the window back;
//   - Quit (tray menu or the page) stops everything and exits.

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/w32"
)

// place shows the window for the first time, in the middle of the screen (see runWindow).
var place = func() {}

//go:embed winres/icon.ico
var trayIcon []byte

// exitApp is how quit() leaves: through Wails when there's a window, so the tray icon
// disappears with it instead of hanging around until the mouse passes over it.
var exitApp = func() { os.Exit(0) }

// windowControl is what the page's own window buttons do (the window has no system title bar).
var windowControl = func(action string, n int) {}

// runWindow opens ТЯГА in its own window and returns when ТЯГА quits.
// A second launch from the same folder doesn't start a second ТЯГА: it brings up the window
// of the running one (a copy in another folder is a different ТЯГА and runs side by side).
func runWindow(token string) bool {
	var win *application.WebviewWindow
	showWin := func() {
		if win == nil {
			return
		}
		place()
		win.Show()
		if win.IsMinimised() {
			win.UnMinimise()
		}
		win.Focus()
	}

	app := application.New(application.Options{
		Name:        "ТЯГА",
		Description: "YouTube video and audio downloader",
		Assets: application.AssetOptions{
			Handler:        handler(token),
			DisableLogging: true,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               "tyaga-" + folderID(),
			OnSecondInstanceLaunch: func(application.SecondInstanceData) { showWin() },
		},
		Windows: application.WindowsOptions{
			WebviewUserDataPath:           webviewDataDir(),
			DisableQuitOnLastWindowClosed: true, // the window only hides; Quit is in the tray
		},
	})
	// Past this point this is the only ТЯГА from this folder: a second one has already exited above.
	openLog()
	logf("TYAGA %s is running. Files are saved to %s", version, outDir)
	exitApp = func() { app.Quit() }

	win = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:           "ТЯГА",
		Width:           winWidth,
		Height:          winHeight,
		DisableResize:   true,
		InitialPosition: application.WindowCentered,
		Hidden:          true,                    // shown by place() once the page has set the height, centred
		URL:             "/?t=" + token + "&w=1", // w: the page is the window (see html.app in index.html)
		Frameless:       true,
		BackgroundType:  application.BackgroundTypeTransparent,
		DevToolsEnabled: os.Getenv("TYAGA_DEVTOOLS") == "1",
		Permissions: map[application.PermissionType]application.Permission{
			application.PermissionNotifications: application.PermissionAllow,
		},
		Windows: application.WindowsWindow{
			Theme:                             application.SystemDefault,
			DisableFramelessWindowDecorations: true, // no frame or square shadow around the cards
			NonClientRegionSupport:            true, // the page's "app-region: drag" parts drag it like a title bar
		},
	})
	// The window's full height: the page's, or less on a small screen (or with big scaling),
	// so it doesn't go under the taskbar. The page asks for it when the list unfolds and for
	// just the cards' height when it folds; the top edge stays where it is.
	var fullHeight atomic.Int64
	fullHeight.Store(winHeight)
	win.OnWindowEvent(events.Common.WindowShow, func(*application.WindowEvent) {
		if sc, err := win.GetScreen(); err == nil && sc != nil && sc.WorkArea.Height > 0 {
			fullHeight.Store(int64(min(winHeight, sc.WorkArea.Height-16)))
		}
	})
	// At start the window stays hidden until the page has told it how tall it is (the list
	// folds when it's empty), then it appears in the middle of the screen. Centring it at its
	// first size and then shrinking it left it high up. If the page never asks, it shows anyway.
	var placed atomic.Bool
	place = func() {
		if placed.CompareAndSwap(false, true) {
			win.Center()
			win.Show()
			win.Focus()
		}
	}
	go func() { time.Sleep(3 * time.Second); place() }()
	// Growing keeps the top edge where it is, unless the bottom would go under the taskbar:
	// then the window moves up just enough (a folded window sits in the middle of the screen,
	// so unfolding it usually does need that).
	onScreen := func() {
		sc, err := win.GetScreen()
		if err != nil || sc == nil || sc.WorkArea.Height == 0 {
			return
		}
		b, wa := win.Bounds(), sc.WorkArea
		if bottom := wa.Y + wa.Height - 8; b.Y+b.Height > bottom {
			b.Y = max(wa.Y+8, bottom-b.Height)
			win.SetBounds(b)
		}
	}
	// × (or Alt+F4) hides the window to the tray, and says so: downloads go on there.
	toTray := func() {
		win.Hide()
		trayBalloon("ТЯГА is in the tray", "Downloads go on. Click the icon to open ТЯГА; Quit is in its menu.")
	}
	windowControl = func(action string, n int) {
		full := int(fullHeight.Load())
		switch action {
		case "min":
			win.Minimise()
		case "close":
			toTray()
		case "full":
			win.SetSize(winWidth, full)
			onScreen()
			place()
		case "height":
			win.SetSize(winWidth, max(120, min(n, full)))
			place()
		}
	}
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		toTray()
	})

	menu := app.NewMenu()
	menu.Add("Open ТЯГА").OnClick(func(*application.Context) { showWin() })
	menu.Add("Open the downloads folder").OnClick(func(*application.Context) {
		os.MkdirAll(outDir, 0o755)
		openFolder(outDir)
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) { askQuit(app, win, showWin) })

	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("ТЯГА")
	tray.SetMenu(menu)
	tray.OnClick(showWin)

	go setupTools()
	if err := app.Run(); err != nil {
		fail("Could not open the window: " + err.Error())
	}
	return true
}

// The window's size: the page's cards plus a little room for their shadows.
const winWidth, winHeight = 800, 800

// askQuit quits right away when nothing is going on, and asks first when something is.
func askQuit(app *application.App, win *application.WebviewWindow, showWin func()) {
	if !busy() {
		quit()
		return
	}
	showWin()
	d := app.Dialog.Question().
		SetTitle("ТЯГА").
		SetMessage("Something is still downloading. Quit anyway?\n\nUnfinished files resume when you download them again.")
	yes := d.AddButton("Quit")
	yes.OnClick(quit)
	no := d.AddButton("Keep downloading")
	d.SetDefaultButton(no)
	d.SetCancelButton(no)
	d.AttachToWindow(win)
	d.Show()
}

// folderID tells copies of ТЯГА in different folders apart.
func folderID() string {
	sum := sha256.Sum256([]byte(strings.ToLower(appDir)))
	return hex.EncodeToString(sum[:8])
}

// webviewDataDir is where WebView2 keeps its cache and the page's remembered switches.
// It's per folder too, so a test copy doesn't share switches with the normal one.
func webviewDataDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "TYAGA", "webview-"+folderID())
}

// ---------------------------------------------------------------------------
//  The sign-in window (login.go)
// ---------------------------------------------------------------------------

const canLogin = true

// Google's sign-in page, going on to YouTube once you're in.
const signInURL = "https://accounts.google.com/ServiceLogin?service=youtube&continue=https%3A%2F%2Fwww.youtube.com%2F"

// signedInJS runs on every page of the sign-in window: once YouTube is open with an account
// (SAPISID is the cookie YouTube's own scripts read to make signed-in requests), it tells ТЯГА.
// It keeps checking, since the cookie can arrive a moment after the page.
const signedInJS = `(function () {
  if (window.__tyaga) return; window.__tyaga = 1;
  var t = setInterval(function () {
    if (/(^|\.)youtube\.com$/.test(location.hostname) && /(^|; )(__Secure-3PAPISID|SAPISID)=/.test(document.cookie)) {
      clearInterval(t);
      window.chrome.webview.postMessage('tyaga:signed-in');
    }
  }, 500);
})()`

// runLogin is ТЯГА started with -login: one ordinary window with Google's sign-in page, in a
// web view with its own data folder, so its cookies can be read once it's closed. It closes by
// itself once you're signed in (or when you close it). Started again while open, it brings up
// the open one.
func runLogin() {
	var win *application.WebviewWindow
	var app *application.App
	app = application.New(application.Options{
		Name:   "ТЯГА",
		Assets: application.AssetOptions{Handler: http.NotFoundHandler(), DisableLogging: true},
		RawMessageHandler: func(_ application.Window, msg string, from *application.OriginInfo) {
			if msg == "tyaga:signed-in" && from != nil && strings.HasSuffix(strings.SplitN(strings.TrimPrefix(from.Origin, "https://"), "/", 2)[0], "youtube.com") {
				app.Quit()
			}
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "tyaga-login-" + folderID(),
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if win != nil {
					win.Show()
					win.Focus()
				}
			},
		},
		Windows: application.WindowsOptions{WebviewUserDataPath: loginDir()},
	})
	win = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:           "ТЯГА · sign in to YouTube",
		Width:           520,
		Height:          760,
		InitialPosition: application.WindowCentered,
		URL:             signInURL,
		Windows:         application.WindowsWindow{Theme: application.SystemDefault},
	})
	win.OnWindowEvent(events.Windows.WebViewNavigationCompleted, func(*application.WindowEvent) { win.ExecJS(signedInJS) })
	app.Run()
}

// loginDir is the sign-in window's web view data, per folder like the main window's.
func loginDir() string {
	return filepath.Join(filepath.Dir(webviewDataDir()), "login-"+folderID())
}

// ---------------------------------------------------------------------------
//  A message from the tray icon
// ---------------------------------------------------------------------------

var findWindowEx = syscall.NewLazyDLL("user32.dll").NewProc("FindWindowExW")

// trayBalloon shows a Windows notification from ТЯГА's tray icon. Wails has no call for it,
// so this finds the icon itself: Wails keeps it on a hidden message window of its window
// class, with id 1 for the first (and only) tray icon. Windows says no to every other window.
func trayBalloon(title, text string) {
	class := w32.MustStringToUTF16Ptr("WailsWebviewWindow")
	var after uintptr
	for i := 0; i < 16; i++ {
		h, _, _ := findWindowEx.Call(uintptr(w32.HWND_MESSAGE), after, uintptr(unsafe.Pointer(class)), 0)
		if h == 0 {
			return
		}
		after = h
		nid := w32.NOTIFYICONDATA{HWnd: w32.HWND(h), UID: 1, UFlags: w32.NIF_INFO, DwInfoFlags: w32.NIIF_INFO}
		nid.CbSize = uint32(unsafe.Sizeof(nid))
		copy(nid.SzInfoTitle[:len(nid.SzInfoTitle)-1], w32.MustStringToUTF16(title))
		copy(nid.SzInfo[:len(nid.SzInfo)-1], w32.MustStringToUTF16(text))
		if w32.ShellNotifyIcon(w32.NIM_MODIFY, &nid) {
			return
		}
	}
}
