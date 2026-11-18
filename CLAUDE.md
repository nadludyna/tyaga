# ТЯГА — notes for AI agents (and humans) working on this repo

ТЯГА (say *TYA-ga*; the file is `tyaga`) downloads video and music from YouTube with yt-dlp.
A **Windows app**: one Go program with its own window and tray icon (Wails, WebView2) showing
one embedded page (`index.html`). On Linux the same program serves the page to a browser; that
is only for the test stand and development, there are no macOS or Linux releases.
Read `README.md` for what it does for people.

## The owner

- Talk to the owner in **Russian**, casually. He is not a developer: explain jargon in a few
  words, show results, don't lecture.
- Everything in the repo (code, comments, README, commits, PRs) is in **English**.
- His PC: Windows, 1920×1080 at **125 % scaling** (work area 1536×824). Check sizes for that.
  Main browser: Brave.
- The name is **ТЯГА** everywhere people see it. Latin `tyaga` only where it's a file name.

## Talking it through

- **When the owner proposes something:** first say back what you understood. Then say
  honestly whether it's a good idea. Good → do it, maybe with a small improvement (say which
  and why). Bad → say plainly that it's a bad idea, why, and what would work better. Never
  agree just because he asked.
- **Push back openly.** What he said earlier is not law: if it no longer fits (a rule, a
  "keep this as is"), say so, propose better and say why. He'd rather argue than guess. Don't
  invent disagreement for its own sake either.
- **He tends to say "go" to a confident answer** ("it's an AI, it probably knows better").
  So don't sound surer than you are, give the reason behind every choice, and say what you
  don't know or didn't check.
- **Check that he understood, don't ask "clear?"**: for anything that matters, ask him to
  explain it back in his own words or on a concrete example ("what happens if you paste a
  playlist link now?"). If his answer is off, explain again, differently. When he really
  understands what we're doing, the work goes much better.
- **After showing something**, say what was checked and how, and what wasn't (e.g. "tested
  on the stand, not with real YouTube").

## How we work

1. **Talk first.** Collect what's wrong or wanted into a list, agree on it, then do it as one
   batch. No PR for every small fix.
2. **Check your own work before showing it** (see "Before you show anything"). The owner
   should not have to point out problems you could have seen yourself.
3. **Give the owner a test build**, not a PR: cross-compile and send the exe
   (`GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags production -trimpath -ldflags "-s -w -H=windowsgui -X main.version=test-NN"`).
   He runs it next to his normal copy, not instead of it. Builds are numbered so he can tell
   which came after which: put each one on his PC as
   `Desktop\клоны репо\ТЯГА builds\NN — DD.MM HH-MM — <what's new, in Russian> (<sha>)\tyaga.exe`
   (his local time, Europe/Warsaw) and build it with `-X main.version=test-NN`, so the page shows the number
   too. Old builds are kept, never deleted. Tell him the folder; no zips in the chat. Each build
   report says what was checked (transitions included) and how, and what wasn't.
   Getting it there: his PC has Go; a Remote Control session can build it from the branch.
4. **One branch = one working session.** A session starts when the owner sits down to work
   on ТЯГА; everything done in it goes to one branch (commit and push as you go, no PR yet).
   When he says the session is over ("заканчиваем сессию"), update the README, open the PR
   for that branch, and he goes off to do other things. The next session starts a new branch
   from `main`. All threads of a session work on that one branch; if a thread had to make its
   own, it is merged into the session's branch and then deleted. Branches are deleted after
   their PR is merged (GitHub does it by itself: "Automatically delete head branches" is on),
   so the repo holds only `main` and the current session's branch.
5. **At the end of a session: the README and `docs/decisions.md`**, not after every change.
   The README's top stays short and holds only what rarely changes: what ТЯГА is, how to
   start it, the screenshot. Details and the things that change often go further down.
   `docs/decisions.md` gets a couple of lines per decision made in the session: what was
   chosen, why, what was rejected.
6. **A session starts with a self-audit**: before he tests anything, go through the latest
   build against `docs/ui-principles.md` and bring him a list of problems.
7. Releases are made by the owner: Actions → Release, with a version like `v1.2.0`. Never
   delete and recreate a release: a fix gets the next version (semver: `v1.2.1` for a fix,
   `v1.3.0` for something new).
8. Claude can't delete branches on GitHub (refused): the owner does it at
   github.com/nadludyna/tyaga/branches.

## Before you show anything

Run the **test stand** (`stand/README.md`): the real program against a fake yt-dlp that prints
exactly what the real one does (a playlist, YouTube's 5-second pauses, thumbnail lines, long
Windows paths, two-file videos, a 403). Then look — really look — at the whole page, not only
at the part you changed, in every state:

- empty list; queued; waiting (YouTube's pause); downloading; converting; done (audio, video,
  playlist as one file); failed with Retry; several downloads at once;
- a wide window and a narrow one (~420 px); dark and light theme.

**Transitions are part of the check, every time.** List every state change your work touches
(something appears, disappears, expands, collapses, a view switches, the list clears, hover,
press, the mouse leaving the window and coming back) and watch each one frame by frame on the
stand (screenshots ~100 ms apart). Anything that pops, vanishes, jumps or snaps is a bug: fix
it before the build. See rule 1a in `docs/ui-principles.md`. The owner should not have to
name them one by one.

For each screenshot ask: is there an empty hole? Is something misaligned with its neighbours?
Do colours compete? Is any text a developer's text (paths, ids, `[tags]`)? Does it say what's
really happening right now? Check the wave numbers `stand/record.py` prints (never stops,
never goes back). Fix what you find, run it again, *then* show it.

Also: `gofmt -l .` (CI fails on it, import order included), `go vet ./...` for linux **and**
`GOOS=windows`, `go test ./...` (cover and merge tests need ffmpeg).

## Design rules

- **How the page behaves** (nothing jumps, never freezes, Nielsen's ten): `docs/ui-principles.md`.
  Check every change against it.
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
- **Motion is alive but calm.** The progress wave works like Material 3's
  `LinearWavyProgressIndicator` (what Morphe uses as is): one steady pace (one wavelength a
  second), flat up to 10 % and from 95 %. The bar follows progress with a very soft,
  critically damped spring: progress arrives in big steps every 0.7 s, and stiffer springs or
  "go to the new value" made it lurch (measured in a simulation). The background speeds up
  with the progress, 1× → 3×, like Morphe's patcher screen. Respect "reduce motion".
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
- Long videos come in pieces (HLS/DASH): yt-dlp only *guesses* the size, and the guess starts
  far too low (a 12-minute download showed 90 % at once). Count the pieces
  (`fragment_index / fragment_count`) when there are any; the size guess only as a last resort.
- Covers: the real album art from the Apple Music catalogue (iTunes Search API, no key; only
  when artist and title clearly match), else the thumbnail, squared if it's art between bars.
  YouTube Music has no public API — don't build on its internal one.
- One song is on many releases (album, single, "Greatest Hits", someone's compilation) and
  Apple often lists a compilation first: Orbital's "In Sides" tracks got the cover of "20".
  When YouTube Music gives the album, only that album's cover counts; for a plain video, the
  album where the song first came out (the album's own date, from the lookup API: a song's
  date is the same on every album). `TYAGA_LIVE=1 go test -run Live` checks the real catalogue.
- Mock data hides real problems. A demo page is not a test: use the stand.
- Portable: ТЯГА writes nothing outside its own folder (`tyaga-data`, hidden, next to the exe,
  see data.go) and the downloads folder: nothing in AppData, the registry or anywhere else,
  and it cleans up after itself (WebView2 caches are trimmed). Anything new it keeps goes
  there, with a line on why in data.go. WebView2 by default writes to AppData: always set its
  data folder.
- YouTube sign-in: yt-dlp reads the cookies of the browser he uses (Brave) afresh on every
  download. ТЯГА's own sign-in window is only a fallback for Chrome and Edge, whose cookies
  can't be read on Windows. Never commit cookies.

## Map

```
main.go      server, API
jobs.go      queue, yt-dlp command line, reading its output (progress, log, errors)
cover.go     covers (Apple Music art, squaring thumbnails)
merge.go     a playlist glued into one file
tools.go     finding / downloading yt-dlp, ffmpeg, Deno
window_windows.go  the window, tray icon, tray menu, sign-in window (Windows)
proc_*.go    starting / stopping yt-dlp, opening folders (Windows, and Linux for the stand)
app.go       the log; quitting when the page is closed (browser mode)
data.go      tyaga-data, the one folder next to the exe where ТЯГА keeps everything (portable)
login.go     signing in to YouTube
index.html   the whole page (the game during downloads is in it too)
menu.html    the tray icon's menu (Windows), a little window of its own
stand/       the test stand
```
