#!/usr/bin/env bash
# Installs omagrab plus its desktop integration:
#   - binary        -> ~/.local/bin/omagrab
#   - Walker entry  -> ~/.local/share/applications/omagrab.desktop
#   - URL handler   -> ~/.local/share/applications/omagrab-url.desktop  (omagrab: scheme)
# Then prints how to load the Chromium right-click extension.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
bindir="$HOME/.local/bin"
appdir="$HOME/.local/share/applications"

mkdir -p "$bindir" "$appdir"

# 1. binary (build if needed)
if [[ ! -x "$here/omagrab" ]]; then
  echo "==> building omagrab"
  (cd "$here" && go build -o omagrab .)
fi
install -m755 "$here/omagrab" "$bindir/omagrab"
echo "==> installed $bindir/omagrab"

# 2. desktop entries
install -m644 "$here/desktop/omagrab.desktop"     "$appdir/omagrab.desktop"
install -m644 "$here/desktop/omagrab-url.desktop" "$appdir/omagrab-url.desktop"
update-desktop-database "$appdir" 2>/dev/null || true
echo "==> installed desktop entries"

# 3. register the omagrab: URL scheme
xdg-mime default omagrab-url.desktop x-scheme-handler/omagrab
echo "==> registered handler: $(xdg-mime query default x-scheme-handler/omagrab)"

case ":$PATH:" in
  *":$bindir:"*) ;;
  *) echo "!!  $bindir is not on your PATH — add it so 'omagrab' resolves" ;;
esac

cat <<EOF

Done. Native side is ready.

To enable right-click in Chromium:
  1. Open  chrome://extensions
  2. Toggle  Developer mode  (top-right)
  3. Click  Load unpacked  and choose:
       $here/extension
  4. Right-click any link / video page -> "⏬ Download with omagrab"

First click shows Chrome's "Open omagrab?" prompt — tick "Always allow".
EOF
