//go:build !windows

package main

import "os"

// Not Windows: the test stand and development. No window; the page opens in the browser.
func runWindow(token string) bool { return false }

var exitApp = func() { os.Exit(0) }

func windowControl(action string, n int) {}

// Signing in from ТЯГА needs its own window, so only on Windows (login.go).
const canLogin = false

func runLogin()              {}
func defaultBrowser() string { return "" }
func loginDir() string       { return "" }

func moveOldWindowData(string) {}
func hideDataDir(string)       {}
