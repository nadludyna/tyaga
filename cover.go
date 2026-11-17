package main

// Track covers. yt-dlp embeds the author's thumbnail as it is (16:9). Then, for every
// finished audio file, the cover is looked at: if it is an album cover sitting in the middle
// of plain bars (how YouTube shows "Artist - Topic" tracks and most music uploads), the bars
// are cut off and a square cover goes back in. A normal 16:9 thumbnail — a clip, a podcast —
// is left whole, so nothing of the author's picture is cut away.

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png" // the embedded cover can be PNG too
	"os"
	"os/exec"
	"path/filepath"
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

	tmp := filepath.Join(dir, "~"+name) // same extension: ffmpeg picks the format by it
	args := []string{"-i", path, "-i", art, "-map", "0:a", "-map", "1:0", "-c", "copy",
		"-disposition:v:0", "attached_pic", "-metadata:s:v", "comment=Cover (front)"}
	if ext == ".mp3" {
		args = append(args, "-id3v2_version", "3")
	}
	if run(append(args, tmp)...) == nil {
		os.Rename(tmp, path)
	} else {
		os.Remove(tmp)
	}
}
