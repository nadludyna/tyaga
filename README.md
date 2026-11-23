<p align="center">
  <img src="winres/icon.svg" width="112" alt="">
</p>

<h1 align="center">ТЯГА</h1>

<p align="center">
  <b>Paste a YouTube link, get a file.</b><br>
  <a href="../../releases/latest"><img src="https://img.shields.io/github/v/release/nadludyna/tyaga?color=ff6b2c&label=download" alt="Latest release"></a>
  <a href="../../actions/workflows/ci.yml"><img src="https://github.com/nadludyna/tyaga/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <img src="https://img.shields.io/badge/Windows-161616" alt="Windows">
</p>

ТЯГА downloads video and music from YouTube: paste a link, pick video or audio, get a clean file with proper tags and a real cover. One small program for Windows, nothing to install, nothing left behind. The heavy lifting is done by [yt-dlp](https://github.com/yt-dlp/yt-dlp).

<p align="center">
  <img src="docs/screenshot.png" width="680" alt="ТЯГА's window: the links box, one row of options, a video downloading on the fishing line and a song done with its cover">
</p>

## Run it

1. Download `tyaga.exe` from [Releases](../../releases/latest) and put it in any folder.
2. Run it. If Windows says "Windows protected your PC": "More info" → "Run anyway" (the program isn't signed, which is normal for small projects).
3. Paste a link, press Download. Files land in `Downloads\ТЯГА`, in `Видео` and `Аудио` (`Video` and `Audio` in English).

On the first run ТЯГА fetches yt-dlp, ffmpeg and Deno (about 250 MB, once, each checked against its official checksum), unless you already have them installed.

## What it can do

- **Video:** MP4 at the best quality or capped at a height you pick. ТЯГА looks at the link first, so the list holds only the heights the video really has. "For editing" (the clapperboard) gives H.264 + AAC, which opens in DaVinci Resolve and any editor.
- **Music:** the original track (M4A) or MP3, clean tags, and the real cover of the album the track is from (from the Apple Music catalogue), not of some compilation that has the same song. Album files are named artist first.
- **Albums** uploaded as one long video get split into tracks by chapters.
- **Playlists and channels** go into a numbered folder, or "as one file" with a chapter per video. A deleted or private video is skipped, the rest is downloaded, and Retry fetches only what's missing.
- **Fragment:** download only a piece: drag the two ends on a slider or pick chapters. A link with `?t=` starts where the link does.
- **No sponsors:** SponsorBlock cuts the ads and "don't forget to subscribe".
- **The fishing line:** one compact row per download, with its picture and "17 MB of 45 MB · 3.5 MB/s · 0:05 left" (or in words what it's doing: waiting for YouTube, converting). Progress is ТЯГА's fish swimming along a thin line; a playlist gets a new cast for every track, and how far the whole list is stays in words. The same progress shows on ТЯГА's taskbar button.
- **When something fails:** what happened in plain words, with Retry and Copy error right on that line.
- **Nothing twice:** a file that's already in the folder is reported, not downloaded again.
- **Into a chat in one go:** a finished file can be copied with one click and pasted into Telegram (or any chat) with Ctrl+V.
- **Sign in to YouTube** for videos that need it: one click on the badge by the icon, the sign-in comes from the browser you use.

## Good to know

- **Portable.** Everything ТЯГА keeps (the tools, settings, sign-in, log) is in one hidden `tyaga-data` folder next to the exe. Nothing goes to AppData. Delete the folder and ТЯГА is gone.
- **Closing.** × hides it to the tray, downloads go on. Quit is in the tray icon's menu.
- **Light and dark, English and Russian.** It follows Windows until you pick with the moon and globe buttons.
- **Pin on top** keeps the window above the others. Win+D still hides it, as it hides every window.
- **Your cookies by hand** still work: save them as `youtube-cookies.txt` next to the program. They are the keys to your account: never send or commit them.
- Something went wrong? `tyaga-data\tyaga.log` says what happened.

## Build it yourself

You need [Go](https://go.dev/dl/) 1.26 or newer:

```
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags production -trimpath -ldflags "-s -w -H=windowsgui" -o tyaga.exe .
```

Tests: `go test ./...`. The test stand (`stand/README.md`) runs the program against a fake yt-dlp, to see every state without YouTube; its layout checks (`stand/check.js`) measure the page for anything misaligned, cut off or moving. CI runs all of it on every pull request: the Go tests, the page's script, and the layout checks in every state, both themes, both languages, wide and narrow. Releases: Actions → Release → a version like `v2.0.0`.

## The name

**ТЯГА** (say *TYA-ga*; the file is called `tyaga`) is a Russian and Ukrainian word for *pull*, *traction*, from *тянуть*, "to pull"; pulling a file down off the internet is *стянуть*. That's the whole job. The icon says the same: a hook that caught a fat fish with a play button on its side.

## How it started

As `TYAGA.cmd`: a batch file and a TypeScript program in one file, Windows-only and needing Deno installed. Then it grew up and was rewritten in Go, into one program that runs everywhere. Then it got its own window and tray and became a Windows app: that's where it's used.

## License

[MIT](LICENSE). yt-dlp, ffmpeg and Deno are separate projects with their own licenses. ТЯГА doesn't ship them, it downloads them from their official pages.
