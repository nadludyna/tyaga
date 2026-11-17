package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
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
