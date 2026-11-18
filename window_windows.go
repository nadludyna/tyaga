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
//   - Quit in the tray menu stops everything and exits.

import (
	_ "embed"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows/registry"
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
	trimWebview(webviewDataDir())
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
			AdditionalBrowserArgs:         lightWebview,
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
			want := max(120, min(n, full))
			win.SetSize(winWidth, want)
			onScreen()
			place()
			if b := win.Bounds(); b.Height != want {
				logf("Window: the page asked for %d (limit %d) and got %d", n, full, b.Height)
			}
		}
	}
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		toTray()
	})

	// The tray icon's menu (right click): a little window of ТЯГА's own (menu.html) in the same
	// look as the main one, instead of Windows' grey menu. It opens by the mouse, over the
	// taskbar, and goes away when you click anywhere else. Quitting while something downloads
	// asks in the menu itself.
	menuWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:          "ТЯГА",
		Width:          menuWidth,
		Height:         menuHeight,
		DisableResize:  true,
		Hidden:         true,
		AlwaysOnTop:    true,
		URL:            "/menu?t=" + token,
		Frameless:      true,
		BackgroundType: application.BackgroundTypeTransparent,
		Windows: application.WindowsWindow{
			Theme:                             application.SystemDefault,
			DisableFramelessWindowDecorations: true,
			HiddenOnTaskbar:                   true,
		},
	})
	// Shown: when. Windows sends a right click twice (button up, then "context menu"), and the
	// taskbar takes the focus back for a moment right after the click; neither may close it.
	var menuShown atomic.Int64
	menuWin.OnWindowEvent(events.Common.WindowLostFocus, func(*application.WindowEvent) {
		if time.Since(time.UnixMilli(menuShown.Load())) > 500*time.Millisecond {
			menuWin.Hide()
		}
	})
	showMenu := func() {
		if time.Since(time.UnixMilli(menuShown.Load())) < 400*time.Millisecond {
			return
		}
		menuShown.Store(time.Now().UnixMilli())
		hwnd := w32.HWND(uintptr(menuWin.NativeWindow()))
		if hwnd == 0 {
			logf("Tray menu: its window isn't there")
		} else {
			if x, y, ok := w32.GetCursorPos(); ok {
				// In screen pixels: above the mouse (the taskbar is usually at the bottom),
				// centred on it, kept inside the screen's free area.
				r := w32.GetWindowRect(hwnd)
				w, h := int(r.Right-r.Left), int(r.Bottom-r.Top)
				mi := w32.MONITORINFO{}
				mi.CbSize = uint32(unsafe.Sizeof(mi))
				w32.GetMonitorInfo(w32.MonitorFromPoint(x, y, w32.MONITOR_DEFAULTTONEAREST), &mi)
				wa := mi.RcWork
				nx, ny := x-w/2, y-h
				if ny < int(wa.Top) {
					ny = y // taskbar on top
				}
				nx = max(int(wa.Left), min(nx, int(wa.Right)-w))
				ny = max(int(wa.Top), min(ny, int(wa.Bottom)-h))
				w32.SetWindowPos(hwnd, 0, nx, ny, 0, 0, w32.SWP_NOSIZE|w32.SWP_NOZORDER|w32.SWP_NOACTIVATE)
				logf("Tray menu: at %d,%d, %dx%d (mouse %d,%d)", nx, ny, w, h, x, y)
			}
		}
		menuWin.Show()
		menuWin.Focus()
		if hwnd != 0 {
			w32.SetForegroundWindow(hwnd)
		}
	}
	trayAction = func(what string) any {
		switch what {
		case "busy":
			return map[string]int{"busy": downloading()}
		case "open":
			menuWin.Hide()
			showWin()
		case "folder":
			menuWin.Hide()
			os.MkdirAll(outDir, 0o755)
			openFolder(outDir)
		case "quit":
			menuWin.Hide()
			go func() { time.Sleep(100 * time.Millisecond); quit() }() // let the answer reach the menu first
		case "hide":
			if time.Since(time.UnixMilli(menuShown.Load())) > 500*time.Millisecond {
				menuWin.Hide()
			}
		}
		return map[string]bool{"ok": true}
	}

	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("ТЯГА")
	tray.OnClick(showWin)
	tray.OnRightClick(showMenu)

	go setupTools()
	if err := app.Run(); err != nil {
		fail("Could not open the window: " + err.Error())
	}
	return true
}

// The window's size: the page's cards plus a little room for their shadows.
const winWidth, winHeight = 800, 800

// The tray menu's window: the menu card (menu.html) plus room for its shadow.
const menuWidth, menuHeight = 256, 176

// lightWebview: WebView2 is Edge inside, and by itself it downloads Edge's add-ons into its
// data folder (DRM for Netflix, speech recognition, ad-block lists: some 60 MB) and phones home.
// ТЯГА's page needs none of it.
var lightWebview = []string{"--disable-component-update", "--disable-background-networking"}

// trimWebview deletes what WebView2 only keeps to be faster next time or downloaded by itself,
// before it starts (while it runs, its files are locked). What's left is a few MB: the
// page's switches and the sign-in.
func trimWebview(dir string) {
	ebw := filepath.Join(dir, "EBWebView")
	for _, d := range []string{"GrShaderCache", "ShaderCache", "GraphiteDawnCache", "component_crx_cache",
		"WidevineCdm", "Subresource Filter", "Speech Recognition", "hyphen-data", "BrowserMetrics", "Crashpad",
		"OnDeviceHeadSuggestModel", "ZxcvbnData", "SafetyTips", "TrustTokenKeyCommitments"} {
		os.RemoveAll(filepath.Join(ebw, d))
	}
	for _, d := range []string{"Cache", "Code Cache", "GPUCache", "DawnGraphiteCache", "DawnWebGPUCache"} {
		os.RemoveAll(filepath.Join(ebw, "Default", d))
	}
}

// hideDataDir makes tyaga-data a hidden folder: it's ТЯГА's business, not something to look at.
func hideDataDir(d string) {
	if p, err := syscall.UTF16PtrFromString(d); err == nil {
		if a, err := syscall.GetFileAttributes(p); err == nil {
			syscall.SetFileAttributes(p, a|syscall.FILE_ATTRIBUTE_HIDDEN)
		}
	}
}

// webviewDataDir is where WebView2 keeps its cache and the page's remembered switches: in
// tyaga-data (data.go), so every copy of ТЯГА has its own.
func webviewDataDir() string { return filepath.Join(dataDir, "webview") }

// moveOldWindowData: builds before tyaga-data kept the web view data in
// %LOCALAPPDATA%\TYAGA\webview-<folder id> and login-<folder id>.
func moveOldWindowData(d string) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return
	}
	old := filepath.Join(base, "TYAGA")
	if filepath.Dir(d) == old {
		return // tyaga-data couldn't be made next to the program: it lives here now
	}
	moveIn(filepath.Join(old, "webview-"+folderID()), filepath.Join(d, "webview"))
	moveIn(filepath.Join(old, "login-"+folderID()), filepath.Join(d, "login"))
	os.Remove(old) // only goes if nothing else is in it
}

// ---------------------------------------------------------------------------
//  The sign-in window (login.go)
// ---------------------------------------------------------------------------

const canLogin = true

// defaultBrowser is the browser that opens links, as yt-dlp names it ("" = unknown).
func defaultBrowser() string {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\Shell\Associations\UrlAssociations\https\UserChoice`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	id, _, _ := k.GetStringValue("ProgId")
	for _, p := range []struct{ prefix, name string }{
		{"Brave", "brave"}, {"Firefox", "firefox"}, {"ChromeHTML", "chrome"}, {"MSEdge", "edge"},
		{"Vivaldi", "vivaldi"}, {"Opera", "opera"}, {"Chromium", "chromium"}, {"Whale", "whale"},
	} {
		if strings.HasPrefix(id, p.prefix) {
			return p.name
		}
	}
	return ""
}

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
	trimWebview(loginDir())
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
		Windows: application.WindowsOptions{WebviewUserDataPath: loginDir(), AdditionalBrowserArgs: lightWebview},
	})
	win = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:           "ТЯГА · sign in to YouTube",
		Width:           520,
		Height:          760,
		InitialPosition: application.WindowCentered,
		// Not URL + ExecJS: Wails holds ExecJS back until its own page script says it's ready,
		// which never happens on Google's and YouTube's pages, so the check never ran. A start
		// page with JS instead: Wails then adds the JS as a script that runs at the start of
		// every page the window opens, and the start page goes on to Google's sign-in.
		HTML:    `<!doctype html><meta charset="utf-8"><script>location.replace(` + strconv.Quote(signInURL) + `)</script>`,
		JS:      signedInJS,
		Windows: application.WindowsWindow{Theme: application.SystemDefault},
	})
	app.Run()
}

// loginDir is the sign-in window's web view data, next to the main window's.
func loginDir() string { return filepath.Join(dataDir, "login") }

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
