package main

import (
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestLiveCovers asks the real Apple Music catalogue which cover each song gets.
// Off by default (needs the internet): TYAGA_LIVE=1 go test -run Live -v
func TestLiveCovers(t *testing.T) {
	if os.Getenv("TYAGA_LIVE") == "" {
		t.Skip("set TYAGA_LIVE=1 to ask the real catalogue")
	}
	cases := []struct{ artist, title, album, want string }{
		{"Orbital", "The Girl With The Sun In Her Head", "In Sides", "081227153366"},
		{"Orbital", "The Girl With The Sun In Her Head", "", "081227153366"},
		{"Orbital", "Belfast", "Orbital 20", "825646890934"},
		{"Orbital", "Petrol", "", "081227153366"},
	}
	for _, c := range cases {
		q := url.Values{"term": {c.artist + " " + c.title}, "media": {"music"}, "entity": {"song"}, "limit": {"25"}}
		var res struct {
			Results []release `json:"results"`
		}
		if json.Unmarshal(get(itunesURL+"?"+q.Encode()), &res) != nil {
			t.Fatalf("no answer from the catalogue")
		}
		picked := pickReleases(res.Results, "song", c.artist, c.title, c.album)
		if c.album == "" && len(picked) > 1 {
			picked = firstAlbum(picked)
		}
		got := "(none: the thumbnail stays)"
		if len(picked) > 0 {
			got = picked[0].CollectionName + " " + picked[0].ArtworkURL100
		}
		t.Logf("%s - %s [album %q] → %s", c.artist, c.title, c.album, got)
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: want the cover %s", c.title, c.want)
		}
	}
}

// peek against the real YouTube: a title and channel in well under the time yt-dlp takes.
func TestLivePeek(t *testing.T) {
	if os.Getenv("TYAGA_LIVE") == "" {
		t.Skip("TYAGA_LIVE=1 to ask the real YouTube")
	}
	t0 := time.Now()
	f := peek("https://youtu.be/fJ9rUzIMcZQ?si=x")
	if f.Title == "" || f.Channel == "" || f.ID != "fJ9rUzIMcZQ" {
		t.Fatalf("no title from oEmbed: %+v", f)
	}
	t.Logf("%q by %q in %v", f.Title, f.Channel, time.Since(t0))
}

// The real unpacked yt-dlp from GitHub: installed, runs, and a second start downloads nothing.
func TestLiveUnpackedYtdlp(t *testing.T) {
	if os.Getenv("TYAGA_LIVE") == "" || ytdlpZipName() == "" {
		t.Skip("TYAGA_LIVE=1 on Windows to fetch the real yt-dlp")
	}
	old := binDir
	defer func() { binDir = old }()
	binDir = t.TempDir()
	t0 := time.Now()
	exe, err := setupUnpackedYtdlp(ytdlpZipName())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("installed in %v", time.Since(t0))
	t1 := time.Now()
	out, err := exec.Command(exe, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("yt-dlp %s started in %v", strings.TrimSpace(string(out)), time.Since(t1))
	t2 := time.Now()
	if _, err := setupUnpackedYtdlp(ytdlpZipName()); err != nil {
		t.Fatal(err)
	}
	t.Logf("the next start's check took %v", time.Since(t2))
}
