#!/bin/sh
# The test stand on Windows (Git Bash): like run.sh, with a yt-dlp.exe that runs the fake
# script with Python (`py`). Needs Go, Python and ffmpeg in PATH. Prints the page address.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
work=${STAND_DIR:-$(cygpath -u "$TEMP")/tyaga-stand}
taskkill //F //IM tyaga-stand.exe >/dev/null 2>&1 || true
sleep 0.5
rm -rf "$work/out" "$work/fakebin" "$work/app.log"; mkdir -p "$work/app" "$work/out" "$work/fakebin"
(cd "$here/winshim" && go build -o "$work/fakebin/yt-dlp.exe" main.go)
(cd "$here/.." && go build -o "$work/app/tyaga-stand.exe" .)
cd "$work/app"
TYAGA_SPONSORBLOCK=fake FAKE_YTDLP=$(cygpath -w "$here/fake-yt-dlp") PATH="$work/fakebin:$PATH" \
  nohup ./tyaga-stand.exe -no-browser -out "$(cygpath -w "$work/out")" > "$work/app.log" 2>&1 &
for _ in $(seq 60); do
  url=$(grep -o "http://[^ ]*" "$work/app.log" 2>/dev/null | head -1 || true)
  [ -n "$url" ] && break
  sleep 0.3
done
echo "$url"
