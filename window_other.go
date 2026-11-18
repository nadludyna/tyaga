//go:build !windows

package main

import "os"

// Only the Windows build has its own window; elsewhere ТЯГА runs in the browser.
func runWindow(token string) bool { return false }

var exitApp = func() { os.Exit(0) }

func windowControl(action string, n int) {}

// Signing in from ТЯГА needs its own window, so only on Windows (login.go).
const canLogin = false

func runLogin()              {}
func defaultBrowser() string { return "" }
func loginDir() string       { return "" }

func moveOldWindowData(string) {}
