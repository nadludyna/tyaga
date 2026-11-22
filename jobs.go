package main

// The download queue: builds the yt-dlp command, runs downloads one at a time,
// reads the output and tags the tracks that were split by chapters.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Options are the switches on the page, the same for every link added together.
type Options struct {
	Mode     string `json:"mode"`     // "video" or "audio"
	Format   string `json:"format"`   // audio: "m4a" or "mp3"
	Quality  int    `json:"quality"`  // video: max height (1440, 1080, 720), 0 = the best there is
	Playlist bool   `json:"playlist"` // download the whole playlist if the link points to one
	Split    bool   `json:"split"`    // audio: split into separate tracks by chapters
	Edit     bool   `json:"edit"`     // video: H.264 + AAC for video editors
	Merge    bool   `json:"merge"`    // playlist: glue everything into one file with chapters
	Sponsors bool   `json:"sponsors"` // cut sponsor segments (and non-music parts of music videos)
	From     string `json:"from"`     // fragment: download only from…to ("1:20", "0:45", "1:02:03"); empty = start/end
	To       string `json:"to"`
}

// parseTime reads "1:02:03", "1:20", "80", "80.5" or "1:20.5" as seconds.
func parseTime(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	var total float64
	parts := strings.Split(s, ":")
	if len(parts) > 3 {
		return 0, false
	}
	for i, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil || v < 0 || math.IsInf(v, 0) || math.IsNaN(v) || (i > 0 && v >= 60) ||
			(i < len(parts)-1 && v != math.Trunc(v)) { // "1.5:20": only the seconds may have a fraction
			return 0, false
		}
		total = total*60 + v
	}
	return total, true
}

// cutRange checks the fragment times. from = 0 and to = -1 mean "from the start" / "to the end".
// ok is false only for times that make no sense; no fragment at all is fine.
func cutRange(from, to string) (start, end float64, ok bool) {
	start, end = 0, -1
	if strings.TrimSpace(from) != "" {
		if start, ok = parseTime(from); !ok {
			return 0, 0, false
		}
	}
	if strings.TrimSpace(to) != "" {
		if end, ok = parseTime(to); !ok || end <= start {
			return 0, 0, false
		}
	}
	return start, end, true
}

// span: seconds → "1m20s", "1h02m03s", "45s" (for file names, so no ":").
func span(sec float64) string {
	s := int(sec)
	switch {
	case s >= 3600:
		return fmt.Sprintf("%dh%02dm%02ds", s/3600, s/60%60, s%60)
	case s >= 60:
		return fmt.Sprintf("%dm%02ds", s/60, s%60)
	}
	return fmt.Sprintf("%ds", s)
}

// cut returns the fragment of a job: whether there is one, and its yt-dlp section and name tail.
func (o Options) cut() (section, suffix string, ok bool) {
	start, end, valid := cutRange(o.From, o.To)
	if !valid || (start == 0 && end < 0) {
		return "", "", false
	}
	section, suffix = "*"+strconv.FormatFloat(start, 'f', -1, 64)+"-", " ["+span(start)+"-"
	if end < 0 {
		section, suffix = section+"inf", suffix+"end]"
	} else {
		section, suffix = section+strconv.FormatFloat(end, 'f', -1, 64), suffix+span(end)+"]"
	}
	return section, suffix, true
}

// Job is one download = one link (a video, a playlist or a channel).
type Job struct {
	ID  int    `json:"id"`
	URL string `json:"url"`
	Options
	List     bool     `json:"list"`     // downloads a list (playlist, channel), not one video
	Note     string   `json:"note"`     // something worth knowing about this link
	Status   string   `json:"status"`   // queued, running, done, error, cancelled
	Title    string   `json:"title"`    // title of the current video, numbered if it's a playlist
	Channel  string   `json:"channel"`  // who uploaded the current video
	Now      string   `json:"now"`      // what's happening this second, live: "Downloading the picture · piece 10 of 80 · 12 MB"
	Thumb    string   `json:"thumb"`    // preview picture of the current video
	Log      []string `json:"log"`      // the last few things yt-dlp said it's doing
	Progress float64  `json:"progress"` // 0–100 for the whole download (a playlist counts as one)
	Speed    string   `json:"speed"`
	Size     string   `json:"size"`  // how much of the current file: "3 MB of 5 MB", "8 MB so far" (pieces)
	Cover    int      `json:"cover"` // > 0: the finished file has a cover (/api/cover); changes when it does
	Wait     int      `json:"wait"`  // seconds YouTube asked yt-dlp to wait before downloading; 0 = not waiting
	Pos      int      `json:"pos"`   // in a playlist: which video this is (1…Of) and how many there are,
	Of       int      `json:"of"`    // so the progress goes once from 0 to 100% for the whole playlist
	ETA      string   `json:"eta"`
	Error    string   `json:"error"`
	ErrorRaw string   `json:"errorRaw"` // yt-dlp's own error line, for "Copy error"
	Files    []string `json:"files"`    // names of finished files
	Already  bool     `json:"already"`  // done without downloading anything: it was all in the folder already

	mix      bool            // a YouTube Mix: only the first mixLimit videos
	listName string          // the playlist's own name, for the finished row
	failed   int             // how many errors yt-dlp reported (in a playlist: videos it skipped)
	part     int             // video: 0 = the picture is downloading, 1 = the sound (a separate file)
	lastB    float64         // bytes downloaded at the last progress line, to notice a new file starting
	paths    []string        // full paths of finished files
	songs    map[string]song // audio: artist / title / album of each finished file, by path
	cover    []byte          // the picture inside the finished file
	splits   []splitInfo     // videos that were split by chapters
	tracks   []track         // playlist items to glue into one file (Merge)
	had, got bool            // yt-dlp found a file already there / downloaded something itself
}

// splitInfo is what yt-dlp reports about a video that was split by chapters.
type splitInfo struct {
	Filepath string `json:"filepath"`
	Album    string `json:"album"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Chapters []struct {
		Title string `json:"title"`
	} `json:"chapters"`
}

var (
	mu      sync.Mutex // guards everything below
	jobs    = []*Job{}
	nextID  = 1
	running *exec.Cmd // the current yt-dlp process; only one download runs at a time
	current *Job
)

// Covers: the author's thumbnail, the picture they chose for the video (not a random frame).
// MP3, M4A and MP4 can't hold WebP, so it's converted to JPEG. For music, album art between
// bars gets cropped to a square afterwards (cover.go).

// audioTags are clean tags for music. The rules run in order, before the download,
// so the cleaned title and artist end up both in the tags and in the file name.
var audioTags = []string{
	// Title: YouTube Music tracks have an exact "track" field, better than the video title.
	"--parse-metadata", "%(track,title)s:%(title)s",
	// Artist: from YouTube Music data, otherwise the channel name.
	// The trailing "|": if nothing is found, the artist is empty instead of a literal "NA".
	"--parse-metadata", "%(artist,creator,uploader|)s:%(artist)s",
	// "Artist - Song" in the title → artist and song separately.
	"--parse-metadata", `title:(?P<artist>.+?) [-–—] (?P<title>.+)`,
	// Channel name tails: "Someone - Topic", "SomeoneVEVO".
	"--replace-in-metadata", "artist", `(?i)\s*(- topic|vevo)$`, "",
	// Junk in brackets: (Official Video), [Lyrics], (Official Audio), (HD)…
	"--replace-in-metadata", "title",
	`(?i)\s*[(\[][^)\]]*\b(official|lyrics?|audio|video|visuali[sz]er|mv|hd|hq|4k)\b[^)\]]*[)\]]`, "",
	// Year: the release date if YouTube knows it, otherwise the upload year.
	"--parse-metadata", `%(release_year,upload_date)s:(?P<meta_date>\d{4})`,
	// Drop the video description and the YouTube link from the tags.
	"--parse-metadata", ":(?P<meta_description>)",
	"--parse-metadata", ":(?P<meta_synopsis>)",
	"--parse-metadata", ":(?P<meta_purl>)",
	"--parse-metadata", ":(?P<meta_comment>)",
}

var (
	// A channel without a tab (/@name, /channel/ID, /c/name, /user/name) → its "Videos" tab.
	channelRe = regexp.MustCompile(`^/(@[^/]+|channel/[^/]+|c/[^/]+|user/[^/]+)/?$`)
	// A channel tab is always a list.
	channelTabRe = regexp.MustCompile(`^/(@[^/]+|channel/[^/]+|c/[^/]+|user/[^/]+)/(videos|shorts|streams|playlists|podcasts|releases)/?$`)
)

// mixLimit: a YouTube Mix (list=RD…) is endless, so only its beginning is taken.
const mixLimit = 25

// classify works out what a link is: a single video or a list, and fixes up channel links.
// playlist is the "whole playlist" checkbox: it decides only for a video link that has a
// playlist attached (watch?v=…&list=…). A playlist or channel link is a list anyway.
func classify(link string, playlist bool) (fixed string, list, mix bool) {
	u, err := url.Parse(link)
	if err != nil {
		return link, false, false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	host = strings.TrimPrefix(host, "m.")
	if host != "youtube.com" && host != "music.youtube.com" && host != "youtu.be" {
		return link, playlist, false // another site: trust the checkbox, yt-dlp knows what to do
	}
	listID := u.Query().Get("list")
	// RD… is a Mix; RDCLAK… is a normal YouTube Music playlist that only looks like one.
	mix = strings.HasPrefix(listID, "RD") && !strings.HasPrefix(listID, "RDCLAK")
	switch {
	case channelRe.MatchString(u.Path):
		u.Path = strings.TrimSuffix(u.Path, "/") + "/videos"
		return u.String(), true, false
	case channelTabRe.MatchString(u.Path):
		return link, true, false
	case u.Path == "/playlist" && listID != "":
		return link, true, mix
	case listID != "" && playlist:
		return link, true, mix
	}
	return link, false, false
}

// buildArgs builds the yt-dlp command line for a download.
func buildArgs(j *Job) []string {
	args := []string{
		// Ignore any yt-dlp settings on the computer, use only what's set here.
		"--ignore-config", "--no-plugin-dirs",
		// Don't overwrite finished files, resume unfinished ones.
		"--no-overwrites", "--continue",
		// File names that are valid on Windows and no longer than 180 characters.
		"--windows-filenames", "--trim-filenames", "180",
		// Output that's easy to parse: no colors, every update on a new line.
		"--no-colors", "--newline", "--progress", "--progress-delta", "0.5", "--encoding", "utf-8",
		"--socket-timeout", "25", "--retries", "3",
		"--embed-metadata", "--embed-chapters",
		// Skip live streams: they never end.
		"--match-filter", "!is_live",
		"--ffmpeg-location", filepath.Dir(tools.ffmpeg),
		// Service lines that onLine() parses:
		"--print", "before_dl:POS|%(playlist_autonumber|)s|%(n_entries|)s",
		"--print", "before_dl:ITEM|%(playlist_index|)s|%(n_entries|)s|%(title)s",
		"--print", "before_dl:THUMB|%(extractor_key)s|%(id)s|%(thumbnail|)s",
		"--print", "before_dl:CHAN|%(channel,uploader|)s",
		"--print", "after_move:FILE|%(filepath)s",
		"--progress-template",
		// done | exact size | size estimate | speed | eta | fragment | of fragments
		"download:PROG|%(progress.downloaded_bytes)s|%(progress.total_bytes)s|%(progress.total_bytes_estimate)s|%(progress.speed)s|%(progress.eta)s|%(progress.fragment_index)s|%(progress.fragment_count)s",
		// --print turns on quiet mode, which hides what yt-dlp is doing ("[youtube] …",
		// "[download] Destination: …"). The page shows those lines as a log, so back on.
		"--no-quiet",
		"-P", outDir,
	}
	// YouTube requires running its JavaScript; yt-dlp needs Deno for that.
	if tools.deno != "" {
		args = append(args, "--js-runtimes", "deno:"+tools.deno)
	}

	// A playlist goes into its own folder with numbered files. A single video goes straight to the downloads folder.
	isList := j.List
	if isList {
		args = append(args, "--yes-playlist")
		if j.mix {
			args = append(args, "--playlist-items", fmt.Sprintf("1:%d", mixLimit))
		}
	} else {
		args = append(args, "--no-playlist")
	}
	single := "%(title)s"
	if j.Mode == "audio" {
		single = "%(artist,uploader&{} - |)s%(title)s" // "Artist - Song"; without an artist just "Song"
	}
	// Fragment (single videos only): just that piece, cut exactly at the given times.
	// The times go into the name, so it doesn't clash with the whole video downloaded before.
	section, suffix, cut := j.cut()
	cut = cut && !isList
	if cut {
		single += suffix
		args = append(args, "--download-sections", section, "--force-keyframes-at-cuts")
	}
	name := single
	if isList {
		name = "%(playlist_title)s/%(playlist_index)03d - %(title)s"
	}
	args = append(args, "-o", name+".%(ext)s")

	// "One file": after the download the pieces get glued together (merge.go), so here
	// yt-dlp only reports each finished piece. Splitting by chapters makes no sense then.
	merge := isList && j.Merge
	if merge {
		args = append(args, "--print", "after_move:TRACK|%(.{filepath,title,artist,uploader,playlist_title,playlist_index})j")
	}

	// SponsorBlock: cut the sponsor ads, self-promotion and "subscribe!" bits.
	// For music also the non-music parts of a clip (talking, intro scenes).
	if j.Sponsors {
		cut := "sponsor,selfpromo,interaction"
		if j.Mode == "audio" {
			cut += ",music_offtopic"
		}
		args = append(args, "--sponsorblock-remove", cut)
	}

	if j.Mode == "video" {
		// Best video + best audio in MP4, not higher than the chosen quality.
		// "For editing" prefers H.264 and AAC. For one file too: if every piece is
		// H.264 + AAC of one size, they can be glued without re-encoding.
		h264 := j.Edit || merge
		var sort []string
		if h264 {
			sort = append(sort, "vcodec:h264")
		}
		if j.Quality > 0 {
			sort = append(sort, fmt.Sprintf("res:%d", j.Quality))
		} else if h264 {
			sort = append(sort, "res")
		}
		if h264 {
			sort = append(sort, "acodec:aac")
		}
		if len(sort) > 0 {
			args = append(args, "-S", strings.Join(sort, ","))
		}
		args = append(args, "-f", "bv*+ba/b", "--merge-output-format", "mp4", "--remux-video", "mp4")
		// The thumbnail as the file's cover: Explorer and players show it instead of some frame.
		// Not for "one file": the pieces are glued by their first video stream.
		if !merge {
			args = append(args, "--embed-thumbnail", "--convert-thumbnails", "jpg")
		}
	} else {
		// For M4A take the AAC track right away, so it doesn't need re-encoding.
		if j.Format == "m4a" {
			args = append(args, "-f", "ba[ext=m4a]/ba/b")
		} else {
			args = append(args, "-f", "ba/b")
		}
		args = append(args, "-x", "--audio-format", j.Format, "--audio-quality", "0")
		args = append(args, "--embed-thumbnail", "--convert-thumbnails", "jpg")
		args = append(args, audioTags...)
		// Artist / song / album after the clean-up: to look up the real album cover (cover.go).
		args = append(args, "--print", "after_move:SONG|%(.{filepath,artist,title,album})j")
		if isList { // track number = position in the playlist
			args = append(args, "--parse-metadata", "%(playlist_index)s:%(track_number)s")
		}
		if j.Split && !merge && !cut {
			// A whole album as one video with chapters → a folder with separate tracks appears next to it.
			args = append(args, "--split-chapters",
				"-o", "chapter:"+name+"/%(section_number)02d - %(section_title)s.%(ext)s",
				"--print", "after_move:DONE|%(.{filepath,album,title,artist,chapters})j")
		}
	}

	// Signed in (login.go): from the browser, read afresh every time, or from the cookies file.
	if b := browserSource(); b != "" {
		args = append(args, "--cookies-from-browser", b)
	} else if fileExists(cookiesFile) {
		args = append(args, "--cookies", cookiesFile)
	}
	return append(args, "--", j.URL) // "--" so the link can't be taken for an option
}

// clock: seconds → "m:ss".
func clock(sec float64) string {
	s := int(sec)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

var (
	signInRe  = regexp.MustCompile(`(?i)sign in|not a bot|login required`)
	tooManyRe = regexp.MustCompile(`(?i)HTTP Error 429|too many requests`)
	sponsorRe = regexp.MustCompile(`(?i)sponsorblock`)
	privateRe = regexp.MustCompile(`(?i)private video`)
	goneRe    = regexp.MustCompile(`(?i)video unavailable|has been removed|account .* terminated`)
)

// explain turns the most common yt-dlp errors into plain language.
func explain(line string) string {
	switch {
	case privateRe.MatchString(line):
		return "The video is private."
	case goneRe.MatchString(line):
		return "The video was removed or isn't available."
	case sponsorRe.MatchString(line):
		return "SponsorBlock didn't answer. Untick \"no sponsors\" and add the link again."
	case signInRe.MatchString(line):
		if canLogin {
			return "YouTube wants you to sign in. Sign in, then Retry."
		}
		return "YouTube wants you to sign in. Put a fresh youtube-cookies.txt next to the program."
	case tooManyRe.MatchString(line):
		return "YouTube is limiting requests for now. Try again later."
	case strings.Contains(line, "HTTP Error 403"):
		return "YouTube refused the download (HTTP 403). It usually goes through on Retry."
	}
	// Whatever else: without "ERROR: [youtube] dQw4w9WgXcQ: " in front.
	line = strings.TrimPrefix(line, "ERROR: ")
	if m := tagRe.FindString(line); m != "" {
		line = idRe.ReplaceAllString(line[len(m):], "")
	}
	return line
}

// num parses a number from yt-dlp output. Unknown values are printed as "NA" → NaN.
func num(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return math.NaN()
	}
	return f
}

var ytID = regexp.MustCompile(`^[\w-]{11}$`)

// thumbURL picks a preview picture from "extractor|id|thumbnail". For YouTube a small
// 320×180 one is built from the id: the "thumbnail" yt-dlp gives is the full-size one.
// Only http(s) links get through, the page puts this straight into <img src>.
func thumbURL(s string) string {
	p := strings.SplitN(s, "|", 3)
	if len(p) < 3 {
		return ""
	}
	if strings.HasPrefix(p[0], "Youtube") && ytID.MatchString(p[1]) {
		return "https://i.ytimg.com/vi/" + p[1] + "/mqdefault.jpg"
	}
	if u, err := url.Parse(p[2]); err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" {
		return p[2]
	}
	return ""
}

// sizeText: "312 MB", "1.4 GB", "820 KB".
func sizeText(b float64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", b/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.0f MB", b/(1<<20))
	}
	return fmt.Sprintf("%.0f KB", b/1024)
}

// thousands: 2041 → "2 041", easier to read at a glance.
func thousands(f float64) string {
	s := strconv.Itoa(int(f))
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + " " + s[i:]
	}
	return s
}

// speedText: "4.2 MB/s", or "350 KB/s" when it's slow (not "0.0 MB/s").
func speedText(bytesPerSec float64) string {
	if bytesPerSec < 1048576 {
		return fmt.Sprintf("%.0f KB/s", bytesPerSec/1024)
	}
	return fmt.Sprintf("%.1f MB/s", bytesPerSec/1048576)
}

// overall turns how far the current file is (0…1) into the progress of the whole download:
// in a playlist of 12, the 7th video half done is (6 + 0.5) / 12.
func (j *Job) overall(frac float64) float64 {
	if j.Of > 1 && j.Pos >= 1 && j.Pos <= j.Of {
		return (float64(j.Pos-1) + frac) / float64(j.Of) * 100
	}
	return frac * 100
}

// onLine parses one line of yt-dlp output and updates the download. Call with mu held.
func onLine(j *Job, line string) {
	switch {
	case strings.HasPrefix(line, "ITEM|"):
		// A new video started. The title may contain "|", so split into at most 3 parts.
		p := strings.SplitN(line[5:], "|", 3)
		if len(p) < 3 {
			return
		}
		j.Title = p[2]
		if p[0] != "" && p[0] != "NA" {
			j.Title = "[" + p[0] + "/" + p[1] + "] " + p[2]
		}
		j.part, j.lastB = 0, 0
		j.Progress = j.overall(0)
	case strings.HasPrefix(line, "POS|"):
		p := strings.Split(line[4:], "|")
		if len(p) == 2 {
			j.Pos, j.Of = int(num(p[0])), int(num(p[1]))
		}
	case strings.HasPrefix(line, "CHAN|"):
		j.Channel = strings.TrimSuffix(strings.TrimSpace(line[5:]), " - Topic")
	case strings.HasPrefix(line, "THUMB|"):
		j.Thumb = thumbURL(line[6:])
	case strings.HasPrefix(line, "[ExtractAudio]") || strings.HasPrefix(line, "[Merger]"):
		j.Speed, j.ETA, j.Size = "", "", "" // downloading is over for this one, converting now
		j.addLog(line)
	case strings.HasPrefix(line, "PROG|"):
		j.Wait = 0
		j.got = true
		p := strings.Split(line[5:], "|")
		if len(p) < 7 {
			return
		}
		done, total, guess, speed, eta := num(p[0]), num(p[1]), num(p[2]), num(p[3]), num(p[4])
		frag, frags := num(p[5]), num(p[6])
		// A video comes as two files, the picture and then the sound: the bytes drop back
		// to almost nothing when the second one starts. The picture is the big one, so it
		// fills the bar to 90% and the sound does the rest, instead of 0→100% twice.
		if j.Mode == "video" && j.part == 0 && j.lastB > 0 && done < j.lastB/4 {
			j.part = 1
		}
		if done > 0 {
			j.lastB = done
		}
		// How far this file is. Downloaded in pieces (HLS, DASH), the size is only a guess that
		// starts far too low (a long video showed 90% at once), so count the pieces instead.
		f := math.NaN()
		switch {
		case frags > 0 && frag >= 0:
			f = frag / frags
		case total > 0:
			f = done / total
		case guess > 0:
			f = done / guess
		}
		// The live line: which file, how many parts, how much (always "of" what, or "so far").
		what := "Downloading the sound"
		if j.Mode == "video" && j.part == 0 {
			what = "Downloading the picture"
		}
		now := []string{what}
		if frags > 0 && frag >= 0 {
			now = append(now, fmt.Sprintf("part %s of %s", thousands(frag), thousands(frags)))
		}
		j.Size = ""
		switch {
		case done > 0 && total > 0:
			j.Size = sizeText(done) + " of " + sizeText(total)
		case done > 0: // in parts the full size isn't known yet
			j.Size = sizeText(done) + " so far"
		}
		if j.Size != "" {
			now = append(now, j.Size)
		}
		j.Now = strings.Join(now, " · ")
		if !math.IsNaN(f) {
			f = math.Min(1, f)
			if j.Mode == "video" {
				f = []float64{f * 0.9, 0.9 + f*0.1}[min(j.part, 1)]
			}
			j.Progress = math.Max(j.Progress, j.overall(f)) // never backwards within one video
		}
		j.Speed, j.ETA = "", ""
		if speed > 0 {
			j.Speed = speedText(speed)
		}
		if eta > 0 && j.Of <= 1 { // in a playlist it's only this video's time left: misleading
			j.ETA = clock(eta)
		}
	case strings.HasPrefix(line, "FILE|"):
		path := line[5:]
		j.paths = append(j.paths, path)
		j.Files = append(j.Files, filepath.Base(path))
	case strings.HasPrefix(line, "SONG|"):
		var s song
		if json.Unmarshal([]byte(line[5:]), &s) == nil && s.Filepath != "" {
			if j.songs == nil {
				j.songs = map[string]song{}
			}
			j.songs[s.Filepath] = s
		}
	case strings.HasPrefix(line, "TRACK|"):
		var t track
		if json.Unmarshal([]byte(line[6:]), &t) == nil && t.Filepath != "" {
			j.tracks = append(j.tracks, t)
		}
	case strings.HasPrefix(line, "DONE|"):
		var info splitInfo
		if json.Unmarshal([]byte(line[5:]), &info) == nil {
			j.splits = append(j.splits, info)
		}
	case strings.HasPrefix(line, "ERROR"):
		j.Error, j.ErrorRaw = explain(line), line
		j.failed++
		j.addLog(line)
	case strings.HasPrefix(line, "[download] Downloading playlist: "):
		j.listName = strings.TrimSpace(strings.TrimPrefix(line, "[download] Downloading playlist: "))
		j.addLog(line)
	case strings.HasPrefix(line, "[download] Downloading item "):
		// The next video of a playlist starts: the counter moves now, its title comes a bit later.
		var n, of int
		if _, err := fmt.Sscanf(line, "[download] Downloading item %d of %d", &n, &of); err == nil {
			j.Pos, j.Of = n, of
			j.Speed, j.ETA, j.Size, j.Wait = "", "", "", 0
			j.part, j.lastB = 0, 0
			j.Progress = math.Max(j.Progress, j.overall(0))
		}
	case strings.HasPrefix(line, "[download] ") && strings.HasSuffix(line, "has already been downloaded"):
		j.had = true // --no-overwrites: the file is in the folder from an earlier download
		j.addLog(line)
	case strings.HasPrefix(line, "[") || strings.HasPrefix(line, "WARNING:"):
		if m := sleepRe.FindStringSubmatch(line); m != nil && strings.HasPrefix(line, "[download]") {
			j.Wait, j.Speed, j.ETA, j.Size = int(num(m[1])+0.5), "", "", "" // nothing is downloading during the pause
			j.addLog(line)
			j.Now = fmt.Sprintf("Waiting: YouTube asks for a %d s pause", j.Wait)
			return
		}
		j.addLog(line)
	}
}

// logSize: how many recent lines the page shows under the progress wave.
const logSize = 4

// pathRe finds a full path in a log line (Windows or Unix, quoted or not) and keeps just the file name.
var pathRe = regexp.MustCompile(`(^|[\s"])(?:[A-Za-z]:[\\/]|/)[^"]*[\\/]([^"\\/]+)`)

// tagRe: "[youtube] dQw4w9WgXcQ: " at the start of a yt-dlp line.
var tagRe = regexp.MustCompile(`^\[([\w:]+)\] `)

// idRe: the video id that "[youtube]" and "[info]" lines start with ("dQw4w9WgXcQ: ").
var idRe = regexp.MustCompile(`^[\w-]{11}: `)

// formatRe: YouTube's format number yt-dlp puts in a file name while it downloads the parts.
var formatRe = regexp.MustCompile(`\.f\d+(\.\w+)\b`)

var sleepRe = regexp.MustCompile(`Sleeping ([\d.]+) seconds`)

// logLine turns a yt-dlp line into something a person reads in a second, or "" for the
// many lines that only matter to a developer (thumbnail files, metadata, deleting temp files).
func logLine(line string) string {
	line = strings.TrimSpace(line)
	tag := ""
	if m := tagRe.FindStringSubmatch(line); m != nil {
		tag, line = m[1], line[len(m[0]):]
		if tag == "youtube" || tag == "info" { // only these start with the id; "Destination" is 11 letters too
			line = idRe.ReplaceAllString(line, "")
		}
	}
	line = pathRe.ReplaceAllString(line, "$1$2")
	line = formatRe.ReplaceAllString(line, "$1") // "Song.f299.mp4" → "Song.mp4"
	has := func(s string) bool { return strings.Contains(line, s) }
	switch {
	case line == "" || tag == "debug":
		return ""
	case tag == "info" && has("thumbnail"), tag == "ThumbnailsConvertor", tag == "Metadata",
		strings.HasPrefix(line, "Deleting original file"), tag == "youtube:tab",
		has("Extracting URL"), strings.HasPrefix(line, "Downloading item "),
		strings.HasPrefix(line, "Finished downloading playlist"), tag == "download" && strings.HasPrefix(line, "100% of"):
		return ""
	case tag == "hlsnative" || tag == "dashsegments", has("m3u8 manifest"), strings.HasPrefix(line, "Total fragments"):
		return "" // how YouTube cuts the file into pieces: nothing a person needs
	case has("JS challenge"), strings.HasPrefix(line, "[jsc"):
		return "Passing YouTube's check" // yt-dlp runs YouTube's script with Deno to be let in
	case has("Downloading webpage"):
		return "Reading the video page"
	case has("client config"), has("Downloading player"), has("API JSON"), has("m3u8 information"):
		return "Asking YouTube for the formats"
	case tag == "info" && has("format(s)"):
		return "Picked the best format"
	case tag == "download" && strings.HasPrefix(line, "Downloading playlist: "):
		return "Playlist: " + strings.TrimPrefix(line, "Downloading playlist: ")
	case tag == "download" && strings.HasPrefix(line, "Destination: "):
		return fileStep // which file it is (the picture or the sound) only the job knows: addLog

	case tag == "download" && strings.HasSuffix(line, "has already been downloaded"):
		return "Already in the folder"
	case tag == "download" && has("Sleeping"):
		return "" // shown as "waiting" with the seconds, not as a log line
	case tag == "ExtractAudio" && strings.HasPrefix(line, "Destination: "):
		return "Converting to " + strings.ToUpper(strings.TrimPrefix(filepath.Ext(line), "."))
	case tag == "VideoRemuxer" && strings.HasPrefix(line, "Not remuxing"), strings.HasPrefix(tag, "Fixup"),
		tag == "MoveFiles", tag == "VideoConvertor" && strings.HasPrefix(line, "Not converting"):
		return "" // yt-dlp's housekeeping: "already in the right format", fixing up a container
	case tag == "VideoRemuxer" || tag == "VideoConvertor":
		return "Converting to MP4"
	case tag == "SponsorBlock" || tag == "ModifyChapters":
		return "Cutting out the sponsors"
	case tag == "Merger":
		return "Joining the picture and the sound"
	case tag == "EmbedThumbnail":
		return "Adding the cover"
	case strings.HasPrefix(line, "WARNING: "):
		return "⚠ " + strings.TrimPrefix(line, "WARNING: ")
	}
	return line
}

// fileStep stands for "yt-dlp starts a file" until addLog says which one (no file names on the page).
const fileStep = "\x00file"

// addLog adds a readable line to the short log the page shows.
func (j *Job) addLog(line string) {
	line = logLine(line)
	if line == "" {
		return
	}
	if line == fileStep {
		// A video comes as two files, the picture first: the second since the last join is the sound.
		line = "Downloading the sound"
		if j.Mode == "video" {
			line = "Downloading the picture"
			for i := len(j.Log) - 1; i >= 0; i-- {
				if j.Log[i] == "Joining the picture and the sound" {
					break
				}
				if j.Log[i] == "Downloading the picture" {
					line = "Downloading the sound"
					break
				}
			}
		}
	}
	j.Now = "" // a new step: the live line of the last one is over
	if r := []rune(line); len(r) > 140 {
		line = string(r[:139]) + "…"
	}
	// yt-dlp asks YouTube for the formats twice (before and after its check): a step that was
	// just there isn't listed again. (Further back it's a new round: the next video of a playlist.)
	for i := max(0, len(j.Log)-2); i < len(j.Log); i++ {
		if j.Log[i] == line {
			return
		}
	}
	j.Log = append(j.Log, line)
	if len(j.Log) > logSize {
		j.Log = j.Log[len(j.Log)-logSize:]
	}
}

// addJobs puts links into the queue.
func addJobs(links []string, o Options) {
	mu.Lock()
	for _, link := range links {
		fixed, list, mix := classify(link, o.Playlist)
		j := &Job{ID: nextID, URL: fixed, Options: o, List: list, mix: mix, Status: "queued", Files: []string{}}
		if mix {
			j.Note = fmt.Sprintf("A YouTube Mix never ends, so only the first %d videos.", mixLimit)
		}
		jobs = append(jobs, j)
		nextID++
	}
	mu.Unlock()
	pump()
}

// pump starts the next download from the queue if nothing is downloading right now
// and all the needed programs are in place.
func pump() {
	mu.Lock()
	defer mu.Unlock()
	if running != nil || !tools.ready() {
		return
	}
	var j *Job
	for _, x := range jobs {
		if x.Status == "queued" {
			j = x
			break
		}
	}
	if j == nil {
		return
	}

	os.MkdirAll(outDir, 0o755)
	j.Status = "running"
	cmd := exec.Command(tools.ytdlp, buildArgs(j)...)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1") // non-Latin titles without mojibake
	hideWindow(cmd)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		j.Status, j.Error = "error", err.Error()
		go pump()
		return
	}
	running, current = cmd, j

	// Read both normal output and error output, line by line.
	var wg sync.WaitGroup
	for _, r := range []io.Reader{stdout, stderr} {
		wg.Add(1)
		go func(r io.Reader) {
			defer wg.Done()
			sc := bufio.NewScanner(r)
			sc.Buffer(make([]byte, 64*1024), 1024*1024)
			for sc.Scan() {
				mu.Lock()
				onLine(j, sc.Text())
				mu.Unlock()
			}
		}(r)
	}

	go func() {
		wg.Wait()
		err := cmd.Wait()

		mu.Lock()
		splits := j.splits
		j.splits = nil
		if len(splits) > 0 {
			j.addLog("[tags] Tagging the tracks split by chapters")
		}
		if j.Status == "running" {
			j.Progress = 100 // the page shows "processing"
		}
		mu.Unlock()

		// Covers: the real album cover from Apple Music, or the thumbnail squared if it's
		// art between bars (cover.go). The split tracks get theirs in tagChapters.
		mu.Lock()
		songs := j.songs
		var covers []string
		if j.Mode == "audio" {
			covers = append(covers, j.paths...)
			for _, t := range j.tracks {
				covers = append(covers, t.Filepath)
			}
		}
		mu.Unlock()
		done := map[string]bool{}
		for _, p := range covers {
			if done[p] {
				continue
			}
			done[p] = true
			s := songs[p]
			if fixCover(p, albumArt(s.Artist, s.Title, s.Album)) {
				mu.Lock()
				j.addLog("[cover] The real album cover for " + s.Title)
				mu.Unlock()
			}
		}

		// Tag the split tracks while the download still counts as active.
		for _, s := range splits {
			tagChapters(s)
		}

		// "One file": glue the downloaded pieces together. If some videos of the playlist
		// failed (deleted, private), the rest is still glued, and the error stays as a note.
		mu.Lock()
		tracks := j.tracks
		j.tracks = nil
		doMerge := j.Status == "running" && len(tracks) > 0
		if doMerge {
			j.addLog(fmt.Sprintf("[merge] Gluing %d files into one", len(tracks)))
		}
		mu.Unlock()
		if doMerge {
			out, mergeErr := mergePlaylist(j, tracks)
			mu.Lock()
			switch {
			case mergeErr == errCancelled:
			case mergeErr != nil:
				j.Status, j.Error = "error", mergeErr.Error()
			default:
				j.paths, j.Files = []string{out}, []string{filepath.Base(out)}
				j.Title = strings.TrimSuffix(filepath.Base(out), filepath.Ext(out)) // the playlist name, no ".mp3"
				j.Progress = 100
				if err != nil && j.Error != "" {
					j.Error = skippedText(j.Mode, j.failed) + " " + j.Error
				}
				err = nil
			}
			mu.Unlock()
		}

		// The cover that's really inside the finished file, for the page to show.
		mu.Lock()
		// A playlist where some videos failed (deleted, private) while the rest downloaded:
		// yt-dlp goes on past them and only says so in its exit code. That's done, with a note
		// and Retry, not a failure: the files are there.
		if j.Status == "running" && err != nil && j.List && len(j.paths) > 0 && j.Error != "" {
			j.Error = skippedText(j.Mode, j.failed) + " " + j.Error
			err = nil
		}
		var last string
		if n := len(j.paths); n > 0 && err == nil && j.Status == "running" {
			last = j.paths[n-1]
		}
		mu.Unlock()
		pic := coverOf(last)

		mu.Lock()
		if pic != nil {
			j.cover, j.Cover = pic, j.Cover+1
		}
		if j.Status == "running" { // if the download was cancelled, the status is already "cancelled"
			if err == nil {
				if j.List && j.listName != "" && len(j.paths) > 1 {
					j.Title = j.listName // a finished playlist is the playlist, not its last video
				}
				j.Status = "done"
				j.Already = j.had && !j.got
			} else {
				j.Status = "error"
				if j.Error == "" {
					j.Error = "yt-dlp failed: " + err.Error()
				}
			}
		}
		running, current = nil, nil
		mu.Unlock()
		pump()
	}()
}

// skippedText: "1 video was skipped.", "3 tracks were skipped."
func skippedText(mode string, n int) string {
	w := "video"
	if mode == "audio" {
		w = "track"
	}
	if n == 1 {
		return "1 " + w + " was skipped."
	}
	return strconv.Itoa(n) + " " + w + "s were skipped."
}

// jobCover returns the cover of a finished download, if it has one.
func jobCover(id int) []byte {
	mu.Lock()
	defer mu.Unlock()
	for _, j := range jobs {
		if j.ID == id {
			return j.cover
		}
	}
	return nil
}

// cancelJob removes a queued download, or stops the running one.
// The unfinished file stays and resumes if the link is added again.
func cancelJob(id int) {
	mu.Lock()
	defer mu.Unlock()
	for _, j := range jobs {
		if j.ID == id && (j.Status == "queued" || j.Status == "running") {
			j.Status = "cancelled"
			if current == j && running != nil {
				killTree(running) // yt-dlp together with the ffmpeg it started
			}
		}
	}
}

// retryJob puts a failed or cancelled download (or a playlist with skipped videos) back into the queue. Files that are
// already there are skipped by yt-dlp (--no-overwrites), unfinished ones resume.
func retryJob(id int) {
	mu.Lock()
	for _, j := range jobs {
		// current == j: a cancelled download whose yt-dlp is still being stopped.
		// Done with skipped videos counts too: Retry fetches only what's missing.
		again := j.Status == "error" || j.Status == "cancelled" || (j.Status == "done" && j.Error != "")
		if j.ID == id && again && current != j {
			j.Status, j.Error, j.ErrorRaw, j.Now = "queued", "", "", ""
			j.Progress, j.Speed, j.ETA, j.Size, j.Wait = 0, "", "", "", 0
			j.Thumb, j.cover = "", nil // Cover (the counter) stays: the page asks for a new picture by it
			j.Files, j.paths, j.tracks, j.splits = []string{}, nil, nil, nil
			j.Pos, j.Of, j.part, j.lastB, j.failed = 0, 0, 0, 0, 0
			j.songs = nil
			j.Log = nil
			j.had, j.got, j.Already = false, false, false
		}
	}
	mu.Unlock()
	pump()
}

// cleared: what the last "Clear finished" took off the list, for Undo.
var cleared []*Job

// clearFinished removes everything that is no longer downloading or waiting, and says how
// many went. They're kept aside until the next clear, so Undo can bring them back.
func clearFinished() int {
	mu.Lock()
	defer mu.Unlock()
	var kept, gone []*Job
	for _, j := range jobs {
		if j.Status == "queued" || j.Status == "running" {
			kept = append(kept, j)
		} else {
			gone = append(gone, j)
		}
	}
	jobs, cleared = kept, gone
	return len(gone)
}

// unclear puts back what the last clear took off, each in its old place (the list is in the
// order the links were added, which is the order of the ids).
func unclear() {
	mu.Lock()
	defer mu.Unlock()
	jobs = append(jobs, cleared...)
	cleared = nil
	sort.SliceStable(jobs, func(a, b int) bool { return jobs[a].ID < jobs[b].ID })
}

// jobFile returns the full path of a finished file — only files this program downloaded itself.
func jobFile(id, n int) (string, bool) {
	mu.Lock()
	defer mu.Unlock()
	for _, j := range jobs {
		if j.ID == id && n >= 0 && n < len(j.paths) {
			return j.paths[n], true
		}
	}
	return "", false
}

var chapterNumRe = regexp.MustCompile(`^\s*\d{1,3}\s*[.)\-–—:]\s*`)

// tagChapters tags the tracks split out of one video:
// title = chapter title, album = album (or the video title), track = "3/12".
// Artist, year and cover stay from the original file. The audio is not re-encoded.
func tagChapters(info splitInfo) {
	if info.Filepath == "" || len(info.Chapters) == 0 {
		return
	}
	dir := strings.TrimSuffix(info.Filepath, filepath.Ext(info.Filepath)) // tracks folder = file name without extension
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	album := info.Album
	if album == "" {
		album = info.Title
	}
	total := len(info.Chapters)
	art := albumArt(info.Artist, "", album) // one cover for the whole album
	for i, ch := range info.Chapters {
		prefix := fmt.Sprintf("%02d - ", i+1)
		file := ""
		for _, e := range entries {
			n := strings.ToLower(e.Name())
			if strings.HasPrefix(e.Name(), prefix) && (strings.HasSuffix(n, ".m4a") || strings.HasSuffix(n, ".mp3")) {
				file = e.Name()
				break
			}
		}
		if file == "" {
			continue
		}
		// "01. Song" / "1) Song" in a chapter title — the number is already in the tag.
		title := strings.TrimSpace(chapterNumRe.ReplaceAllString(ch.Title, ""))

		src := filepath.Join(dir, file)
		tmp := filepath.Join(dir, "~"+file) // same extension: ffmpeg picks the format by it
		// Only audio and cover (the cover is a "video" stream). After splitting, M4A also keeps
		// a chapter data stream that ffmpeg can't copy, so it's left out.
		args := []string{"-y", "-hide_banner", "-loglevel", "error", "-i", src, "-map", "0:a", "-map", "0:v?", "-c", "copy",
			"-metadata", fmt.Sprintf("track=%d/%d", i+1, total), "-metadata", "album=" + album}
		if title != "" {
			args = append(args, "-metadata", "title="+title)
		}
		if strings.HasSuffix(strings.ToLower(file), ".mp3") {
			args = append(args, "-id3v2_version", "3") // ID3v2.3 is readable everywhere, Windows included
		}
		args = append(args, tmp)

		cmd := exec.Command(tools.ffmpeg, args...)
		hideWindow(cmd)
		if cmd.Run() == nil {
			os.Rename(tmp, src)
		} else {
			os.Remove(tmp)
		}
		fixCover(src, art)
	}
}
