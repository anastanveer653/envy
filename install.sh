#!/usr/bin/env bash
set -euo pipefail

REPO="user/envy"
BINARY="envy"
INSTALL_DIR="${ENVY_INSTALL_DIR:-/usr/local/bin}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
DIM='\033[2m'
NC='\033[0m'

log()    { echo -e "${CYAN}  →${NC} $1"; }
success(){ echo -e "${GREEN}  ✓${NC} $1"; }
error()  { echo -e "${RED}  ✗${NC} $1" >&2; exit 1; }

# Detect OS and arch
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) error "Unsupported architecture: $ARCH" ;;
esac

case "$OS" in
  linux|darwin) ;;
  *) error "Unsupported OS: $OS (use Windows installer from releases page)" ;;
esac

echo ""
echo -e "  ${CYAN}Installing envy...${NC}"
echo ""

# Get latest version
log "Fetching latest release..."
LATEST=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | cut -d'"' -f4)

if [ -z "$LATEST" ]; then
  error "Could not fetch latest version"
fi

log "Latest version: ${LATEST}"

# Download
URL="https://github.com/${REPO}/releases/download/${LATEST}/${BINARY}-${OS}-${ARCH}"
TMP="$(mktemp)"

log "Downloading from GitHub releases..."
curl -fsSL "$URL" -o "$TMP" || error "Download failed"
chmod +x "$TMP"

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP" "${INSTALL_DIR}/${BINARY}"
else
  sudo mv "$TMP" "${INSTALL_DIR}/${BINARY}"
fi

success "Installed to ${INSTALL_DIR}/${BINARY}"
success "envy ${LATEST} is ready!"
echo ""
echo -e "  ${DIM}Run 'envy init' in your project to get started${NC}"
echo ""
