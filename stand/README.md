# Test stand

ТЯГА against a **fake yt-dlp** that prints exactly what the real one prints (taken from real
runs): a playlist, YouTube's "Sleeping 5 seconds" pauses, thumbnail lines, long Windows-style
paths, videos that come as two files, a jumpy speed, a 403 for links with `FAIL` in them, a private video in
the middle of a playlist whose link has `SKIP` in it, a short clip (`TINY` in the link) or a
long video in 400 pieces (`LONG`), and "has already been downloaded" for a file that's already in
the folder (add the same link twice). It
makes real little files with covers, so the cover and merge steps run for real.

Use it to look at the page in every state **before** showing a change to anyone: no YouTube,
no network, the same run every time.

```sh
stand/run.sh                               # builds and starts it, prints the page address
python stand/record.py audio-list 55 /tmp/r # records a run: screenshots, video, wave numbers
```

The layout check: paste `stand/check.js` into the page (devtools console, or a test tool's
"run JavaScript") and run `tyagaCheck()`; an empty list means nothing's off (see
docs/ui-principles.md, "Before every build").

Link checks (`-J`, inspect.go): `CHAP` in the link is an album with 10 chapters, `SPON` a video
with ads (the stand fakes SponsorBlock: `TYAGA_SPONSORBLOCK=fake`), `HD720` goes up to 720p only.

The real window (Windows): `stand/run-window.sh` builds ТЯГА and opens its own window over a
magenta screen; `powershell -ExecutionPolicy Bypass -File stand/record-window.ps1 -links
"https://youtu.be/TINYclip001" -secs 15 -name run` types the link into it, starts it and
records the screen to run.mp4 in the system's temp folder. Close both after: `taskkill /IM
ffplay.exe` and `taskkill /IM tyaga-stand.exe`.

On Windows (Git Bash): `stand/run-windows.sh` does the same with a little `yt-dlp.exe` that runs
the fake with `py` (stand/winshim).

Needs Go, Python 3, ffmpeg in PATH; `record.py` also needs Playwright
(`pip install playwright && playwright install chromium`).
