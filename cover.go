package main

// Track covers. yt-dlp embeds the video's thumbnail. Then, for every finished audio file:
//
//  1. The real album cover is looked up in the Apple Music catalogue (the iTunes Search API:
//     public, no key) by artist and title. It's used only when both clearly match, so a
//     wrong album never gets in. Square, 1200×1200.
//  2. If there is none, the thumbnail is looked at: an album cover sitting between plain bars
//     (how YouTube shows "Artist - Topic" tracks and many music uploads) gets the bars cut
//     off. A normal 16:9 thumbnail — a clip, a podcast — is left whole.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
	"unicode"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png" // the embedded cover can be PNG too
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// artBox finds an album cover framed by plain bars: it trims rows and columns that look like
// the edge colour from all four sides and returns what's left, if that is roughly a square.
func artBox(img image.Image) (image.Rectangle, bool) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 32 || h < 32 {
		return b, false
	}
	step := max(1, min(w, h)/90) // look at ~90 points per line, plenty for flat bars

	// plain: every sampled pixel of the line is close to the line's first pixel.
	plain := func(x0, y0, dx, dy, n int) bool {
		r0, g0, b0 := rgb(img.At(x0, y0))
		for i := 0; i < n; i += step {
			r, g, bb := rgb(img.At(x0+dx*i, y0+dy*i))
			if abs(r-r0)+abs(g-g0)+abs(bb-b0) > 40 {
				return false
			}
		}
		return true
	}
	left, right, top, bottom := b.Min.X, b.Max.X, b.Min.Y, b.Max.Y
	for left < right-1 && plain(left, b.Min.Y, 0, 1, h) {
		left++
	}
	for right-1 > left && plain(right-1, b.Min.Y, 0, 1, h) {
		right--
	}
	for top < bottom-1 && plain(left, top, 1, 0, right-left) {
		top++
	}
	for bottom-1 > top && plain(left, bottom-1, 1, 0, right-left) {
		bottom--
	}

	box := image.Rect(left, top, right, bottom)
	bw, bh := box.Dx(), box.Dy()
	if bw*bh < w*h/8 { // almost everything is "plain": an empty or one-colour picture
		return b, false
	}
	trimmed := bw < w*96/100 || bh < h*96/100
	square := bw*100 >= bh*90 && bh*100 >= bw*90
	if !trimmed || !square {
		return b, false
	}
	// Exactly square, centred in what was found, a hair smaller: JPEG blurs the edge of the
	// art into the bars, and a thin light or dark line around a cover looks broken.
	side := min(bw, bh)
	side -= max(2, side/100) * 2
	x, y := box.Min.X+(bw-side)/2, box.Min.Y+(bh-side)/2
	return image.Rect(x, y, x+side, y+side), true
}

func rgb(c color.Color) (int, int, int) {
	r, g, b, _ := c.RGBA()
	return int(r >> 8), int(g >> 8), int(b >> 8)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// squareCover takes the cover out of an audio file and, if it's album art between bars,
// puts back just the art. The audio is not re-encoded. Any failure leaves the file as it was.
func squareCover(path string) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".mp3" && ext != ".m4a" {
		return
	}
	dir, name := filepath.Dir(path), filepath.Base(path)
	pic := filepath.Join(dir, "~cover-"+name+".img")
	defer os.Remove(pic)
	run := func(args ...string) error {
		cmd := exec.Command(tools.ffmpeg, append([]string{"-y", "-hide_banner", "-loglevel", "error"}, args...)...)
		hideWindow(cmd)
		return cmd.Run()
	}
	// No cover (the thumbnail failed) → ffmpeg fails here, and that's the end of it.
	if run("-i", path, "-map", "0:v:0", "-c", "copy", "-f", "image2", pic) != nil {
		return
	}
	data, err := os.ReadFile(pic)
	if err != nil {
		return
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return
	}
	box, ok := artBox(img)
	if !ok {
		return
	}
	sub, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	})
	if !ok {
		return
	}
	var buf bytes.Buffer
	if jpeg.Encode(&buf, sub.SubImage(box), &jpeg.Options{Quality: 93}) != nil {
		return
	}
	art := filepath.Join(dir, "~cover-"+name+".jpg")
	defer os.Remove(art)
	if os.WriteFile(art, buf.Bytes(), 0o644) != nil {
		return
	}

	putCover(path, art)
}

// putCover replaces the cover of an audio file with the picture in the file pic.
// The audio is not re-encoded; on any failure the file stays as it was.
func putCover(path, pic string) bool {
	dir, name := filepath.Dir(path), filepath.Base(path)
	tmp := filepath.Join(dir, "~"+name) // same extension: ffmpeg picks the format by it
	args := []string{"-y", "-hide_banner", "-loglevel", "error",
		"-i", path, "-i", pic, "-map", "0:a", "-map", "1:0", "-c", "copy",
		"-disposition:v:0", "attached_pic", "-metadata:s:v", "comment=Cover (front)"}
	if strings.EqualFold(filepath.Ext(path), ".mp3") {
		args = append(args, "-id3v2_version", "3")
	}
	cmd := exec.Command(tools.ffmpeg, append(args, tmp)...)
	hideWindow(cmd)
	if cmd.Run() != nil {
		os.Remove(tmp)
		return false
	}
	return os.Rename(tmp, path) == nil
}

// ---------------------------------------------------------------------------
//  Real album covers from the Apple Music catalogue
// ---------------------------------------------------------------------------

// song: what yt-dlp knows about a finished audio file (after the tag clean-up).
type song struct {
	Filepath string `json:"filepath"`
	Artist   string `json:"artist"`
	Title    string `json:"title"`
	Album    string `json:"album"`
}

var (
	itunesURL = "https://itunes.apple.com/search" // a variable so tests can point it elsewhere
	web       = &http.Client{Timeout: 12 * time.Second}
	artCache  sync.Map // search term → cover bytes (nil = not found), so a playlist asks once per album
)

var bracketRe = regexp.MustCompile(`\([^)]*\)|\[[^\]]*\]|【[^】]*】|「|」`)

// norm makes names comparable: lower case, no brackets ("(feat. X)", "[Remastered]"),
// no punctuation. Letters of any script stay, so Japanese titles compare too.
func norm(s string) string {
	s = strings.ToLower(bracketRe.ReplaceAllString(s, " "))
	if i := strings.Index(s, " feat"); i > 0 {
		s = s[:i]
	}
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}), " ")
}

// same: the two names are the same thing, maybe with something extra on one side
// ("Daft Punk" and "Daft Punk & Pharrell Williams").
func same(a, b string) bool {
	a, b = norm(a), norm(b)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	// Whole words only: "one" is not inside "someone else".
	short := min(len([]rune(a)), len([]rune(b)))
	wa, wb := " "+a+" ", " "+b+" "
	return short >= 3 && (strings.Contains(wa, wb) || strings.Contains(wb, wa))
}

// albumArt finds the real cover. With a title it looks for that song and takes its album's
// cover; with only an album (a full album split by chapters) it looks for the album.
// nil when nothing matches clearly enough, or there's no internet.
func albumArt(artist, title, album string) []byte {
	if norm(artist) == "" || (norm(title) == "" && norm(album) == "") {
		return nil
	}
	entity, name := "song", title
	if title == "" {
		entity, name = "album", album
	}
	term := artist + " " + name
	key := entity + "|" + norm(term)
	if v, ok := artCache.Load(key); ok {
		return v.([]byte)
	}
	var art []byte
	defer func() { artCache.Store(key, art) }()

	q := url.Values{"term": {term}, "media": {"music"}, "entity": {entity}, "limit": {"10"}}
	body := get(itunesURL + "?" + q.Encode())
	var res struct {
		Results []struct {
			ArtistName     string `json:"artistName"`
			TrackName      string `json:"trackName"`
			CollectionName string `json:"collectionName"`
			ArtworkURL100  string `json:"artworkUrl100"`
		} `json:"results"`
	}
	if body == nil || json.Unmarshal(body, &res) != nil {
		return nil
	}
	for _, r := range res.Results {
		got := r.TrackName
		if entity == "album" {
			got = r.CollectionName
		}
		if !same(r.ArtistName, artist) || !same(got, name) || r.ArtworkURL100 == "" {
			continue
		}
		// The address says the size: ask for 1200×1200 instead of 100×100.
		big := strings.Replace(r.ArtworkURL100, "100x100bb", "1200x1200bb", 1)
		for _, u := range []string{big, r.ArtworkURL100} {
			if pic := get(u); pic != nil {
				if _, _, err := image.DecodeConfig(bytes.NewReader(pic)); err == nil {
					art = pic
					return art
				}
			}
		}
	}
	return nil
}

// get downloads a small thing from the internet; nil on any problem.
func get(u string) []byte {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "TYAGA (github.com/nadludyna/tyaga)")
	resp, err := web.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil
	}
	return b
}

// realCover puts the album's real cover into an audio file. False if there's none.
func realCover(path string, art []byte) bool {
	if art == nil {
		return false
	}
	pic := filepath.Join(filepath.Dir(path), "~art-"+filepath.Base(path)+".jpg")
	defer os.Remove(pic)
	return os.WriteFile(pic, art, 0o644) == nil && putCover(path, pic)
}

// fixCover: the real album cover if there is one, otherwise the thumbnail, squared if it's art between bars.
// Returns true when the real cover went in.
func fixCover(path string, art []byte) bool {
	if realCover(path, art) {
		return true
	}
	squareCover(path)
	return false
}
