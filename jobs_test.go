package main

import (
	"slices"
	"strings"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		link     string
		playlist bool
		fixed    string
		list     bool
		mix      bool
	}{
		{"https://www.youtube.com/watch?v=abc", true, "https://www.youtube.com/watch?v=abc", false, false},
		{"https://www.youtube.com/watch?v=abc&list=PLx", false, "https://www.youtube.com/watch?v=abc&list=PLx", false, false},
		{"https://www.youtube.com/watch?v=abc&list=PLx", true, "https://www.youtube.com/watch?v=abc&list=PLx", true, false},
		{"https://www.youtube.com/playlist?list=PLx", false, "https://www.youtube.com/playlist?list=PLx", true, false},
		{"https://www.youtube.com/@someone", false, "https://www.youtube.com/@someone/videos", true, false},
		{"https://youtube.com/channel/UC123/", false, "https://youtube.com/channel/UC123/videos", true, false},
		{"https://www.youtube.com/@someone/shorts", false, "https://www.youtube.com/@someone/shorts", true, false},
		{"https://m.youtube.com/watch?v=abc&list=RDabc", true, "https://m.youtube.com/watch?v=abc&list=RDabc", true, true},
		{"https://music.youtube.com/playlist?list=RDCLAK5uy_x", false, "https://music.youtube.com/playlist?list=RDCLAK5uy_x", true, false},
		{"https://youtu.be/abc?list=PLx", true, "https://youtu.be/abc?list=PLx", true, false},
		{"https://soundcloud.com/someone/sets/album", true, "https://soundcloud.com/someone/sets/album", true, false},
	}
	for _, c := range cases {
		fixed, list, mix := classify(c.link, c.playlist)
		if fixed != c.fixed || list != c.list || mix != c.mix {
			t.Errorf("classify(%q, %v) = %q %v %v, want %q %v %v", c.link, c.playlist, fixed, list, mix, c.fixed, c.list, c.mix)
		}
	}
}

// value returns what follows flag in args ("" if the flag isn't there).
func value(args []string, flag string) string {
	if i := slices.Index(args, flag); i >= 0 && i+1 < len(args) {
		return args[i+1]
	}
	return ""
}

func TestBuildArgs(t *testing.T) {
	video := &Job{URL: "https://youtu.be/abc", Options: Options{Mode: "video", Quality: 720}}
	args := buildArgs(video)
	if got := value(args, "-S"); got != "res:720" {
		t.Errorf("quality: -S %q", got)
	}
	if !slices.Contains(args, "--no-playlist") || slices.Contains(args, "--sponsorblock-remove") {
		t.Errorf("single video without sponsors: %v", args)
	}
	if args[len(args)-2] != "--" || args[len(args)-1] != video.URL {
		t.Errorf("the link must come last, after --: %v", args[len(args)-2:])
	}

	merged := &Job{URL: "https://www.youtube.com/watch?v=a&list=RDa", List: true, mix: true,
		Options: Options{Mode: "video", Merge: true, Sponsors: true}}
	args = buildArgs(merged)
	if got := value(args, "-S"); got != "vcodec:h264,res,acodec:aac" {
		t.Errorf("merge prefers H.264: -S %q", got)
	}
	if got := value(args, "--playlist-items"); got != "1:25" {
		t.Errorf("mix limit: %q", got)
	}
	if got := value(args, "--sponsorblock-remove"); got != "sponsor,selfpromo,interaction" {
		t.Errorf("video sponsors: %q", got)
	}
	if !strings.Contains(strings.Join(args, " "), "after_move:TRACK|") {
		t.Error("merge must ask yt-dlp for TRACK lines")
	}

	audio := &Job{URL: "https://www.youtube.com/playlist?list=PLx", List: true,
		Options: Options{Mode: "audio", Format: "mp3", Split: true, Merge: true, Sponsors: true}}
	args = buildArgs(audio)
	if got := value(args, "--sponsorblock-remove"); !strings.HasSuffix(got, ",music_offtopic") {
		t.Errorf("audio sponsors: %q", got)
	}
	if slices.Contains(args, "--split-chapters") {
		t.Error("no splitting by chapters when gluing into one file")
	}
	if got := value(args, "--audio-format"); got != "mp3" {
		t.Errorf("audio format: %q", got)
	}
}

func TestOnLine(t *testing.T) {
	j := &Job{}
	onLine(j, "ITEM|3|12|A | title with bars")
	if j.Title != "[3/12] A | title with bars" {
		t.Errorf("title: %q", j.Title)
	}
	onLine(j, "PROG|512|1024|NA|1048576|65|NA|NA")
	if j.Progress != 50 || j.Speed != "1.0 MB/s" || j.ETA != "1:05" {
		t.Errorf("progress: %v %q %q", j.Progress, j.Speed, j.ETA)
	}
	onLine(j, "PROG|NA|NA|NA|NA|NA|NA|NA") // unknown values must not break anything
	onLine(j, "FILE|/out/My list/003 - Song.m4a")
	if len(j.Files) != 1 || j.Files[0] != "003 - Song.m4a" {
		t.Errorf("files: %v", j.Files)
	}
	onLine(j, `TRACK|{"filepath": "/out/x.m4a", "title": "Song \"1\"", "artist": "Band", "playlist_title": "List"}`)
	if len(j.tracks) != 1 || j.tracks[0].Title != `Song "1"` {
		t.Errorf("tracks: %+v", j.tracks)
	}
	onLine(j, "ERROR: [youtube] abc: Sign in to confirm you're not a bot")
	if !strings.Contains(j.Error, "cookies") {
		t.Errorf("error: %q", j.Error)
	}
}

func TestEscapes(t *testing.T) {
	if got := metaEscape("a=b;c#d\\e\nf"); got != `a\=b\;c\#d\\e f` {
		t.Errorf("metaEscape: %q", got)
	}
	if got := concatEscape("It's"); got != `It'\''s` {
		t.Errorf("concatEscape: %q", got)
	}
}

func TestExpectedSum(t *testing.T) {
	list := "02f974af9f48745988fdf9eb07ba6bd1ee85a26fd240c79ba4ffec13ed32c882  a.zip\n" +
		"64bd76725aaf89def5d78ec79ea42b713324bde7a4634e115cd958f0e8ca02d2 *b.zip\n"
	if got := expectedSum(list, "b.zip"); got != "64bd76725aaf89def5d78ec79ea42b713324bde7a4634e115cd958f0e8ca02d2" {
		t.Errorf("list: %q", got)
	}
	if got := expectedSum(list, "c.zip"); got != "" {
		t.Errorf("a file that isn't listed: %q", got)
	}
	powershell := "\r\nAlgorithm : SHA256\r\nHash      : CBA84B26E0DF5AE7F74FC66B86A95B866872A86292EBD84FA1E959927EDD93BF\r\nPath      : C:\\a\\deno.zip\r\n"
	if got := expectedSum(powershell, "deno.zip"); got != "CBA84B26E0DF5AE7F74FC66B86A95B866872A86292EBD84FA1E959927EDD93BF" {
		t.Errorf("single hash: %q", got)
	}
}

func TestCut(t *testing.T) {
	good := []struct {
		from, to        string
		section, suffix string
	}{
		{"1:20", "3:45", "*80-225", " [1m20s-3m45s]"},
		{"90", "", "*90-inf", " [1m30s-end]"},
		{"", "0:45", "*0-45", " [0s-45s]"},
		{"1:02:03", "1:05:00.5", "*3723-3900.5", " [1h02m03s-1h05m00s]"},
	}
	for _, c := range good {
		section, suffix, ok := Options{From: c.from, To: c.to}.cut()
		if !ok || section != c.section || suffix != c.suffix {
			t.Errorf("cut(%q, %q) = %q %q %v", c.from, c.to, section, suffix, ok)
		}
	}
	if _, _, ok := (Options{}).cut(); ok {
		t.Error("no times must mean no fragment")
	}
	for _, bad := range [][2]string{{"3:00", "1:00"}, {"1:75", ""}, {"abc", ""}, {"-5", ""}, {"1:2:3:4", ""}, {"1:00", "1:00"}} {
		if _, _, ok := cutRange(bad[0], bad[1]); ok {
			t.Errorf("cutRange(%q, %q) should be rejected", bad[0], bad[1])
		}
	}

	args := buildArgs(&Job{URL: "https://youtu.be/a", Options: Options{Mode: "audio", Format: "m4a", Split: true, From: "1:20", To: "3:45"}})
	if value(args, "--download-sections") != "*80-225" || !slices.Contains(args, "--force-keyframes-at-cuts") {
		t.Errorf("fragment args: %v", args)
	}
	if slices.Contains(args, "--split-chapters") {
		t.Error("a fragment isn't split by chapters")
	}
	if !strings.Contains(value(args, "-o"), "[1m20s-3m45s]") {
		t.Errorf("fragment name: %q", value(args, "-o"))
	}
	list := buildArgs(&Job{URL: "https://www.youtube.com/playlist?list=PLx", List: true, Options: Options{Mode: "video", From: "1:20"}})
	if slices.Contains(list, "--download-sections") {
		t.Error("no fragments for playlists")
	}
}

func TestRetryJob(t *testing.T) {
	mu.Lock()
	saved := jobs
	jobs = []*Job{
		{ID: 1, Status: "error", Error: "HTTP Error 403: Forbidden", Progress: 40, Files: []string{"a.mp3"}, paths: []string{"/x/a.mp3"}},
		{ID: 2, Status: "done", Files: []string{"b.mp3"}},
	}
	mu.Unlock()
	defer func() { mu.Lock(); jobs = saved; mu.Unlock() }()

	retryJob(1)
	retryJob(2) // a finished download stays as it is

	mu.Lock()
	defer mu.Unlock()
	if j := jobs[0]; j.Status != "queued" || j.Error != "" || j.Progress != 0 || len(j.Files) != 0 || j.paths != nil {
		t.Errorf("failed job after retry: %+v", *j)
	}
	if jobs[1].Status != "done" || len(jobs[1].Files) != 1 {
		t.Errorf("finished job was touched: %+v", *jobs[1])
	}
}

func TestThumbURL(t *testing.T) {
	cases := map[string]string{
		"Youtube|dQw4w9WgXcQ|https://i.ytimg.com/vi_webp/dQw4w9WgXcQ/maxresdefault.webp": "https://i.ytimg.com/vi/dQw4w9WgXcQ/mqdefault.jpg",
		"Soundcloud|123|https://i1.sndcdn.com/a.jpg":                                     "https://i1.sndcdn.com/a.jpg",
		"Generic|x|javascript:alert(1)":                                                  "",
		"Generic|x|NA":                                                                   "",
		"broken":                                                                         "",
	}
	for in, want := range cases {
		if got := thumbURL(in); got != want {
			t.Errorf("thumbURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLog(t *testing.T) {
	j := &Job{}
	for _, l := range []string{
		"[youtube] Extracting URL: https://www.youtube.com/watch?v=abc",
		"[youtube] dQw4w9WgXcQ: Downloading webpage",
		"[youtube] dQw4w9WgXcQ: Downloading tv client config",
		"[youtube] dQw4w9WgXcQ: Downloading player 4fcd6e4a",
		"[info] Downloading video thumbnail 44 ...",
		`[info] Writing video thumbnail 44 to: C:\Users\Me\Downloads\YouTube\My list\01 - Song.webp`,
		"[debug] noise",
		"[download] Sleeping 5.00 seconds as required by the site...",
		`[download] Destination: C:\Users\Me\Downloads\YouTube\My list\01 - Song.webm`,
		"[download] 100% of 4.00MiB in 00:00:08 at 4.61MiB/s",
		"WARNING: something odd",
		`[ThumbnailsConvertor] Converting thumbnail "C:\Users\Me\Song.webp" to jpg`,
		"[ExtractAudio] Destination: /home/me/Downloads/YouTube/Song.mp3",
		"Deleting original file /home/me/Song.webm (pass -k to keep)",
		`[EmbedThumbnail] ffmpeg: Adding thumbnail to "/home/me/Song.mp3"`,
	} {
		onLine(j, l)
	}
	want := []string{
		"⚠ something odd",
		"Converting to Song.mp3",
		"Adding the cover",
	}
	if !slices.Equal(j.Log[1:], want) || j.Log[0] != "Downloading 01 - Song.webm" {
		t.Errorf("log = %q", j.Log)
	}
	for _, l := range j.Log {
		if strings.ContainsAny(l, `\/`) {
			t.Errorf("a path got into the log: %q", l)
		}
	}
	if logLine("[youtube] dQw4w9WgXcQ: Downloading webpage") != "Reading the video page" ||
		logLine("[download] Sleeping 5.00 seconds as required by the site...") != "" ||
		logLine(`[Merger] Merging formats into "/x/Video.mp4"`) != "Joining the picture and the sound" {
		t.Error("friendly lines changed")
	}
}

func TestWaiting(t *testing.T) {
	j := &Job{Speed: "1.0 MB/s"}
	onLine(j, "[download] Sleeping 5.00 seconds as required by the site...")
	if j.Wait != 5 || j.Speed != "" || j.Now != "Waiting: YouTube asks for a 5 s pause" {
		t.Errorf("during the pause: wait %d speed %q", j.Wait, j.Speed)
	}
	onLine(j, "PROG|1|10|NA|1|1|NA|NA")
	if j.Wait != 0 {
		t.Errorf("still waiting after a progress line")
	}
}

func TestProgressWholePlaylist(t *testing.T) {
	j := &Job{Options: Options{Mode: "audio"}}
	onLine(j, "POS|7|12")
	onLine(j, "ITEM|7|12|Song")
	if j.Progress != 50 {
		t.Errorf("start of the 7th of 12: %v, want 50", j.Progress)
	}
	onLine(j, "PROG|50|100|NA|1000|5|NA|NA")
	if j.Progress != 54.166666666666664 || j.ETA != "" {
		t.Errorf("7th half done: %v (eta %q), want ~54.2 and no eta", j.Progress, j.ETA)
	}
}

func TestProgressVideoTwoFiles(t *testing.T) {
	j := &Job{Options: Options{Mode: "video"}}
	onLine(j, "ITEM|||Video")
	onLine(j, "PROG|500|1000|NA|1|1|NA|NA")
	onLine(j, "PROG|1000|1000|NA|1|1|NA|NA")
	if j.Progress != 90 {
		t.Errorf("picture done: %v, want 90", j.Progress)
	}
	onLine(j, "PROG|10|100|NA|1|1|NA|NA") // the sound starts: must not jump back
	if j.Progress < 90 || j.Progress > 91 {
		t.Errorf("sound started: %v, want ~90.1", j.Progress)
	}
	onLine(j, "PROG|100|100|NA|1|1|NA|NA")
	if j.Progress != 100 {
		t.Errorf("all done: %v, want 100", j.Progress)
	}
}

func TestNextItem(t *testing.T) {
	j := &Job{Options: Options{Mode: "audio"}, Speed: "0.2 MB/s"}
	onLine(j, "[download] Downloading item 2 of 3")
	if j.Pos != 2 || j.Of != 3 || j.Speed != "" || j.Progress < 33.3 || j.Progress > 33.4 {
		t.Errorf("item 2 of 3: pos %d/%d speed %q progress %v", j.Pos, j.Of, j.Speed, j.Progress)
	}
}

func TestExplain(t *testing.T) {
	if got := explain("ERROR: [youtube] FAILabcdefg: unable to download video data: HTTP Error 403: Forbidden"); !strings.HasPrefix(got, "YouTube refused") {
		t.Errorf("403: %q", got)
	}
	if got := explain("ERROR: [youtube] dQw4w9WgXcQ: Video unavailable"); got != "Video unavailable" {
		t.Errorf("other error: %q", got)
	}
}

func TestSpeedText(t *testing.T) {
	if speedText(52428) != "51 KB/s" || speedText(4404019) != "4.2 MB/s" {
		t.Errorf("%q %q", speedText(52428), speedText(4404019))
	}
}

// A long video in pieces: the size guess starts far too low; the pieces count is what's true.
func TestProgressFragments(t *testing.T) {
	j := &Job{Options: Options{Mode: "video"}}
	onLine(j, "ITEM|||Long video")
	onLine(j, "PROG|3000000|NA|3100000|3000000|730|10|2041") // guess says 97%, pieces say 0.5%
	if j.Progress > 1 {
		t.Errorf("piece 10 of 2041: %v%%, want ~0.4", j.Progress)
	}
	onLine(j, "PROG|900000000|NA|950000000|3000000|300|1020|2041")
	if j.Progress < 44 || j.Progress > 46 {
		t.Errorf("piece 1020 of 2041: %v%%, want ~45 (half of the picture's 90)", j.Progress)
	}
	if logLine("[hlsnative] Total fragments: 2041") != "" || logLine("[info] dQw4w9WgXcQ: Downloading m3u8 manifest") != "" {
		t.Error("piece details got into the log")
	}
}

func TestFormatNumberHidden(t *testing.T) {
	if got := logLine(`[download] Destination: C:\\x\\Song.f299.mp4`); got != "Downloading Song.mp4" {
		t.Errorf("%q", got)
	}
}

func TestLiveLine(t *testing.T) {
	j := &Job{Options: Options{Mode: "video"}}
	onLine(j, "CHAN|Pilotredsky - Topic")
	onLine(j, "PROG|327155712|NA|9000000000|3000000|300|1020|2041")
	if j.Channel != "Pilotredsky" || j.Now != "Downloading the picture · piece 1 020 of 2 041 · 312 MB" {
		t.Errorf("channel %q, now %q", j.Channel, j.Now)
	}
	onLine(j, "[Merger] Merging formats into \"/x/V.mp4\"")
	if j.Now != "" {
		t.Errorf("live line stays after the step ended: %q", j.Now)
	}
}
