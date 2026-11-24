package main

// yt-dlp on Windows: ТЯГА keeps the official "unpacked" build (yt-dlp_win.zip: yt-dlp.exe and
// its _internal folder) in bin\yt-dlp, and uses it even when another yt-dlp is installed. The
// usual one-file yt-dlp.exe unpacks itself into a temp folder at every start: 2–2.5 s before it
// does anything (and 30 MB written each time); this one starts in 0.8 s. Measured on the
// owner's PC: a link's look went from 4.2 s to 2.5, and every download starts that much sooner.
// It can't update itself (-U), so ТЯГА does: at each start the release's checksum list (a small
// file) says whether there's a new one; only then is the archive downloaded again. Offline, or
// if the download fails, what's there is used (or, the first time, the old way: a yt-dlp found
// in bin or PATH).

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ytdlpRelease: where the release files are (a variable for the tests).
var ytdlpRelease = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/"

func ytdlpDir() string { return filepath.Join(binDir, "yt-dlp") }

// ytdlpZipName: the unpacked build for this Windows, or "" where there's none.
func ytdlpZipName() string {
	// (The test stand gives ТЯГА its fake yt-dlp in PATH, FAKE_YTDLP set: the real one isn't brought in.)
	if runtime.GOOS != "windows" || os.Getenv("FAKE_YTDLP") != "" {
		return ""
	}
	return map[string]string{"amd64": "yt-dlp_win.zip", "arm64": "yt-dlp_win_arm64.zip"}[runtime.GOARCH]
}

// setupUnpackedYtdlp installs or updates bin\yt-dlp and returns its yt-dlp.exe.
func setupUnpackedYtdlp(zipName string) (string, error) {
	dir := ytdlpDir()
	exe := filepath.Join(dir, "yt-dlp.exe")
	stamp := filepath.Join(dir, "release.sha256") // the archive it was unpacked from
	have := fileExists(exe)
	if have {
		tools.setStatus("Checking for yt-dlp updates…")
	}
	sums, err := getSmall(ytdlpRelease + "SHA2-256SUMS")
	if err != nil {
		if have {
			logf("yt-dlp: couldn't check for updates (%v), using the one there is", err)
			return exe, nil
		}
		return "", err
	}
	want := expectedSum(sums, zipName)
	if want == "" {
		if have {
			return exe, nil
		}
		return "", errors.New("the checksum list doesn't mention " + zipName)
	}
	if b, _ := os.ReadFile(stamp); have && strings.EqualFold(strings.TrimSpace(string(b)), want) {
		return exe, nil // the newest already
	}

	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", err
	}
	archive, got, err := fetchToTemp("yt-dlp", ytdlpRelease+zipName)
	if archive != "" {
		defer os.Remove(archive)
	}
	if err == nil && !strings.EqualFold(got, want) {
		err = errors.New("the download is damaged or not the published file (checksum mismatch)")
	}
	if err != nil {
		if have {
			logf("yt-dlp: couldn't update (%v), using the one there is", err)
			return exe, nil
		}
		return "", err
	}

	// Unpacked next to the old one, then swapped: a half-unpacked yt-dlp is never used.
	fresh, old := dir+".new", dir+".old"
	os.RemoveAll(fresh)
	os.RemoveAll(old)
	if err := unzipAll(archive, fresh); err != nil {
		os.RemoveAll(fresh)
		return "", err
	}
	if !fileExists(filepath.Join(fresh, "yt-dlp.exe")) {
		os.RemoveAll(fresh)
		return "", errors.New("the archive has no yt-dlp.exe")
	}
	if err := os.WriteFile(filepath.Join(fresh, "release.sha256"), []byte(want+"\n"), 0o644); err != nil {
		os.RemoveAll(fresh)
		return "", err
	}
	if fileExists(exe) || dirExists(dir) {
		if err := os.Rename(dir, old); err != nil { // (in use: the old one stays this time)
			os.RemoveAll(fresh)
			if have {
				return exe, nil
			}
			return "", err
		}
	}
	if err := os.Rename(fresh, dir); err != nil {
		os.Rename(old, dir)
		return "", err
	}
	os.RemoveAll(old)
	// The one-file yt-dlp.exe ТЯГА used to keep in bin isn't needed any more.
	os.Remove(filepath.Join(binDir, exeName("yt-dlp")))
	if have {
		logf("yt-dlp updated")
	} else {
		logf("yt-dlp installed (the unpacked build, starts faster)")
	}
	return exe, nil
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// getSmall fetches a small text file (a checksum list).
func getSmall(url string) (string, error) {
	c := http.Client{Timeout: 15 * time.Second, Transport: fetch.Transport}
	resp, err := c.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server replied %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return string(b), err
}

// fetchToTemp downloads url into a temp file in bin, showing progress, and returns it with its SHA-256.
func fetchToTemp(name, url string) (string, string, error) {
	tmp, err := os.CreateTemp(binDir, name+"-*.part")
	if err != nil {
		return "", "", err
	}
	resp, err := fetch.Get(url)
	if err != nil {
		tmp.Close()
		return tmp.Name(), "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		tmp.Close()
		return tmp.Name(), "", fmt.Errorf("server replied %s", resp.Status)
	}
	hash := sha256.New()
	body := newStallReader(resp.Body, time.Minute)
	defer body.t.Stop()
	_, err = io.Copy(io.MultiWriter(tmp, hash), &progressReader{r: body, total: resp.ContentLength, name: name})
	tmp.Close()
	return tmp.Name(), hex.EncodeToString(hash.Sum(nil)), err
}

// unzipAll unpacks the whole archive into dir, keeping its folders. A name that would land
// outside dir ("../…", a drive) stops it.
func unzipAll(archive, dir string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		dst := filepath.Join(root, filepath.FromSlash(f.Name))
		if rel, err := filepath.Rel(root, dst); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(f.Name) {
			return fmt.Errorf("a file in the archive points outside it: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := extract(f, dst); err != nil {
			return err
		}
	}
	return nil
}
