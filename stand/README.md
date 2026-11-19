# Test stand

ТЯГА against a **fake yt-dlp** that prints exactly what the real one prints (taken from real
runs): a playlist, YouTube's "Sleeping 5 seconds" pauses, thumbnail lines, long Windows-style
paths, videos that come as two files, a jumpy speed, a 403 for links with `FAIL` in them, a private video in
the middle of a playlist whose link has `SKIP` in it. It
makes real little files with covers, so the cover and merge steps run for real.

Use it to look at the page in every state **before** showing a change to anyone: no YouTube,
no network, the same run every time.

```sh
stand/run.sh                               # builds and starts it, prints the page address
python stand/record.py audio-list 55 /tmp/r # records a run: screenshots, video, wave numbers
```

Needs Go, Python 3 with Pillow, ffmpeg in PATH; `record.py` also needs Playwright
(`pip install playwright && playwright install chromium`).
