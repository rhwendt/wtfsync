#!/usr/bin/env bash
# Renders the tray app's pages with sample data (demo mode, see internal/ui/web/demo.js) into PNGs.
# usage: screenshots.sh [out-dir]    (default docs/screenshots)
# Needs python3 and Chrome or Chromium; set CHROME to its path if it isn't on PATH.
set -euo pipefail

out=$(realpath -m "${1:-docs/screenshots}")
chrome=${CHROME:-$(command -v google-chrome || command -v chromium || command -v chromium-browser || true)}
if [ -z "$chrome" ]; then
  echo "screenshots.sh: needs google-chrome or chromium (or CHROME=/path/to/chrome)" >&2
  exit 1
fi
mkdir -p "$out"

port=8765
python3 -m http.server "$port" --bind 127.0.0.1 --directory internal/ui/web >/dev/null 2>&1 &
server=$!
trap 'kill $server' EXIT
for _ in $(seq 50); do
  curl -fs "http://127.0.0.1:$port/main.html" >/dev/null && break
  sleep 0.1
done

shot() { # name, page and query, width, height
  "$chrome" --headless --disable-gpu --no-first-run --hide-scrollbars --force-device-scale-factor=2 \
    --window-size="$3,$4" --virtual-time-budget=3000 \
    --screenshot="$out/$1.png" "http://127.0.0.1:$port/$2" >/dev/null 2>&1
}

shot main "main.html?demo&flavor=classic_beta&theme=light" 1000 640
shot main-dark "main.html?demo&flavor=classic_beta&theme=dark" 1000 640
shot settings "main.html?demo&view=settings&theme=light" 1000 760
shot wizard "main.html?demo&view=wizard&step=4&theme=light" 1000 640
shot conflicts "conflicts.html?demo&theme=light" 760 460
echo "wrote $(ls "$out"/*.png | wc -l) screenshots to $out"
