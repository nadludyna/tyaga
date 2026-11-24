//go:build !windows

package main

// mediaDir: Windows' Music or Videos folder; elsewhere (the test stand) there's none, and the
// downloads folder is used (typeDir).
func mediaDir(audio bool) string { return "" }
