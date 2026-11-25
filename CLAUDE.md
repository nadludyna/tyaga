# ТЯГА: notes for AI agents (and people) working on this repo

ТЯГА (say *TYA-ga*; the file is `tyaga`) downloads video and music from YouTube with yt-dlp.
It is a **Windows app**: one Go program with its own window and tray icon (Wails v3, WebView2)
showing one embedded page (`index.html`); the tray icon's menu is a little window of its own
(`menu.html`). On Linux the same program serves the page to a browser: that is only for the
test stand and CI, there are no macOS or Linux releases.

`README.md` says what it does for people, how to run it from the source and where things are.
`docs/ui-principles.md` is how the window must behave; `docs/decisions.md` is why things are the
way they are, and what was tried and rejected. Read both before changing the page.

## Ground rules

- Everything in the repo (code, comments, docs, commits, PRs) is in **English**.
- The name is **ТЯГА** wherever people read it (the page, logs, messages, docs). Latin `tyaga`
  only where it's a file or module name; the User-Agent header and environment variables stay
  ASCII.
- **Portable.** ТЯГА writes nothing outside its own hidden folder `tyaga-data` (next to the exe,
  or the project folder under `go run .`) and the downloads folder: no AppData, no registry, no
  `%USERPROFILE%\.cache`. Anything new it keeps goes into `tyaga-data`, with a line on why in the
  comment at the top of `data.go`. WebView2 and the tools ТЯГА runs default to AppData and the
  user's profile: always give them a folder (see `toolEnv`, `cacheArgs` in `tools.go`).
- **Talks only to what it needs** (listed under "Privacy and security" in the README). A new
  network call means a new line there, and a reason.
- **Never commit cookies** (`youtube-cookies.txt`, `*cookies*.txt`, `tyaga-data/`): they are the
  keys to someone's YouTube account. `.gitignore` covers them; don't work around it.
- The page shows titles and errors as **text only** (never `innerHTML` with anything from yt-dlp
  or YouTube); options from the page and from `queue.json` are checked against known values
  (`cleanOptions`); a double-click opens only sound and video files (`isMedia`).

## How to work on it

1. **Start from what's there.** Read the code and the docs around the change. A feature that
   was tried and rejected is in `docs/decisions.md`: don't bring it back without a new reason.
2. **Check your own work before showing it.** Nobody should have to point out a problem you
   could have seen yourself. The checks are below; CI runs most of them on every PR.
3. **One PR per piece of work**, its description saying what changed, why, how it was checked
   and what wasn't. A decision that changes how ТЯГА behaves gets a couple of lines in
   `docs/decisions.md` (what was chosen, why, what was rejected); the README is updated when
   what people see or do changes.
4. **Releases** are made from GitHub: Actions → Release, with a version like `v1.2.0`. Never
   delete and recreate a release: a fix gets the next version (`v1.2.1`; something new `v1.3.0`).

## The checks

From the repo's root (Windows: Git Bash; the stand also runs on Linux):

```sh
gofmt -l .                                   # must print nothing (CI fails on it, import order too)
go vet ./... && GOOS=windows go vet ./...    # both: half the code is Windows-only
go test ./...                                # the merge and cover tests need ffmpeg in PATH
TYAGA_LIVE=1 go test -run Live -v            # also asks the real YouTube, GitHub and Apple
node stand/syntax.js                         # the page's script compiles
```

**The test stand** (`stand/README.md`): the real program against a fake yt-dlp that prints what
the real one does (a playlist, YouTube's pauses, two-file videos, a 403, long Windows paths), so
every state can be seen without YouTube. `stand/run.sh` (Linux) or `stand/run-windows.sh` prints
the page's address; then, with Playwright (`npm i playwright`, or `CHROME=<path to a Chromium
browser>`):

- `node stand/ci.js <address>`: the layout checks (`stand/check.js`) in every state, both themes,
  both languages, wide and narrow. Must come back clean.
- `node stand/scenarios.js <address>`: the page used the way people use it (a chat message
  pasted, the same video three ways, ten links, cancel, a failure, a playlist, Clear while
  downloading, the fishing line frame by frame). A case found by hand that it misses goes into it.
- `node stand/snapshot.js <address> <dir>` before a change and
  `node stand/snapshot.js <address> <dir2> --compare <dir>` after it: 172 pictures compared
  pixel by pixel. Use it for any clean-up or refactor that must not change how anything looks;
  every differing pixel is either intended (say which) or a mistake.

**Look, don't only measure.** Go through the whole page, not only the part you changed, in every
state: empty; a link pasted; several links; queued; waiting (YouTube's pause); downloading;
converting; done (audio, video, a playlist as one file); failed with Retry; cancelled; "not
finished" after a restart; the drop-downs and the folder menu open; the list folded. Wide (653 px)
and narrow (~440 px), dark and light, English and Russian. Windows at 125 % scaling (a 1536×824
work area) is a common laptop: check sizes for it. Is there an empty hole? Something misaligned
with its neighbours? Text a developer would write (paths, ids, `[youtube]` tags)? Does it say
what's really happening right now?

**Transitions are part of the check.** List every state change the work touches (something
appears, disappears, expands, collapses, a view switches, the list clears, hover, press, the
mouse leaving and coming back) and watch each one frame by frame on the stand (screenshots
~100 ms apart). Anything that pops, vanishes, jumps or snaps is a bug (rule 1a in
`docs/ui-principles.md`).

**The real window** can show what a browser can't (the window is see-through): blinks, the
window's size changing. `go run .` on Windows, or `stand/run-window.sh` with
`stand/record-window.ps1` to record it over a magenta screen.

## Design rules

- **How the page behaves** (nothing jumps, the window's height changes only on listed events,
  never freezes, Nielsen's ten): `docs/ui-principles.md`. Check every change against it.
- **Material 3 Expressive** is the reference. Dark first; light must work as well.
- **A downloader, not a show.** The window answers four questions and nothing else: where to
  paste, what I'll get, is it coming and how long, where is it. One compact two-line row per
  download, no log of steps, no decorations that compete with that.
- **No empty holes.** When something is added or removed, rebalance its neighbours so the whole
  thing still fits together.
- **Nothing moves when you click.** A switch must not change the size or place of any control; a
  button holds its label in both languages (the other one invisible), so switching the language
  moves nothing.
- **An app, not a document.** Buttons and labels aren't selected by Ctrl+A or a stray click;
  only what's worth copying is (the links box, titles, errors).
- **Orange is for the main thing**: solid only for Download and the progress; selected switches
  are tonal (a light tint and orange text). No orange frames or glows around everything.
- **Plain words.** No paths, ids, tags or stack traces on the page. Errors say what happened and
  what to do; the technical text only behind "Copy error". Every string the page shows has its
  Russian in the `RU` table (or `RU_RE` for messages with numbers in them), and the layout check
  reports anything left in English.
- **Nothing moves by itself while nothing happens.** The window is transparent: anything that
  moves makes Windows redraw what's under it, and idle animations cost 5–17 % of a CPU. Motion
  only while something downloads, and as the answer to a click. Respect "reduce motion".
- **Light on weak laptops.** No `backdrop-filter` over moving things; check with the CPU slowed
  6× (Chromium devtools). The list is updated in place, never rebuilt (rebuilding restarts the
  animations and makes pictures blink).
- **The fishing line** (a download's progress) follows the numbers with a soft spring: progress
  arrives in big steps every 0.7 s, and a stiff spring made it lurch. It never goes back. While
  downloading the row shows the video's thumbnail; when done, the cover that's really in the file.

## Things that bit us before

- yt-dlp's `--print` turns on quiet mode: without `--no-quiet` there's no progress to read.
- YouTube makes yt-dlp sleep a few seconds per video ("Sleeping 5.00 seconds"): show it as
  waiting, not as a stuck download.
- Progress: a playlist's line is the current track; a video is two files (picture ~90 %, sound
  ~10 %). Long videos come in pieces (HLS/DASH) and yt-dlp only *guesses* their size, far too low
  at first: count the pieces (`fragment_index / fragment_count`) when there are any.
- Covers come from Apple's iTunes Search API (no key), only when artist and title clearly match;
  else the thumbnail, squared if it's art between bars. One song is on many releases and Apple
  often lists a compilation first: when YouTube Music names the album only that album counts,
  otherwise the album where the song first came out. YouTube Music has no public API: don't
  build on its internal one.
- The YouTube sign-in is read afresh from the browser the person uses, on every download. ТЯГА's
  own sign-in window is only for Chrome and Edge, whose cookies can't be read on Windows.
- `exeName("yt-dlp")` is plain `yt-dlp` on Linux, the same as the unpacked yt-dlp's folder: tests
  that run on CI (Linux) must not assume `.exe`.
- Mock data hides real problems: a demo page is not a test, use the stand. Its fake yt-dlp's
  output is copied from the real one; when the real one changes, change the fake too.
