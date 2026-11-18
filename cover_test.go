package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// picture: a w×h frame filled with bg, with a busy (striped) square of side art in the middle.
// art = 0: the whole frame is busy, like a normal video thumbnail.
func picture(w, h, art int, bg color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	r := img.Bounds()
	if art > 0 {
		r = image.Rect((w-art)/2, (h-art)/2, (w+art)/2, (h+art)/2)
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 7), uint8(y * 5), uint8((x + y) * 3), 255})
		}
	}
	return img
}

// jpegged: through JPEG and back, so the edges get the usual compression noise.
func jpegged(t *testing.T, img image.Image) image.Image {
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	out, err := jpeg.Decode(&b)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestArtBox(t *testing.T) {
	black, white := color.RGBA{0, 0, 0, 255}, color.RGBA{250, 250, 250, 255}
	cases := []struct {
		name string
		img  image.Image
		crop bool
		side int
	}{
		{"art between black bars", picture(1280, 720, 720, black), true, 720},
		{"smaller art in the middle", picture(1280, 720, 500, white), true, 500},
		{"4:3 with bars on all sides", picture(480, 360, 270, black), true, 270},
		{"normal video thumbnail", picture(1280, 720, 0, black), false, 0},
		{"already square", picture(600, 600, 0, black), false, 0},
		{"one colour", picture(1280, 720, 1, black), false, 0},
	}
	for _, c := range cases {
		for _, img := range []image.Image{c.img, jpegged(t, c.img)} {
			box, ok := artBox(img)
			if ok != c.crop {
				t.Errorf("%s: crop = %v, want %v (box %v)", c.name, ok, c.crop, box)
				continue
			}
			if ok && (box.Dx() != box.Dy() || box.Dx() < c.side*96/100 || box.Dx() > c.side+4) {
				t.Errorf("%s: box %v, want a square of about %d", c.name, box, c.side)
			}
		}
	}
}

// TestSquareCover: a real MP3 and M4A with album art between bars come out with a square cover.
func TestSquareCover(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	tools.ffmpeg = ffmpeg
	dir := t.TempDir()
	thumb := filepath.Join(dir, "thumb.jpg")
	var b bytes.Buffer
	jpeg.Encode(&b, picture(640, 360, 360, color.RGBA{0, 0, 0, 255}), nil)
	os.WriteFile(thumb, b.Bytes(), 0o644)

	for _, ext := range []string{".mp3", ".m4a"} {
		song := filepath.Join(dir, "Song"+ext)
		args := []string{"-v", "error", "-y", "-f", "lavfi", "-i", "sine=d=1", "-i", thumb,
			"-map", "0:a", "-map", "1:0", "-c:v", "copy", "-disposition:v:0", "attached_pic"}
		if ext == ".m4a" {
			args = append(args, "-c:a", "aac")
		}
		if out, err := exec.Command(ffmpeg, append(args, song)...).CombinedOutput(); err != nil {
			t.Fatalf("making %s: %v %s", song, err, out)
		}
		squareCover(song)

		pic := filepath.Join(dir, "out"+ext+".jpg")
		if out, err := exec.Command(ffmpeg, "-v", "error", "-y", "-i", song, "-map", "0:v:0", "-c", "copy", "-f", "image2", pic).CombinedOutput(); err != nil {
			t.Fatalf("%s: no cover after: %v %s", ext, err, out)
		}
		f, _ := os.Open(pic)
		cfg, _, err := image.DecodeConfig(f)
		f.Close()
		if err != nil || cfg.Width != cfg.Height || cfg.Width < 350 {
			t.Errorf("%s: cover %dx%d (%v), want a ~360 square", ext, cfg.Width, cfg.Height, err)
		}
		if out, err := exec.Command(ffmpeg, "-v", "error", "-i", song, "-map", "0:a", "-f", "null", "-").CombinedOutput(); err != nil {
			t.Errorf("%s: audio broken: %v %s", ext, err, out)
		}
	}
}

func TestSame(t *testing.T) {
	yes := [][2]string{
		{"Daft Punk", "daft punk"},
		{"Daft Punk & Pharrell Williams", "Daft Punk"},
		{"Get Lucky (feat. Pharrell Williams) [Radio Edit]", "Get Lucky"},
		{"魅力的なBEMANI", "魅力的なBEMANI"},
		{"Song — Remastered 2011", "Song Remastered 2011"},
	}
	no := [][2]string{
		{"Daft Punk", "Justice"},
		{"Lucky", "Get Lucky Tonight Forever"[:0]}, // empty never matches
		{"AB", "ABBA"},                             // too short to count as "contains"
		{"One", "Someone Else"},
	}
	for _, c := range yes {
		if !same(c[0], c[1]) {
			t.Errorf("same(%q, %q) = false, want true", c[0], c[1])
		}
	}
	for _, c := range no {
		if same(c[0], c[1]) {
			t.Errorf("same(%q, %q) = true, want false", c[0], c[1])
		}
	}
}

// TestAlbumArt: a fake catalogue. The wrong artist is skipped, the 1200×1200 picture is asked for.
func TestAlbumArt(t *testing.T) {
	var b bytes.Buffer
	jpeg.Encode(&b, picture(40, 40, 0, color.RGBA{0, 0, 0, 255}), nil)
	pic := b.Bytes()
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.String())
		switch {
		case r.URL.Path == "/search":
			base := "http://" + r.Host
			fmt.Fprintf(w, `{"results":[
				{"artistName":"Someone Else","trackName":"Get Lucky","artworkUrl100":"%s/wrong/100x100bb.jpg"},
				{"artistName":"Daft Punk & Pharrell Williams","trackName":"Get Lucky (feat. Pharrell Williams)","collectionName":"Random Access Memories","artworkUrl100":"%s/ram/100x100bb.jpg"}]}`, base, base)
		case r.URL.Path == "/ram/1200x1200bb.jpg":
			w.Write(pic)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	saved := itunesURL
	itunesURL = srv.URL + "/search"
	defer func() { itunesURL = saved }()

	if got := albumArt("Daft Punk", "Get Lucky", ""); !bytes.Equal(got, pic) {
		t.Fatalf("no cover found; asked %v", asked)
	}
	for _, a := range asked {
		if strings.Contains(a, "/wrong/") {
			t.Errorf("took the cover of the wrong artist: %s", a)
		}
	}
	if got := albumArt("Nobody Known", "Nothing", ""); got != nil {
		t.Errorf("a cover for a song that isn't there")
	}
	if got := albumArt("", "Get Lucky", ""); got != nil {
		t.Errorf("a cover without an artist: too easy to get a wrong one")
	}
}

// TestPickReleases: one song on many releases. The known album wins; without it, the
// earliest release of exactly that song; compilations of various artists never.
func TestPickReleases(t *testing.T) {
	all := []release{
		{ArtistName: "Daft Punk", CollectionArtistName: "Various Artists", TrackName: "Get Lucky", CollectionName: "Now 85", ReleaseDate: "2013-01-01", ArtworkURL100: "now"},
		{ArtistName: "Daft Punk", TrackName: "Get Lucky (Radio Edit)", CollectionName: "Get Lucky - Single", ReleaseDate: "2013-04-19", ArtworkURL100: "edit"},
		{ArtistName: "Daft Punk", TrackName: "Get Lucky", CollectionName: "Random Access Memories (10th Anniversary Edition)", ReleaseDate: "2023-05-12", ArtworkURL100: "anniv"},
		{ArtistName: "Daft Punk", TrackName: "Get Lucky", CollectionName: "Random Access Memories", ReleaseDate: "2013-05-17", ArtworkURL100: "ram"},
		{ArtistName: "Daft Punk", TrackName: "Get Lucky", CollectionName: "Get Lucky - Single", ReleaseDate: "2013-04-19", ArtworkURL100: "single"},
	}
	first := func(album string) string {
		got := pickReleases(all, "song", "Daft Punk", "Get Lucky", album)
		if len(got) == 0 {
			return ""
		}
		return got[0].ArtworkURL100
	}
	if got := first("Random Access Memories"); got != "ram" {
		t.Errorf("album known: got %q, want the album's cover", got)
	}
	if got := first("Get Lucky"); got != "single" {
		t.Errorf("the single as the album: got %q, want the single's cover", got)
	}
	if got := first("Some Bootleg"); got != "" {
		t.Errorf("album not in the catalogue: got %q, want none (the thumbnail stays)", got)
	}
	if got := first(""); got != "single" {
		t.Errorf("no album: got %q, want the earliest exact release", got)
	}
}

// TestPickReleasesOrbital: real results (Apple, 2026-11) for a track of Orbital's "In Sides"
// album on YouTube Music. The compilation "20" comes first; it used to win.
func TestPickReleasesOrbital(t *testing.T) {
	all := []release{
		{ArtistName: "Orbital", TrackName: "The Girl with the Sun in Her Head", CollectionName: "20", ReleaseDate: "1997-04-21T07:00:00Z", ArtworkURL100: "20"},
		{ArtistName: "Orbital", TrackName: "The Girl with the Sun in Her Head (Floex Remix)", CollectionName: "30 Something", ReleaseDate: "1991-01-01T08:00:00Z", ArtworkURL100: "30"},
		{ArtistName: "Orbital", TrackName: "The Girl with the Sun in Her Head", CollectionName: "In Sides", ReleaseDate: "1997-04-21T07:00:00Z", ArtworkURL100: "insides"},
		{ArtistName: "Orbital", TrackName: "The Girl with the Sun in Her Head", CollectionName: "A Beginner's Guide (Best Of)", ReleaseDate: "1997-04-21T07:00:00Z", ArtworkURL100: "best"},
	}
	got := pickReleases(all, "song", "Orbital", "The Girl With The Sun In Her Head", "In Sides")
	if len(got) == 0 || got[0].ArtworkURL100 != "insides" {
		t.Errorf("In Sides: got %v, want the In Sides cover", got)
	}
	got = pickReleases(all, "song", "Orbital", "The Girl With The Sun In Her Head", "Orbital 20")
	if len(got) == 0 || got[0].ArtworkURL100 != "20" {
		t.Errorf("Orbital 20: got %v, want the cover of \"20\"", got)
	}
}

// TestFirstAlbum: a plain video, no album known. Every Orbital result says 1997 for the song;
// the catalogue's album dates show In Sides (1996) came before the compilation 20 (2009).
func TestFirstAlbum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lookup" || r.URL.Query().Get("id") != "20,1,30" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"results":[
			{"wrapperType":"collection","collectionId":20,"releaseDate":"2009-01-01T08:00:00Z"},
			{"wrapperType":"collection","collectionId":1,"releaseDate":"1996-01-01T08:00:00Z"},
			{"wrapperType":"collection","collectionId":30,"releaseDate":"1995-01-01T08:00:00Z"}]}`)
	}))
	defer srv.Close()
	saved := itunesURL
	itunesURL = srv.URL + "/search"
	defer func() { itunesURL = saved }()

	all := []release{
		{ArtistName: "Orbital", TrackName: "The Girl with the Sun in Her Head", CollectionName: "20", CollectionID: 20, ReleaseDate: "1997-04-21T07:00:00Z", ArtworkURL100: "20"},
		{ArtistName: "Orbital", TrackName: "The Girl with the Sun in Her Head", CollectionName: "In Sides", CollectionID: 1, ReleaseDate: "1997-04-21T07:00:00Z", ArtworkURL100: "insides"},
		{ArtistName: "Orbital", TrackName: "The Girl with the Sun in Her Head - Single", CollectionName: "The Girl with the Sun in Her Head - Single", CollectionID: 30, ReleaseDate: "1997-04-21T07:00:00Z", ArtworkURL100: "single"},
		{ArtistName: "Orbital", TrackName: "The Girl with the Sun in Her Head (Floex Remix)", CollectionName: "30 Something", ReleaseDate: "1991-01-01T08:00:00Z", ArtworkURL100: "remix"},
	}
	got := firstAlbum(pickReleases(all, "song", "Orbital", "The Girl With The Sun In Her Head", ""))
	var order []string
	for _, r := range got {
		order = append(order, r.ArtworkURL100)
	}
	if strings.Join(order, " ") != "insides 20 single" {
		t.Errorf("order %v, want In Sides, then 20, then the single; no remix", order)
	}
}
