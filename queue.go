package main

// The queue outlives ТЯГА: what was still waiting or downloading when ТЯГА closed (Quit with
// something unfinished, the PC shutting down, a crash) is back in the list at the next start,
// as "not finished" with Retry, which resumes the unfinished file. It's kept in
// tyaga-data/queue.json (see data.go), written every couple of seconds while it changes and
// removed when nothing is left unfinished. Finished rows don't come back: the list starts clean.

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type savedJob struct {
	URL string `json:"url"`
	Options
	List    bool   `json:"list"`
	Mix     bool   `json:"mix,omitempty"`
	Note    string `json:"note,omitempty"`
	Title   string `json:"title,omitempty"`
	Channel string `json:"channel,omitempty"`
	Thumb   string `json:"thumb,omitempty"`
}

// quitting: ТЯГА is on its way out. The queue is written one last time by quit() itself, and a
// download it stops keeps its unfinished file (to resume), unlike one cancelled by hand.
var quitting bool

func queueFile() string { return filepath.Join(dataDir, "queue.json") }

// unfinished lists what has to come back next time. Call with mu held.
func unfinished() []savedJob {
	var s []savedJob
	for _, j := range jobs {
		if j.Status != "queued" && j.Status != "running" && !(j.Status == "cancelled" && j.Stopped) {
			continue
		}
		title := strings.TrimSpace(trackRe.ReplaceAllString(j.Title, ""))
		if j.List {
			title = j.listName // a playlist is its own name, not the video it was on
		}
		s = append(s, savedJob{URL: j.URL, Options: j.Options, List: j.List, Mix: j.mix, Note: j.Note,
			Title: title, Channel: j.Channel, Thumb: j.Thumb})
	}
	return s
}

// writeQueue writes the list if it changed since last (an empty one: the file goes).
func writeQueue(s []savedJob, last *string) {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil || string(b) == *last {
		return
	}
	*last = string(b)
	if len(s) == 0 {
		os.Remove(queueFile())
		return
	}
	os.WriteFile(queueFile(), b, 0o644)
}

// keepQueue writes the queue down every couple of seconds while ТЯГА runs.
func keepQueue() {
	last := ""
	if b, err := os.ReadFile(queueFile()); err == nil {
		last = string(b)
	}
	for range time.Tick(2 * time.Second) {
		mu.Lock()
		if quitting {
			mu.Unlock()
			return
		}
		s := unfinished()
		mu.Unlock()
		writeQueue(s, &last)
	}
}

// loadQueue puts what was left unfinished last time back into the list, stopped: Retry
// resumes each one. Nothing starts by itself: ТЯГА was closed on purpose, maybe.
func loadQueue() {
	b, err := os.ReadFile(queueFile())
	if err != nil {
		return
	}
	var s []savedJob
	if json.Unmarshal(b, &s) != nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	for _, x := range s {
		u, err := url.Parse(x.URL)
		if x.URL == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			continue // only what Download itself would take (a hand-edited file can't add more)
		}
		x.Options = cleanOptions(x.Options)
		jobs = append(jobs, &Job{ID: nextID, URL: x.URL, Options: x.Options, List: x.List, mix: x.Mix, Note: x.Note,
			Title: x.Title, Channel: x.Channel, Thumb: x.Thumb, Status: "cancelled", Stopped: true, Files: []string{}})
		nextID++
	}
	if len(s) > 0 {
		logf("%d unfinished download(s) from last time are back in the list", len(s))
	}
}

// noteTemp remembers the files yt-dlp writes on its way to the finished one (the download
// itself, its .part, the picture, the file being converted or joined), so that a download
// cancelled by hand leaves nothing behind (dropLeftovers). Call with mu held.
func noteTemp(j *Job, line string) {
	var p string
	switch {
	case strings.HasPrefix(line, "[download] Destination: "):
		p = strings.TrimPrefix(line, "[download] Destination: ")
	case strings.HasPrefix(line, "[ExtractAudio] Destination: "):
		p = strings.TrimPrefix(line, "[ExtractAudio] Destination: ")
	case strings.HasPrefix(line, "[Merger] Merging formats into \""):
		p = strings.TrimSuffix(strings.TrimPrefix(line, "[Merger] Merging formats into \""), "\"")
	case strings.HasPrefix(line, "[info] Writing video thumbnail ") && strings.Contains(line, " to: "):
		p = line[strings.LastIndex(line, " to: ")+5:]
	default:
		return
	}
	if p = strings.TrimSpace(p); p != "" {
		j.temps = append(j.temps, p)
	}
}

// leftovers: the files a cancelled download left that aren't finished files: its unfinished
// pieces, pictures and half-converted files. Only inside its own folder. Call with mu held.
func leftovers(j *Job) []string {
	if j.dir == "" {
		return nil
	}
	final := map[string]bool{}
	for _, p := range j.paths {
		final[filepath.Clean(p)] = true
	}
	var out []string
	for _, t := range j.temps {
		p := t
		if !filepath.IsAbs(p) {
			p = filepath.Join(j.dir, p)
		}
		p = filepath.Clean(p)
		if rel, err := filepath.Rel(j.dir, p); err != nil || rel == "." || strings.HasPrefix(rel, "..") || final[p] {
			continue
		}
		frags, _ := filepath.Glob(globEscape(p) + ".part-Frag*")
		for _, f := range append([]string{p, p + ".part", p + ".ytdl"}, frags...) {
			if fileExists(f) {
				out = append(out, f)
			}
		}
	}
	return out
}

// globEscape keeps [ ] * ? in a file name from being read as a pattern.
func globEscape(s string) string {
	r := strings.NewReplacer("[", "[[]", "*", "[*]", "?", "[?]")
	return r.Replace(s)
}
