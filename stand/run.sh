#!/bin/sh
# Builds ТЯГА and starts it against the fake yt-dlp, in a scratch folder. Prints the page address.
#   stand/run.sh            → open the printed address in a browser
set -eu
here=$(cd "$(dirname "$0")" && pwd)
work=${STAND_DIR:-/tmp/tyaga-stand}
pkill -f "tyaga -no-browser -out $work/out" 2>/dev/null || true
sleep 0.5
rm -rf "$work/out" "$work/fakebin"; mkdir -p "$work/app/bin" "$work/out" "$work/fakebin"
cp "$here/fake-yt-dlp" "$work/fakebin/yt-dlp"
(cd "$here/.." && go build -o "$work/app/tyaga" .)
cd "$work/app"
PATH="$work/fakebin:$PATH" nohup ./tyaga -no-browser -out "$work/out" > "$work/app.log" 2>&1 &
for _ in $(seq 40); do
  url=$(grep -o "http://[^ ]*" "$work/app.log" | head -1 || true)
  [ -n "$url" ] && break
  sleep 0.3
done
echo "$url"
