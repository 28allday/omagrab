#!/usr/bin/env bash
#
# install.sh — install omagrab, a yt-dlp TUI for Omarchy / Arch.
#
# Quick install (nothing to clone):
#
#   curl -fsSL https://raw.githubusercontent.com/28allday/omagrab/main/install.sh | bash
#
# When run from a git clone it builds from source instead (if Go is present),
# otherwise it downloads the latest released binary for your architecture.
#
#   ./install.sh
#
# Environment overrides:
#   PREFIX=/somewhere         install prefix (default ~/.local → ~/.local/bin)
#   OMAGRAB_VERSION=v0.1.0    pin a release (default: latest)
#
# omagrab needs yt-dlp + ffmpeg at run time (and wl-clipboard for --clip); the
# installer fetches them via pacman on Arch if they're missing. On an Omarchy
# desktop it also adds a floating Walker entry.
set -euo pipefail

REPO="28allday/omagrab"
NAME="omagrab"
PREFIX="${PREFIX:-$HOME/.local}"
BIN_DIR="$PREFIX/bin"
BIN="$BIN_DIR/$NAME"
APP_DIR="$HOME/.local/share/applications"
VERSION="${OMAGRAB_VERSION:-latest}"

bold() { printf '\033[1m%s\033[0m\n' "$1"; }
warn() { printf '\033[33m%s\033[0m\n' "$1" >&2; }
err()  { printf '\033[31m%s\033[0m\n' "$1" >&2; }
log()  { printf '\033[1;35m==>\033[0m %s\n' "$*"; }

# ensure_dep makes sure a command exists, installing its package via pacman on
# Arch if missing. The third arg ("optional") only warns instead of aborting.
ensure_dep() {
  local cmd="$1" pkg="$2" optional="${3:-}"
  command -v "$cmd" >/dev/null 2>&1 && return 0
  if command -v pacman >/dev/null 2>&1; then
    bold "Installing missing dependency: ${pkg}"
    sudo pacman -S --needed --noconfirm "$pkg"
  elif [ "$optional" = "optional" ]; then
    warn "Optional dependency '${cmd}' (${pkg}) not found — install it for full functionality."
  else
    err "Missing dependency '${cmd}'. Install the '${pkg}' package and re-run."
    exit 1
  fi
}

# Run-time dependencies.
ensure_dep yt-dlp  yt-dlp                       # required: the downloader itself
ensure_dep ffmpeg  ffmpeg                       # required: extraction / merge / embed
ensure_dep wl-paste wl-clipboard optional       # optional: needed only for --clip

mkdir -p "$BIN_DIR"

# If the script lives next to the source tree, we're in a clone.
SCRIPT_DIR=""
if [ -n "${BASH_SOURCE[0]:-}" ] && [ -f "${BASH_SOURCE[0]}" ]; then
  SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fi

# Detect OS/arch for prebuilt asset names.
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$(uname -m)" in
  x86_64 | amd64)  arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *)               arch="" ;;
esac

# ---- obtain the binary ----------------------------------------------------
TMPBIN=""
if [ -n "$SCRIPT_DIR" ] && [ -f "$SCRIPT_DIR/go.mod" ] && command -v go >/dev/null 2>&1; then
  bold "Building $NAME from source…"
  TMPBIN="$(mktemp)"
  trap 'rm -f "$TMPBIN"' EXIT
  ( cd "$SCRIPT_DIR" && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$TMPBIN" . )
elif [ -n "$SCRIPT_DIR" ] && [ -n "$arch" ] && [ -x "$SCRIPT_DIR/dist/${NAME}-${os}-${arch}" ]; then
  log "Using prebuilt binary from dist/"
  TMPBIN="$SCRIPT_DIR/dist/${NAME}-${os}-${arch}"
else
  # Download the released binary for this OS/arch (curl-style install).
  [ "$os" = "linux" ] || { err "omagrab ships Linux binaries only (detected: $os). Clone the repo and build with Go."; exit 1; }
  [ -n "$arch" ] || { err "unsupported architecture: $(uname -m)"; exit 1; }
  asset="${NAME}-${os}-${arch}"
  if [ "$VERSION" = "latest" ]; then
    url="https://github.com/$REPO/releases/latest/download/$asset"
  else
    url="https://github.com/$REPO/releases/download/$VERSION/$asset"
  fi
  log "Downloading $asset ($VERSION)…"
  TMPBIN="$(mktemp)"
  trap 'rm -f "$TMPBIN"' EXIT
  if command -v curl >/dev/null 2>&1; then
    curl -fSL --proto '=https' --tlsv1.2 -o "$TMPBIN" "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$TMPBIN" "$url"
  else
    err "need curl or wget to download the binary."; exit 1
  fi
fi

# ---- install --------------------------------------------------------------
install -Dm755 "$TMPBIN" "$BIN"
bold "Installed: $BIN"

# ---- Omarchy desktop integration -----------------------------------------
# Only on Omarchy: add a Walker entry that opens omagrab, and a Hyprland window
# rule so it floats small and centered. Both the Walker entry and the optional
# clipboard keybind use the dedicated "omagrab" app-id, so this one rule covers
# every launch path.
if command -v omarchy-launch-tui >/dev/null 2>&1 || [ -d "$HOME/.local/share/omarchy" ]; then
  mkdir -p "$APP_DIR"

  # Install a bundled icon into the user's hicolor theme — a download arrow over
  # a progress bar, in the terminal-accent magenta.
  icon_dir="$HOME/.local/share/icons/hicolor/scalable/apps"
  mkdir -p "$icon_dir"
  cat > "$icon_dir/omagrab.svg" <<'SVG'
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256" width="256" height="256">
  <rect width="256" height="256" rx="56" fill="#16161e"/>
  <path d="M128 56 L128 150 M92 116 L128 152 L164 116" fill="none" stroke="#bb9af7" stroke-width="16" stroke-linecap="round" stroke-linejoin="round"/>
  <rect x="64" y="190" width="128" height="12" rx="6" fill="#2a2e42"/>
  <rect x="64" y="190" width="84" height="12" rx="6" fill="#bb9af7"/>
</svg>
SVG
  gtk-update-icon-cache -q -t -f "$HOME/.local/share/icons/hicolor" 2>/dev/null || true

  cat > "$APP_DIR/omagrab.desktop" <<EOF
[Desktop Entry]
Name=omagrab
Comment=yt-dlp TUI — paste a URL, pick Audio or Video, download
Exec=xdg-terminal-exec --app-id=omagrab -e $BIN
Icon=omagrab
Terminal=false
Type=Application
Categories=AudioVideo;Network;Utility;
Keywords=yt-dlp;download;video;audio;music;
EOF
  bold "Omarchy detected — added floating Walker entry (with icon)."
  echo "    Launch it from Walker by searching 'omagrab'."

  # Write a small floating window rule, idempotently, between markers we own so
  # re-running the installer never duplicates it. Back the file up first.
  windows_conf="$HOME/.config/hypr/windows.conf"
  begin="# >>> omagrab windowrules begin"
  end="# <<< omagrab windowrules end"
  if [ -f "$windows_conf" ] && grep -qF "$begin" "$windows_conf"; then
    echo "    Hyprland float rule already present — left as-is."
  else
    mkdir -p "$(dirname "$windows_conf")"
    [ -f "$windows_conf" ] && cp "$windows_conf" "$windows_conf.omagrab-bak"
    {
      printf '\n%s\n' "$begin"
      printf '# omagrab — small floating window, centered.\n'
      printf 'windowrule = float on,     match:class ^(omagrab)$\n'
      printf 'windowrule = center on,    match:class ^(omagrab)$\n'
      printf 'windowrule = size 520 260, match:class ^(omagrab)$\n'
      printf '%s\n' "$end"
    } >> "$windows_conf"
    echo "    Added floating window rule to ~/.config/hypr/windows.conf (520x260, centered)."
    command -v hyprctl >/dev/null 2>&1 && hyprctl reload >/dev/null 2>&1 || true
  fi

  cat <<EOF

Optional clipboard keybind: copy a link, hit the key, and omagrab opens with the
URL pre-filled (it already floats — the rule above covers it). Add to
~/.config/hypr/bindings.conf:

    bindd = SUPER SHIFT, V, omagrab, exec, xdg-terminal-exec --app-id=omagrab -e omagrab --clip
EOF
fi

case ":$PATH:" in
  *":$BIN_DIR:"*) : ;;
  *) warn "Note: $BIN_DIR is not on your PATH. Add it to use 'omagrab' directly." ;;
esac
echo "Run 'omagrab' to start, or 'omagrab --help' for options."
