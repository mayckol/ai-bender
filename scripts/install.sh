#!/bin/sh
# bender installer — curl | sh friendly.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/mayckol/ai-bender/main/scripts/install.sh | sh
#   curl -fsSL .../install.sh | BENDER_VERSION=v0.22.0 sh
#   curl -fsSL .../install.sh | BENDER_PREFIX="$HOME/.local" sh
#
# Env:
#   BENDER_VERSION  tag to install (default: latest release)
#   BENDER_PREFIX   install prefix; binary goes to $BENDER_PREFIX/bin (default: /usr/local)
#   BENDER_REPO     override repo slug (default: mayckol/ai-bender)

set -eu

REPO="${BENDER_REPO:-mayckol/ai-bender}"
PREFIX="${BENDER_PREFIX:-/usr/local}"
BIN_DIR="$PREFIX/bin"
VERSION="${BENDER_VERSION:-}"

log()  { printf '==> %s\n' "$*" >&2; }
fail() { printf 'error: %s\n' "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"; }

need uname
need tar
need mkdir
need install

if command -v curl >/dev/null 2>&1; then
  DL='curl -fsSL'
elif command -v wget >/dev/null 2>&1; then
  DL='wget -qO-'
else
  fail "need curl or wget"
fi

if command -v shasum >/dev/null 2>&1; then
  SHA='shasum -a 256'
elif command -v sha256sum >/dev/null 2>&1; then
  SHA='sha256sum'
else
  fail "need shasum or sha256sum"
fi

os_raw="$(uname -s)"
case "$os_raw" in
  Linux)   OS=linux ;;
  Darwin)  OS=darwin ;;
  MINGW*|MSYS*|CYGWIN*) fail "Windows not supported by this installer — use Scoop/manual" ;;
  *) fail "unsupported OS: $os_raw" ;;
esac

arch_raw="$(uname -m)"
case "$arch_raw" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) fail "unsupported arch: $arch_raw" ;;
esac

if [ -z "$VERSION" ]; then
  log "resolving latest release for $REPO"
  VERSION="$($DL "https://api.github.com/repos/$REPO/releases/latest" \
    | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  [ -n "$VERSION" ] || fail "could not resolve latest version"
fi
# normalize: accept v0.22.0 or 0.22.0
case "$VERSION" in v*) VER_NUM="${VERSION#v}" ;; *) VER_NUM="$VERSION"; VERSION="v$VERSION" ;; esac

ASSET="bender_${VERSION}_${OS}-${ARCH}.tar.gz"
BASE="https://github.com/$REPO/releases/download/$VERSION"
TAR_URL="$BASE/$ASSET"
SUMS_URL="$BASE/sha256sums.txt"

TMP="$(mktemp -d 2>/dev/null || mktemp -d -t bender)"
trap 'rm -rf "$TMP"' EXIT INT HUP TERM

log "downloading $ASSET"
$DL "$TAR_URL"  > "$TMP/$ASSET"     || fail "download failed: $TAR_URL"
$DL "$SUMS_URL" > "$TMP/sha256sums" || fail "download failed: $SUMS_URL"

log "verifying sha256"
EXPECTED="$(awk -v f="$ASSET" '$2==f {print $1}' "$TMP/sha256sums")"
[ -n "$EXPECTED" ] || fail "checksum for $ASSET not in sha256sums.txt"
GOT="$($SHA "$TMP/$ASSET" | awk '{print $1}')"
[ "$EXPECTED" = "$GOT" ] || fail "sha256 mismatch: expected $EXPECTED got $GOT"

log "extracting"
tar -xzf "$TMP/$ASSET" -C "$TMP" || fail "extract failed"
[ -f "$TMP/bender" ] || fail "bender binary not found in archive"

log "installing to $BIN_DIR/bender"
if [ -w "$PREFIX" ] || [ -w "$BIN_DIR" ] 2>/dev/null; then
  mkdir -p "$BIN_DIR"
  install -m 0755 "$TMP/bender" "$BIN_DIR/bender"
else
  log "need sudo for $BIN_DIR (set BENDER_PREFIX=\$HOME/.local to avoid sudo)"
  sudo mkdir -p "$BIN_DIR"
  sudo install -m 0755 "$TMP/bender" "$BIN_DIR/bender"
fi

log "installed $("$BIN_DIR/bender" version 2>/dev/null || echo "$VERSION")"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) log "warning: $BIN_DIR not on PATH — add it to your shell rc" ;;
esac
