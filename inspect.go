package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"
)

// What a link is, looked up before downloading it: the page shows it under the links box
// (a picture, the title, what's in it) and offers only the options that make sense for it,
// already set the sensible way: "split into tracks" for an album uploaded as one video,
// "cut the ads" when SponsorBlock knows of ads in it, the qualities the video really has.
type Facts struct {
	Kind      string  `json:"kind"` // "video"; "list": a playlist or a channel; "inlist": a video inside a playlist
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Channel   string  `json:"channel"`
	Duration  float64 `json:"duration"`
	Thumb     string  `json:"thumb"`
	Chapters  int     `json:"chapters"`
	Parts     []Part  `json:"parts,omitempty"` // the chapters themselves: picked in the fragment drop-down
	Music     bool    `json:"music"`           // YouTube Music, an artist's "- Topic" channel, or the Music category
	Heights   []int   `json:"heights"`         // the video heights there are, biggest first
	ListTitle string  `json:"listTitle"`
	ListCount int     `json:"listCount"`
	Ads       int     `json:"ads"`    // SponsorBlock: sponsor, self-promotion and "subscribe!" bits
	AdsSec    float64 `json:"adsSec"` // how long they are together
	Offtopic  float64 `json:"offtopic"`
	Error     string  `json:"error,omitempty"`
}

// Part is one chapter of a video: where it starts and ends, in seconds, and its name.
type Part struct {
	Title string  `json:"title"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

var (
	factsMu    sync.Mutex
	factsCache = map[string]Facts{} // by link: pasting the same one again answers at once
)

// inspect looks the link up with yt-dlp (no download) and asks SponsorBlock about ads.
// A playlist is looked up flat (only its list, which is quick). A video inside a playlist
// is looked up twice at once: the video, and how many there are in its playlist.
func inspect(link string) Facts {
	factsMu.Lock()
	f, ok := factsCache[link]
	factsMu.Unlock()
	if ok {
		return f
	}
	fixed, list, _ := classify(link, false)
	inList := !list && strings.Contains(link, "list=")
	if list {
		f = lookup(fixed, true)
		f.Kind = "list"
	} else {
		var lf Facts
		var wg sync.WaitGroup
		if inList {
			wg.Add(1)
			go func() { defer wg.Done(); full, _, _ := classify(link, true); lf = lookup(full, true) }()
		}
		f = lookup(fixed, false)
		wg.Wait()
		f.Kind = "video"
		if inList && lf.Error == "" && lf.ListCount > 1 {
			f.Kind, f.ListTitle, f.ListCount = "inlist", lf.ListTitle, lf.ListCount
		}
		if f.Error == "" && f.ID != "" {
			f.Ads, f.AdsSec, f.Offtopic = sponsorSegments(f.ID)
		}
	}
	if u, err := url.Parse(link); err == nil && u.Host == "music.youtube.com" {
		f.Music = true
	}
	if f.Error == "" { // a failed look is tried again next time
		factsMu.Lock()
		factsCache[link] = f
		factsMu.Unlock()
	}
	return f
}

// lookup runs yt-dlp -J (everything about the link as JSON, nothing downloaded).
func lookup(link string, flat bool) Facts {
	if !tools.ready() {
		return Facts{Error: "not ready"}
	}
	ctx, stop := context.WithTimeout(context.Background(), 40*time.Second)
	defer stop()
	args := []string{"--ignore-config", "--no-plugin-dirs", "--no-warnings", "--skip-download",
		"-J", "--socket-timeout", "15", "--encoding", "utf-8"}
	if flat {
		args = append(args, "--flat-playlist", "--yes-playlist")
	} else {
		args = append(args, "--no-playlist")
	}
	if tools.deno != "" {
		args = append(args, "--js-runtimes", "deno:"+tools.deno)
	}
	if b := browserSource(); b != "" {
		args = append(args, "--cookies-from-browser", b)
	} else if fileExists(cookiesFile) {
		args = append(args, "--cookies", cookiesFile)
	}
	cmd := exec.CommandContext(ctx, tools.ytdlp, append(args, "--", link)...)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return Facts{Error: "couldn't look it up"}
	}
	var v struct {
		ID         string
		Title      string
		Channel    string
		Uploader   string
		Duration   float64
		Thumbnail  string
		Categories []string
		Chapters   []struct {
			Title     string
			StartTime float64 `json:"start_time"`
			EndTime   float64 `json:"end_time"`
		}
		Formats []struct {
			Height int
			Vcodec string
		}
		Entries       []json.RawMessage
		PlaylistCount int `json:"playlist_count"`
	}
	if json.Unmarshal(out, &v) != nil {
		return Facts{Error: "couldn't read what yt-dlp said"}
	}
	f := Facts{ID: v.ID, Title: v.Title, Channel: v.Channel, Duration: v.Duration, Thumb: v.Thumbnail, Chapters: len(v.Chapters)}
	for i, c := range v.Chapters {
		end := c.EndTime
		if end <= c.StartTime { // not given: up to the next chapter, or the end of the video
			end = v.Duration
			if i+1 < len(v.Chapters) {
				end = v.Chapters[i+1].StartTime
			}
		}
		f.Parts = append(f.Parts, Part{Title: c.Title, Start: c.StartTime, End: end})
	}
	if f.Channel == "" {
		f.Channel = v.Uploader
	}
	f.Music = slices.Contains(v.Categories, "Music") || strings.HasSuffix(f.Channel, " - Topic")
	for _, x := range v.Formats {
		if x.Height > 0 && x.Vcodec != "none" && !slices.Contains(f.Heights, x.Height) {
			f.Heights = append(f.Heights, x.Height)
		}
	}
	slices.Sort(f.Heights)
	slices.Reverse(f.Heights)
	if flat {
		f.ListTitle, f.ListCount = v.Title, max(v.PlaylistCount, len(v.Entries))
	}
	return f
}

// sponsorSegments asks SponsorBlock (the same service yt-dlp uses to cut them) what's known
// about the video: how many ad bits and how long, and how long the non-music parts are.
// Nothing known, or no answer: none.
func sponsorSegments(id string) (n int, sec, offtopic float64) {
	base := "https://sponsor.ajay.app"
	if os.Getenv("TYAGA_SPONSORBLOCK") == "fake" { // the test stand: ids with SPON in them have ads
		if strings.Contains(id, "SPON") {
			return 2, 80, 0
		}
		return 0, 0, 0
	}
	q := url.Values{"videoID": {id}, "categories": {`["sponsor","selfpromo","interaction","music_offtopic"]`}}
	c := http.Client{Timeout: 6 * time.Second}
	resp, err := c.Get(base + "/api/skipSegments?" + q.Encode())
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	var segs []struct {
		Segment  [2]float64
		Category string
	}
	if json.NewDecoder(resp.Body).Decode(&segs) != nil {
		return
	}
	for _, s := range segs {
		if s.Category == "music_offtopic" {
			offtopic += s.Segment[1] - s.Segment[0]
			continue
		}
		n++
		sec += s.Segment[1] - s.Segment[0]
	}
	return
}
