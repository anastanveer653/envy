#!/bin/sh
# Install the latest envy release.
#
#   curl -fsSL https://raw.githubusercontent.com/anastanveer653/envy/main/install.sh | sh
#
# Environment:
#   ENVY_VERSION      release tag to install (default: latest), e.g. v1.0.0
#   ENVY_INSTALL_DIR  target directory (default: /usr/local/bin, or ~/.local/bin
#                     when /usr/local/bin is not writable and sudo is unavailable)
set -eu

REPO="anastanveer653/envy"
BINARY="envy"

info()  { printf '  \033[36m→\033[0m %s\n' "$1"; }
ok()    { printf '  \033[32m✓\033[0m %s\n' "$1"; }
fail()  { printf '  \033[31m✗\033[0m %s\n' "$1" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"; }
need curl
need tar
need uname

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux|darwin) ;;
  *) fail "unsupported OS: $os (on Windows, download the .zip from https://github.com/$REPO/releases)" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) fail "unsupported architecture: $arch" ;;
esac

tag="${ENVY_VERSION:-}"
if [ -z "$tag" ]; then
  info "Looking up the latest release"
  tag=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)
  [ -n "$tag" ] || fail "could not determine the latest release"
fi
version="${tag#v}"
archive="${BINARY}_${version}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$tag"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

info "Downloading envy $tag ($os/$arch)"
curl -fsSL "$base/$archive" -o "$tmp/$archive" || fail "download failed: $base/$archive"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt" || fail "could not download checksums.txt"

info "Verifying checksum"
expected=$(grep " $archive\$" "$tmp/checksums.txt" | cut -d' ' -f1)
[ -n "$expected" ] || fail "no checksum listed for $archive"
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$archive" | cut -d' ' -f1)
else
  actual=$(shasum -a 256 "$tmp/$archive" | cut -d' ' -f1)
fi
[ "$expected" = "$actual" ] || fail "checksum mismatch for $archive"

tar -xzf "$tmp/$archive" -C "$tmp" "$BINARY"

dir="${ENVY_INSTALL_DIR:-/usr/local/bin}"
if [ -w "$dir" ]; then
  install -m 0755 "$tmp/$BINARY" "$dir/$BINARY"
elif [ -z "${ENVY_INSTALL_DIR:-}" ] && ! command -v sudo >/dev/null 2>&1; then
  dir="$HOME/.local/bin"
  mkdir -p "$dir"
  install -m 0755 "$tmp/$BINARY" "$dir/$BINARY"
else
  info "Installing to $dir (requires sudo)"
  sudo install -m 0755 "$tmp/$BINARY" "$dir/$BINARY"
fi

ok "Installed envy $tag to $dir/$BINARY"
case ":$PATH:" in
  *":$dir:"*) ;;
  *) printf '  Add %s to your PATH to use envy.\n' "$dir" ;;
esac
printf '\n  Get started:  cd your-project && envy init\n\n'
