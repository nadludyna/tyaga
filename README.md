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

**ТЯГА** (say *TYA-ga*; the file is called `tyaga`) is a Russian and Ukrainian word for *pull*, *draw*, *traction*, from *тянуть*, "to pull". Pulling a file down off the internet is *стянуть*, so the name is the whole job: ТЯГА pulls video and music off YouTube and onto your disk. The icon says the same: a hook that caught a fat fish with a play button on its side.

One small program, nothing to install, and the interface is just a page in your browser. The heavy lifting is done by [yt-dlp](https://github.com/yt-dlp/yt-dlp); ТЯГА makes it pleasant.

<p align="center">
  <img src="docs/screenshot.png" width="680" alt="ТЯГА: a link box, video/audio switches and a download queue with a progress wave">
</p>

## What it can do

**Video.** MP4 at the best quality, or capped at 1080p / 720p / 480p if you'd rather not download a 4K monster. Tick "for editing" and you get H.264 + AAC instead of YouTube's usual VP9/AV1 + Opus, so it opens in DaVinci Resolve and any other editor. Above 1080p YouTube often has no H.264, so that may come out in 1080p.

**Music.** M4A (the original YouTube track, untouched) or MP3. The tags come out clean: real artist, title without "(Official Video)", year.

**Covers.** The cover is the picture the author chose for the video, not a random frame. Album art sitting between bars becomes a clean square; a normal thumbnail stays whole, nothing cut off. Videos get their thumbnail as the cover too, so Explorer shows it.

**Albums.** A whole album uploaded as one video with chapters? It gets split into proper numbered tracks.

**Playlists and channels.** A playlist goes into its own folder, numbered. A channel link works too, and you get its videos. Or tick "as one file" to glue the whole playlist into one long audio or video, where every video becomes a chapter.

**No sponsors.** SponsorBlock cuts the "this video is sponsored by…" bits and the "don't forget to subscribe". For music it also trims the talking and the movie-scene intros from clips, leaving just the song.

**Just a piece.** Need only the solo from 1:20 to 3:45? Tick "fragment" and set the minutes and seconds: type them or spin them with the arrows (Shift jumps by 10, seconds roll over into minutes). Only that bit is downloaded, cut exactly where you said.

**Try again.** YouTube sometimes throws a random 403. A failed or cancelled download has a Retry button: it goes back into the queue, and whatever already downloaded is skipped.

**From your phone.** Paste a link on the phone and the computer downloads it. Then grab the finished file right onto the phone.

**Watch it go.** The progress is a wave that runs faster the faster the download goes and slows down when it stalls. Under it, a short log of what yt-dlp is doing right now. The progress is in the tab title too, and when a download finishes while you're in another window, you get a notification.

Plus the little things: a queue (paste ten links at once, or drag a link onto the page), a thumbnail next to every download, "Show in folder" for finished ones, a hint on every switch when you hover it, the switches remember how you left them, and endless YouTube Mixes are stopped after the first 25 videos. And it's alive: notes fall in the background and dodge the cursor, and when a download is done the fish on the hook jumps.

Everything lands in `Downloads/YouTube`. If you'd rather it didn't move, turn on "reduce motion" in your system: ТЯГА keeps still then.

## Run it

1. Grab the archive for your system from [Releases](../../releases/latest).
2. Unpack it anywhere.
3. Run `tyaga`. A page opens. That's it.

On the first run ТЯГА fetches what it needs into a `bin` folder next to itself: yt-dlp, ffmpeg and Deno (yt-dlp needs Deno to deal with YouTube). On Windows that's about 250 MB, once. Every download is checked against the checksum its project publishes. yt-dlp updates itself on every start, because YouTube keeps changing things. Already have these tools installed? ТЯГА uses yours.

The folder is portable, so carry it on a USB stick if you like.

**Closing it.** On Windows there's no console window, only the page. Hit **Quit** at the bottom, or just close the tab: ТЯГА finishes whatever is downloading and leaves on its own. Run it again while it's open, and it simply brings the page back. If something goes wrong, `tyaga.log` next to the program says what happened.

**Windows** may say "Windows protected your PC". Click "More info", then "Run anyway". The program isn't signed, which is normal for small projects.

**macOS** may refuse to open it: `xattr -d com.apple.quarantine tyaga`. ffmpeg comes from Homebrew: `brew install ffmpeg`.

**Linux:** `sudo apt install ffmpeg` (or your distro's version of it).

## From your phone

Click "Phone: off" at the bottom of the page. An address with a short code appears, something like `http://192.168.1.5:8765/?t=k7m2pq`. Open it on a phone on the same Wi-Fi.

The code is new every time, nobody on the network gets in without it, and after 20 wrong guesses the door closes by itself. The first time, Windows asks about the firewall: allow private networks. And the computer has to stay on, since it's the one doing the downloading.

## When YouTube wants you to sign in

Some videos are for signed-in users only, and sometimes YouTube decides you're a bot. Export your YouTube cookies in Netscape format (the "Get cookies.txt LOCALLY" extension does it) and save the file as `youtube-cookies.txt` next to the program.

Cookies are the keys to your account. Don't send them to anyone and don't commit them anywhere (`.gitignore` already keeps them out).

## Under the hood

ТЯГА starts a tiny web server that only this computer can reach, with a one-time token in the address, and opens the page. The page hands links to the server, the server runs yt-dlp one link at a time and reports how it's going.

```
main.go      the server and phone access
app.go       life without a console: log, single instance, quitting
jobs.go      the queue and the yt-dlp command
merge.go     gluing a playlist into one file
cover.go     square album covers out of thumbnails with bars
tools.go     finding and downloading yt-dlp, ffmpeg, Deno
proc_*.go    the bits that differ between Windows and Linux/macOS
index.html   the whole page
```

Only the Go standard library, no third-party packages. The icon and the "Properties → Details" info of the Windows exe live in `winres/` and get baked into `rsrc_windows_*.syso`, which `go build` picks up by itself.

## Build it yourself

You need [Go](https://go.dev/dl/) 1.22 or newer:

```
go build -o tyaga .
```

Tests: `go test ./...` (the merge and cover tests need ffmpeg and skip themselves without it). Every pull request runs them, plus a build for every system.

For Windows without the console window: `GOOS=windows GOARCH=amd64 go build -ldflags -H=windowsgui -o tyaga.exe .`

Flags: `-out folder` sets where files go, `-no-browser` skips opening the page (ТЯГА then doesn't quit by itself).

To publish a release with builds for every system: Actions → Release → Run workflow → a version like `v1.2.0`.

## How it started

As `TYAGA.cmd`: a batch file and a TypeScript program in one file, Windows-only and needing Deno installed. Then it grew up and was rewritten in Go, into one program that runs everywhere.

## License

[MIT](LICENSE). yt-dlp, ffmpeg and Deno are separate projects with their own licenses. ТЯГА doesn't ship them, it downloads them from their official pages.
