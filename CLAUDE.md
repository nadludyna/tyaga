# ТЯГА — notes for AI agents (and humans) working on this repo

ТЯГА (say *TYA-ga*; the file is `tyaga`) downloads video and music from YouTube with yt-dlp,
through a page in the browser. One Go program: a tiny local server + one embedded page
(`index.html`). Read `README.md` for what it does for people.

## The owner

- Talk to the owner in **Russian**, casually. He is not a developer: explain jargon in a few
  words, show results, don't lecture.
- Everything in the repo (code, comments, README, commits, PRs) is in **English**.
- The name is **ТЯГА** everywhere people see it. Latin `tyaga` only where it's a file name.

## How we work

1. **Talk first.** Collect what's wrong or wanted into a list, agree on it, then do it as one
   batch. No PR for every small fix.
2. **Check your own work before showing it** (see "Before you show anything"). The owner
   should not have to point out problems you could have seen yourself.
3. **Give the owner a test build**, not a PR: cross-compile and send the exe
   (`GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -H=windowsgui -X main.version=test-<sha>"`),
   zipped as `tyaga/tyaga.exe`. He runs it next to his normal copy, not instead of it.
4. **PR only when he says so.** Work on a branch; open the PR when the batch is done and tested.
5. Releases are made by the owner: Actions → Release, with a version like `v1.2.0`.

## Before you show anything

Run the **test stand** (`stand/README.md`): the real program against a fake yt-dlp that prints
exactly what the real one does (a playlist, YouTube's 5-second pauses, thumbnail lines, long
Windows paths, two-file videos, a 403). Then look — really look — at the whole page, not only
at the part you changed, in every state:

- empty list; queued; waiting (YouTube's pause); downloading; converting; done (audio, video,
  playlist as one file); failed with Retry; several downloads at once;
- a wide window and a narrow one (~420 px); dark and light theme.

For each screenshot ask: is there an empty hole? Is something misaligned with its neighbours?
Do colours compete? Is any text a developer's text (paths, ids, `[tags]`)? Does it say what's
really happening right now? Check the wave numbers `stand/record.py` prints (never stops,
never goes back). Fix what you find, run it again, *then* show it.

Also: `gofmt -l .` (CI fails on it, import order included), `go vet ./...` for linux **and**
`GOOS=windows`, `go test ./...` (cover and merge tests need ffmpeg).

## Design rules

- Reference: **Material 3 Expressive** (the owner likes how the Morphe app looks). Dark first,
  light must work too.
- **No empty holes.** When something is added or removed, rebalance its neighbours so the
  whole thing still fits together. Rows keep their text in one column.
- **Nothing moves when you click.** Flipping a switch must not change the size or place of
  any control (fixed columns, not widths that depend on how many options are next to it).
- **An app, not a document.** Buttons and labels don't get selected by Ctrl+A or a stray
  click; only what's worth copying is selectable (the links box, titles, errors, the log).
- **Orange is for the main thing only**: the Download button, the selected switches, the
  progress wave. Not frames, not glows around every item.
- **Plain words for people.** No file paths, video ids, `[youtube]`-style tags or stack traces
  on the page. Errors say what happened and what to do.
- **Motion is alive but calm.** The progress wave always moves at a steady pace and smoothly
  speeds up while the bar moves forward; it never freezes or restarts. Respect "reduce motion".
- **Light on weak laptops.** No `backdrop-filter` over the animated background; check with the
  CPU slowed down (Chrome devtools, 6×). The download list is updated in place, never rebuilt
  (rebuilding restarts animations and makes pictures blink).
- While downloading, no video thumbnail (it's not what ends up in the file). When done, the
  row shows the cover that's really inside the file.

## Things that bit us before

- yt-dlp's `--print` turns on quiet mode: without `--no-quiet` the page log is empty.
- YouTube makes yt-dlp sleep a few seconds per video ("Sleeping 5.00 seconds"): show it as
  waiting, not as a stuck download.
- Progress: a playlist counts as a whole; a video is two files (picture 90 %, sound 10 %).
- Covers: the real album art from the Apple Music catalogue (iTunes Search API, no key; only
  when artist and title clearly match), else the thumbnail, squared if it's art between bars.
  YouTube Music has no public API — don't build on its internal one.
- Mock data hides real problems. A demo page is not a test: use the stand.

## Map

```
main.go      server, API
jobs.go      queue, yt-dlp command line, reading its output (progress, log, errors)
cover.go     covers (Apple Music art, squaring thumbnails)
merge.go     a playlist glued into one file
tools.go     finding / downloading yt-dlp, ffmpeg, Deno
app.go       quitting when the page is closed
index.html   the whole page
stand/       the test stand
```
