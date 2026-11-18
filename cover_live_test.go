package main

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"
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
