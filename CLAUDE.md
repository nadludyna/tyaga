# ТЯГА — notes for AI agents (and humans) working on this repo

ТЯГА (say *TYA-ga*; the file is `tyaga`) downloads video and music from YouTube with yt-dlp,
through a page in the browser. One Go program: a tiny local server + one embedded page
(`index.html`). Read `README.md` for what it does for people.

## The owner

- Talk to the owner in **Russian**, casually. He is not a developer: explain jargon in a few
  words, show results, don't lecture.
- Everything in the repo (code, comments, README, commits, PRs) is in **English**.
- The name is **ТЯГА** everywhere people see it. Latin `tyaga` only where it's a file name.

## Talking it through

- **When the owner proposes something:** first say back what you understood. Then say
  honestly whether it's a good idea. Good → do it, maybe with a small improvement (say which
  and why). Bad → say plainly that it's a bad idea, why, and what would work better. Never
  agree just because he asked.
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
   too. Old builds are kept, never deleted. Tell him the folder; no zips in the chat.
4. **One branch = one working session.** A session starts when the owner sits down to work
   on ТЯГА; everything done in it goes to one branch (commit and push as you go, no PR yet).
   When he says the session is over ("заканчиваем сессию"), update the README, open the PR
   for that branch, and he goes off to do other things. The next session starts a new branch
   from `main`. All threads of a session work on that one branch; if a thread had to make its
   own, it is merged into the session's branch and then deleted. Branches are deleted after
   their PR is merged (GitHub does it by itself: "Automatically delete head branches" is on),
   so the repo holds only `main` and the current session's branch.
5. **The README is updated at the end of a session**, not after every change. Its top stays
   short and holds only what rarely changes: what ТЯГА is, how to start it, the screenshot.
   Details and the things that change often go further down.
6. Releases are made by the owner: Actions → Release, with a version like `v1.2.0`.

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
