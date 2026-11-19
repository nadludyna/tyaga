package main

// Things that work differently on Windows.

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"unsafe"
)

const createNoWindow = 0x08000000

// hideWindow runs yt-dlp and ffmpeg without extra black console windows.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// killTree stops a process together with its children (yt-dlp starts ffmpeg).
func killTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	k := exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
	hideWindow(k)
	k.Run()
}

// openURL opens a link in the default browser.
func openURL(url string) {
	exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

var (
	ole32   = syscall.NewLazyDLL("ole32.dll")
	shell32 = syscall.NewLazyDLL("shell32.dll")
	user32  = syscall.NewLazyDLL("user32.dll")
)

// revealFile opens Explorer with the file selected, the way Windows itself does it
// (SHOpenFolderAndSelectItems). yt-dlp's paths for playlists mix / and \, and Explorer
// only understands \: with the mix it just opened a folder, so the path is cleaned first.
// If the shell call fails, "explorer /select" is the fallback.
func revealFile(path string) {
	path = filepath.Clean(path)
	unlockForeground()

	ok := make(chan bool, 1)
	go func() { // COM wants one thread for the whole call
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		const apartmentThreaded = 0x2
		ole32.NewProc("CoInitializeEx").Call(0, apartmentThreaded)
		defer ole32.NewProc("CoUninitialize").Call()

		p, err := syscall.UTF16PtrFromString(path)
		if err != nil {
			ok <- false
			return
		}
		pidl, _, _ := shell32.NewProc("ILCreateFromPathW").Call(uintptr(unsafe.Pointer(p)))
		if pidl == 0 {
			ok <- false
			return
		}
		defer shell32.NewProc("ILFree").Call(pidl)
		hr, _, _ := shell32.NewProc("SHOpenFolderAndSelectItems").Call(pidl, 0, 0, 0)
		ok <- hr == 0 // S_OK
	}()

	if !<-ok {
		// explorer wants /select,"path" and doesn't understand Go's usual quoting of the whole argument.
		cmd := exec.Command("explorer.exe")
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + path + `"`}
		cmd.Start()
	}
}

// unlockForeground lets Explorer come to the front: Windows only lets the program you're
// working in hand the front on, and that's ТЯГА (the click was in its window), so it says
// any program may take it. No synthetic Alt tap: that was a trick from when the page lived in
// a browser tab, and in ТЯГА's own window a lone Alt opens the window menu, which swallowed
// the next keys and broke Alt+Tab.
func unlockForeground() {
	const anyProcess = ^uint32(0) // ASFW_ANY
	user32.NewProc("AllowSetForegroundWindow").Call(uintptr(anyProcess))
}

// openFolder opens a folder in Explorer.
func openFolder(dir string) {
	unlockForeground()
	exec.Command("explorer.exe", dir).Start()
}

// consoleUTF8 makes non-Latin text in the console readable instead of mojibake.
func consoleUTF8() {
	syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleOutputCP").Call(65001)
}

// showError shows an error in a message box: there's no console window to print it to.
func showError(msg string) {
	text, _ := syscall.UTF16PtrFromString(msg)
	title, _ := syscall.UTF16PtrFromString("ТЯГА")
	const iconError = 0x10
	syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0,
		uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), iconError)
}
