#!/bin/sh
set -e

REPO="diegovrocha/sshtui"
DEST="/usr/local/bin"

OS=$(uname -s | tr A-Z a-z)
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')

URL="https://github.com/${REPO}/releases/latest/download/sshtui_${OS}_${ARCH}.tar.gz"

echo "Installing sshtui..."
echo "  OS:   ${OS}"
echo "  Arch: ${ARCH}"
echo "  From: ${URL}"
echo ""

# Use sudo only when needed (skip when already root, e.g. in Docker)
if [ "$(id -u)" -eq 0 ]; then
    SUDO=""
else
    SUDO="sudo"
fi

curl -sSLf "$URL" | $SUDO tar -xz -C "$DEST" sshtui

echo "✔ sshtui installed to ${DEST}/sshtui"
echo "  Run: sshtui"
