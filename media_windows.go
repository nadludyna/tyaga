//go:build windows

package main

import "golang.org/x/sys/windows"

// mediaDir is Windows' own Music or Videos folder, where Windows says it is: moved to another
// drive or into OneDrive, it's still found (a path put together by hand wouldn't be). "" if
// Windows doesn't say.
func mediaDir(audio bool) string {
	id := windows.FOLDERID_Videos
	if audio {
		id = windows.FOLDERID_Music
	}
	p, err := windows.KnownFolderPath(id, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return ""
	}
	return p
}
