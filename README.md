<p align="center">
  <img src="winres/icon.svg" width="112" alt="">
</p>

<h1 align="center">ТЯГА</h1>

<p align="center">
  <b>Paste a YouTube link, get a file.</b><br>
  <a href="../../releases/latest"><img src="https://img.shields.io/github/v/release/nadludyna/tyaga?color=ff6b2c&label=download" alt="Latest release"></a>
  <a href="../../actions/workflows/ci.yml"><img src="https://github.com/nadludyna/tyaga/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <img src="https://img.shields.io/badge/Windows%20%C2%B7%20macOS%20%C2%B7%20Linux-161616" alt="Windows, macOS, Linux">
</p>

ТЯГА downloads video and music from YouTube: paste a link, pick video or audio, get a clean file with proper tags and a real cover. One small program, nothing to install. On Windows it has its own window and lives in the tray; elsewhere the interface is a page in your browser. The heavy lifting is done by [yt-dlp](https://github.com/yt-dlp/yt-dlp).

<p align="center">
  <img src="docs/screenshot.png" width="680" alt="ТЯГА: a link box, video/audio switches and a download with a progress wave and a live step list">
</p>

## Run it

1. Grab the archive for your system from [Releases](../../releases/latest).
2. Unpack it anywhere.
3. Run `tyaga`. On Windows its window opens in the middle of the screen; elsewhere a page opens in your browser. That's it.

## What it can do

**Video.** MP4 at the best quality, or capped at 1440p / 1080p / 720p. Tick "for editing" and you get H.264 + AAC instead of YouTube's usual VP9/AV1 + Opus, so it opens in DaVinci Resolve and any other editor (above 1080p YouTube often has no H.264, so that may come out in 1080p).

**Music.** M4A (the original YouTube track, untouched) or MP3. Clean tags: real artist, title without "(Official Video)", year.

**Real covers.** For music ТЯГА looks up the album cover in the Apple Music catalogue by artist and title (1200×1200) and uses it only when both clearly match. Only the artist and the song name are sent. Otherwise the cover is the picture the author chose for the video: album art between bars becomes a clean square, a normal thumbnail stays whole. Videos get their thumbnail as the cover too, so Explorer shows it.

**Albums.** A whole album uploaded as one video with chapters is split into numbered tracks.

**Playlists and channels.** A playlist goes into its own numbered folder; a channel link works too. Or tick "as one file" to glue the playlist into one long audio or video, with a chapter per video.

**No sponsors.** SponsorBlock cuts "this video is sponsored by…" and "don't forget to subscribe". For music it also trims the talking and movie-scene intros from clips.

**Just a piece.** Tick "fragment" and set the minutes and seconds (arrows work, Shift jumps by 10). Only that bit is downloaded, cut exactly there.

**See what's going on.** A Material-style wavy progress bar for the whole download (a playlist counts as one), with the numbers in words: "video 2 of 12", "40% of all", "this video 15%", the speed and time left. Under it, the steps as a small live timeline: what's done, and what's happening right now ("Downloading the picture · part 1 359 of 2 041 · 1.9 GB so far"), including YouTube's short pauses. The channel is shown under the title. The progress is in the tab title too, and you get a notification when a download finishes while you're elsewhere.

**When it fails.** Plain words for what happened, a Retry button (whatever already downloaded is skipped) and Copy error, which copies the technical details to send to someone.

**When it's done.** The row shows the cover that's really inside the file and what came out (Audio · MP3, Video · MP4 · 1080p), with "Show in folder".

**Something to do while you wait.** While a download runs, "Play while you wait" opens a little bricks-and-ball game in place of the list. A finished download doesn't interrupt it: the line above says so, and the game waits where you left it (Esc closes it).

Plus: a queue (paste ten links at once, or drag a link onto the page), a hint on every switch, the switches remember how you left them, endless YouTube Mixes stop after 25 videos, and the page is light enough for a weak laptop. Notes fall in the background and speed up as a download goes; the fish on the hook jumps when one is done. Turn on "reduce motion" in your system and ТЯГА keeps still.

Everything lands in `Downloads/YouTube`.

## Good to know

On the first run ТЯГА fetches what it needs into its own folder (see below): yt-dlp, ffmpeg and Deno (yt-dlp needs Deno to deal with YouTube). On Windows that's about 250 MB, once. Every download is checked against the checksum its project publishes. yt-dlp updates itself on every start, because YouTube keeps changing things. Already have these tools installed? ТЯГА uses yours.

**Portable, leaves nothing behind.** Everything ТЯГА keeps is in one hidden `tyaga-data` folder next to the program: the tools above, the window's settings, the sign-in, the log. Nothing goes to AppData or anywhere else, except your downloads. Delete the folder and ТЯГА is gone; carry it on a USB stick if you like.

**Closing it.** On Windows × hides the window to the tray (a notification says so) and downloads go on; click the tray icon to bring it back, **Quit** is in the tray icon's menu. In the browser, just close the tab: ТЯГА finishes whatever is downloading and leaves on its own. Run it again while it's open, and it simply brings the window back. If something goes wrong, `tyaga-data/tyaga.log` says what happened.

**Windows** may say "Windows protected your PC". Click "More info", then "Run anyway". The program isn't signed, which is normal for small projects.

**macOS** may refuse to open it: `xattr -d com.apple.quarantine tyaga`. ffmpeg comes from Homebrew: `brew install ffmpeg`.

**Linux:** `sudo apt install ffmpeg` (or your distro's version of it).

## When YouTube wants you to sign in

Some videos are for signed-in users only, and sometimes YouTube decides you're a bot. On Windows click **Sign in to YouTube** at the top: ТЯГА takes the sign-in from the browser you use (Brave, Firefox and most others), and if you're not signed in there, opens Google's sign-in page in it (your saved password is right there) and notices by itself when you're in. Chrome and Edge lock their cookies, so for them ТЯГА opens its own small sign-in window instead, which closes by itself once you're in.

Or, anywhere: export your YouTube cookies in Netscape format (the "Get cookies.txt LOCALLY" extension does it) and save the file as `youtube-cookies.txt` next to the program.

Cookies are the keys to your account. Don't send them to anyone and don't commit them anywhere (`.gitignore` already keeps them out).

## Under the hood

On Windows the page lives in ТЯГА's own window ([Wails](https://wails.io) on the WebView2 that comes with Windows), with no network port at all. Elsewhere ТЯГА starts a tiny web server that only this computer can reach, with a one-time token in the address, and opens the page. The page hands links to the server, the server runs yt-dlp one link at a time and reports how it's going.

```
main.go      the server
app.go       life without a console: log, single instance, quitting
data.go      tyaga-data, the one folder where ТЯГА keeps everything
login.go     signing in to YouTube
window_*.go  the Windows window, tray and sign-in window
jobs.go      the queue and the yt-dlp command
merge.go     gluing a playlist into one file
cover.go     covers: the real album art from Apple Music, or the thumbnail squared
tools.go     finding and downloading yt-dlp, ffmpeg, Deno
proc_*.go    the bits that differ between Windows and Linux/macOS
index.html   the whole page (the game too)
menu.html    the tray icon's menu
stand/       a test stand: the program against a fake yt-dlp
```

Besides the Go standard library: Wails v3 for the Windows window and `golang.org/x/sys` for the Windows registry. The icon and the "Properties → Details" info of the Windows exe live in `winres/` and get baked into `rsrc_windows_*.syso`, which `go build` picks up by itself.

## Build it yourself

You need [Go](https://go.dev/dl/) 1.26 or newer:

```
go build -o tyaga .
```

Tests: `go test ./...` (the merge and cover tests need ffmpeg and skip themselves without it). Every pull request runs them, plus a build for every system.

To see the page in every state without YouTube, there's a test stand: the program against a fake yt-dlp that prints what the real one does (`stand/README.md`). Notes for AI agents working on the repo are in `CLAUDE.md`.

For Windows, with its window: `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags production -trimpath -ldflags "-s -w -H=windowsgui" -o tyaga.exe .`

Flags: `-out folder` sets where files go, `-no-browser` skips opening the page (ТЯГА then doesn't quit by itself).

To publish a release with builds for every system: Actions → Release → Run workflow → a version like `v1.2.0`.

## The name

**ТЯГА** (say *TYA-ga*; the file is called `tyaga`) is a Russian and Ukrainian word for *pull*, *traction*, from *тянуть*, "to pull"; pulling a file down off the internet is *стянуть*. That's the whole job. The icon says the same: a hook that caught a fat fish with a play button on its side.

## How it started

As `TYAGA.cmd`: a batch file and a TypeScript program in one file, Windows-only and needing Deno installed. Then it grew up and was rewritten in Go, into one program that runs everywhere.

## License

[MIT](LICENSE). yt-dlp, ffmpeg and Deno are separate projects with their own licenses. ТЯГА doesn't ship them, it downloads them from their official pages.
