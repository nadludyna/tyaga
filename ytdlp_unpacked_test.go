package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// A release like yt-dlp's: the zip (yt-dlp.exe and _internal\) and the checksum list.
func fakeRelease(t *testing.T, files map[string]string) (zipBytes []byte, sum string) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	zw.Close()
	h := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(h[:])
}

func TestUnpackedYtdlp(t *testing.T) {
	oldBin, oldRel := binDir, ytdlpRelease
	defer func() { binDir, ytdlpRelease = oldBin, oldRel }()
	binDir = t.TempDir()

	var archive []byte
	var sums string
	var zipGets atomic.Int32
	down := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if down {
			http.Error(w, "no", http.StatusBadGateway)
			return
		}
		switch r.URL.Path {
		case "/SHA2-256SUMS":
			w.Write([]byte(sums))
		case "/yt-dlp_win.zip":
			zipGets.Add(1)
			w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	ytdlpRelease = srv.URL + "/"
	release := func(version string) {
		var sum string
		archive, sum = fakeRelease(t, map[string]string{"yt-dlp.exe": "exe " + version, "_internal/base_library.zip": "lib " + version})
		sums = "1111111111111111111111111111111111111111111111111111111111111111  yt-dlp.exe\n" + sum + "  yt-dlp_win.zip\n"
	}
	read := func(p string) string { b, _ := os.ReadFile(p); return string(b) }

	// The first start: installed; the one-file yt-dlp.exe of before goes.
	os.WriteFile(filepath.Join(binDir, exeName("yt-dlp")), []byte("old one-file"), 0o755)
	release("1")
	exe, err := setupUnpackedYtdlp("yt-dlp_win.zip")
	if err != nil || read(exe) != "exe 1" || read(filepath.Join(ytdlpDir(), "_internal", "base_library.zip")) != "lib 1" {
		t.Fatalf("not installed: %v, %q", err, read(exe))
	}
	if fileExists(filepath.Join(binDir, exeName("yt-dlp"))) {
		t.Error("the old one-file yt-dlp should be gone")
	}
	// The next start, no new release: nothing downloaded.
	if _, err := setupUnpackedYtdlp("yt-dlp_win.zip"); err != nil || zipGets.Load() != 1 {
		t.Errorf("downloaded again without a new release (%d), %v", zipGets.Load(), err)
	}
	// A new release: brought up to date, nothing of the old one left over.
	release("2")
	if exe, err := setupUnpackedYtdlp("yt-dlp_win.zip"); err != nil || read(exe) != "exe 2" {
		t.Errorf("not updated: %v, %q", err, read(exe))
	}
	if dirExists(ytdlpDir()+".old") || dirExists(ytdlpDir()+".new") {
		t.Error("the swap left a folder behind")
	}
	// Offline: what's there is used.
	down = true
	if exe, err := setupUnpackedYtdlp("yt-dlp_win.zip"); err != nil || read(exe) != "exe 2" {
		t.Errorf("offline: %v, %q", err, read(exe))
	}
	down = false

	// A damaged download on a first install: an error (the old way takes over), nothing half-made.
	os.RemoveAll(ytdlpDir())
	release("3")
	sums = "2222222222222222222222222222222222222222222222222222222222222222  yt-dlp_win.zip\n"
	if _, err := setupUnpackedYtdlp("yt-dlp_win.zip"); err == nil || dirExists(ytdlpDir()) {
		t.Errorf("a checksum mismatch should fail and leave nothing: %v", err)
	}

	// An archive with a name pointing outside the folder is refused.
	evil, sum := fakeRelease(t, map[string]string{"yt-dlp.exe": "x", "../escaped.txt": "x"})
	archive, sums = evil, sum+"  yt-dlp_win.zip\n"
	if _, err := setupUnpackedYtdlp("yt-dlp_win.zip"); err == nil {
		t.Error("an archive writing outside its folder should be refused")
	}
	if fileExists(filepath.Join(binDir, "escaped.txt")) || fileExists(filepath.Join(filepath.Dir(binDir), "escaped.txt")) {
		t.Error("a file got out of the folder")
	}
}
