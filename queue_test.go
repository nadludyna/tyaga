package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What was unfinished when ТЯГА closed comes back stopped, with its title and picture; what
// was done doesn't. A cancelled-by-hand download isn't kept.
func TestQueueSurvivesRestart(t *testing.T) {
	old, oldJobs, oldID := dataDir, jobs, nextID
	defer func() { dataDir, jobs, nextID = old, oldJobs, oldID }()
	dataDir = t.TempDir()
	jobs = []*Job{
		{ID: 1, URL: "https://youtu.be/aaaaaaaaaaa", Status: "done", Title: "Done one"},
		{ID: 2, URL: "https://youtu.be/bbbbbbbbbbb", Status: "running", Title: "[2/5] Song", List: true, listName: "My list", Options: Options{Mode: "audio"}},
		{ID: 3, URL: "https://youtu.be/ccccccccccc", Status: "queued", Title: "Waiting one", Thumb: "https://i.ytimg.com/vi/ccccccccccc/mqdefault.jpg"},
		{ID: 4, URL: "https://youtu.be/ddddddddddd", Status: "cancelled", Title: "Not wanted"},
	}
	last := ""
	writeQueue(unfinished(), &last)

	jobs, nextID = nil, 1
	loadQueue()
	if len(jobs) != 2 {
		t.Fatalf("want the 2 unfinished back, got %d", len(jobs))
	}
	if j := jobs[0]; j.Title != "My list" || !j.List || j.Mode != "audio" || j.Status != "cancelled" || !j.Stopped {
		t.Errorf("the playlist came back wrong: %+v", j)
	}
	if j := jobs[1]; j.Title != "Waiting one" || j.Thumb == "" || !j.Stopped {
		t.Errorf("the queued one came back wrong: %+v", j)
	}

	// Nothing unfinished any more: the file goes.
	jobs = nil
	writeQueue(unfinished(), &last)
	if fileExists(queueFile()) {
		t.Error("queue.json should be gone when nothing is unfinished")
	}
}

// A cancel removes the half-done pieces, the picture and the source file, but never a finished
// file and nothing outside the download's folder.
func TestLeftovers(t *testing.T) {
	dir := t.TempDir()
	mk := func(name string) string {
		p := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x"), 0o644)
		return p
	}
	done := mk("List [x]/01 - Done.m4a")
	mk("List [x]/02 - Half.webm.part")
	mk("List [x]/02 - Half.webm.part-Frag3")
	mk("List [x]/02 - Half.webp")
	video := mk("Clip.f299.mp4")
	outside := filepath.Join(t.TempDir(), "other.mp4")
	os.WriteFile(outside, []byte("x"), 0o644)

	j := &Job{dir: dir, paths: []string{done}}
	for _, l := range []string{
		"[download] Destination: " + done,
		"[download] Destination: " + filepath.Join(dir, "List [x]", "02 - Half.webm"),
		"[info] Writing video thumbnail 44 to: " + filepath.Join(dir, "List [x]", "02 - Half.webp"),
		"[download] Destination: " + video,
		"[download] Destination: " + outside,
	} {
		noteTemp(j, l)
	}
	got := map[string]bool{}
	for _, p := range leftovers(j) {
		got[filepath.Base(p)] = true
	}
	for _, want := range []string{"02 - Half.webm.part", "02 - Half.webm.part-Frag3", "02 - Half.webp", "Clip.f299.mp4"} {
		if !got[want] {
			t.Errorf("%s should go; got %v", want, got)
		}
	}
	if got["01 - Done.m4a"] || got["other.mp4"] {
		t.Errorf("a finished file or one outside the folder would go: %v", got)
	}
}

// The video id as a link shows it: what the preview's picture and SponsorBlock start from at once.
func TestVideoID(t *testing.T) {
	for link, want := range map[string]string{
		"https://youtu.be/dQw4w9WgXcQ":                              "dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ?t=60":                         "dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PLx&si=a": "dQw4w9WgXcQ",
		"https://m.youtube.com/watch?v=dQw4w9WgXcQ":                 "dQw4w9WgXcQ",
		"https://music.youtube.com/watch?v=dQw4w9WgXcQ":             "dQw4w9WgXcQ",
		"https://youtube.com/shorts/dQw4w9WgXcQ?si=x":               "dQw4w9WgXcQ",
		"https://www.youtube.com/live/dQw4w9WgXcQ":                  "dQw4w9WgXcQ",
		"https://www.youtube.com/playlist?list=PLx":                 "",
		"https://www.youtube.com/@someone":                          "",
		"https://vimeo.com/76979871":                                "",
	} {
		if got := videoID(link); got != want {
			t.Errorf("videoID(%s) = %q, want %q", link, got, want)
		}
	}
}

// Not picked: audio to Windows' Music, video to Videos. Picked: that folder, sorted.
func TestDirFor(t *testing.T) {
	if mediaDir(true) == "" {
		t.Skip("Windows' own folders: Windows only")
	}
	oldOut, oldPicked := outDir, outPicked
	defer func() { outDir, outPicked = oldOut, oldPicked }()
	outDir, outPicked = t.TempDir(), false
	if got := dirFor(true, "ru"); got != mediaDir(true) || !strings.HasSuffix(strings.ToLower(got), "music") && !strings.Contains(got, "Муз") {
		t.Errorf("audio, not picked: %s", got)
	}
	if got := dirFor(false, "ru"); got != mediaDir(false) {
		t.Errorf("video, not picked: %s", got)
	}
	t.Logf("Music: %s, Videos: %s", mediaDir(true), mediaDir(false))
	outPicked = true
	if got := dirFor(true, "ru"); got != filepath.Join(outDir, "Аудио") {
		t.Errorf("audio, picked: %s", got)
	}
}

// Security: only sound and video open on a double-click (a downloaded .exe or .bat would run),
// and options from the page or queue.json keep to the known values.
func TestOnlyMediaAndKnownOptions(t *testing.T) {
	for p, want := range map[string]bool{`C:\M\song.m4a`: true, `C:\M\clip.MP4`: true, `C:\M\a.mp3`: true,
		`C:\M\setup.exe`: false, `C:\M\x.bat`: false, `C:\M\x.cmd`: false, `C:\M\x.lnk`: false, `C:\M\x.ps1`: false, `C:\M\x.mp4.exe`: false} {
		if isMedia(p) != want {
			t.Errorf("isMedia(%s) = %v", p, !want)
		}
	}
	o := cleanOptions(Options{Mode: "--exec calc", Format: "exe", Quality: 9999, Lang: "../x", From: "rm", To: "-rf"})
	if o.Mode != "video" || o.Format != "m4a" || o.Quality != 0 || o.Lang != "ru" || o.From != "" || o.To != "" {
		t.Errorf("unknown values got through: %+v", o)
	}
}
