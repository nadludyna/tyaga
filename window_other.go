//go:build !windows

package main

import "os"

// Only the Windows build has its own window; elsewhere ТЯГА runs in the browser.
func runWindow(token string) bool { return false }

var exitApp = func() { os.Exit(0) }

func windowControl(action string, n int) {}
