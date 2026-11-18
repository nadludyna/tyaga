//go:build windows

package main

// ТЯГА's own window on Windows (Wails, on the Edge WebView2 that comes with Windows 10/11).
// The page and its API are the same as in the browser; Wails just serves them to the window
// itself, with no network port at all. The window lives next to a tray icon:
//   - there's no system title bar: the page is the whole window, rounded and shadowed by
//     Windows 11 itself; a strip along the top of the page drags it and holds the buttons;
//   - the close button hides the window to the tray, downloads go on;
//   - a click on the tray icon brings the window back;
//   - Quit (tray menu or the page) stops everything and exits.

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"golang.org/x/sys/windows/registry"
)

//go:embed winres/icon.ico
var trayIcon []byte

// exitApp is how quit() leaves: through Wails when there's a window, so the tray icon
// disappears with it instead of hanging around until the mouse passes over it.
var exitApp = func() { os.Exit(0) }

// windowControl is what the page's own window buttons do (the window has no system title bar).
var windowControl = func(action string) {}

// runWindow opens ТЯГА in its own window and returns when ТЯГА quits.
// A second launch from the same folder doesn't start a second ТЯГА: it brings up the window
// of the running one (a copy in another folder is a different ТЯГА and runs side by side).
func runWindow(token string) bool {
	var win *application.WebviewWindow
	showWin := func() {
		if win == nil {
			return
		}
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

	dark := systemDark()
	bg := application.NewRGB(0xf6, 0xf5, 0xf2) // the page's own background, so opening doesn't flash white
	if dark {
		bg = application.NewRGB(0x0f, 0x0f, 0x10)
	}
	win = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "ТЯГА",
		Width:            860,
		Height:           860,
		MinWidth:         420,
		MinHeight:        520,
		InitialPosition:  application.WindowCentered,
		URL:              "/?t=" + token + "&w=1", // w: the page draws its own title bar
		Frameless:        true,
		BackgroundColour: bg,
		DevToolsEnabled:  os.Getenv("TYAGA_DEVTOOLS") == "1",
		Permissions: map[application.PermissionType]application.Permission{
			application.PermissionNotifications: application.PermissionAllow,
		},
		Windows: application.WindowsWindow{
			Theme:                  application.SystemDefault,
			NonClientRegionSupport: true, // the page's "app-region: drag" strip drags it like a title bar
		},
	})
	windowControl = func(action string) {
		switch action {
		case "min":
			win.Minimise()
		case "max":
			win.ToggleMaximise()
		case "close":
			win.Hide()
		}
	}
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		win.Hide()
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

// systemDark: Windows is set to the dark app mode.
func systemDark() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	light, _, err := k.GetIntegerValue("AppsUseLightTheme")
	return err == nil && light == 0
}
