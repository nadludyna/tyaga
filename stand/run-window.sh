#!/bin/sh
# ТЯГА's own window (the stand build, fake yt-dlp) over a magenta screen, so a blink of the
# see-through window shows. Then stand/record-window.ps1 starts a download and records it.
# Needs the fake yt-dlp built once by run-windows.sh, and ffplay (comes with ffmpeg).
set -eu
here=$(cd "$(dirname "$0")" && pwd)
work=$(cygpath -u "$TEMP")/tyaga-stand
taskkill //F //IM tyaga-stand.exe >/dev/null 2>&1 || true
taskkill //F //IM ffplay.exe >/dev/null 2>&1 || true
sleep 0.5
rm -rf "$work/out"; mkdir -p "$work/out"
(cd "$here/.." && go build -o "$work/app/tyaga-stand.exe" .)
nohup ffplay -v error -f lavfi -i "color=c=0xff00ff:s=1920x1080" -noborder -left 0 -top 0 -x 1920 -y 1080 >/dev/null 2>&1 &
sleep 1.5
cd "$work/app"
TYAGA_SPONSORBLOCK=fake FAKE_YTDLP=$(cygpath -w "$here/fake-yt-dlp") PATH="$work/fakebin:$PATH" \
  nohup ./tyaga-stand.exe -out "$(cygpath -w "$work/out")" > "$work/win.log" 2>&1 &
sleep 4
