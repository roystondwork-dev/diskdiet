#!/bin/sh
# Installs diskdiet.
#
#   curl -fsSL https://roystondwork-dev.github.io/diskdiet/install.sh | sh
#
# Downloads the latest release for this OS and CPU from GitHub, checks it
# against the release's checksums, and installs it to ~/.local/bin (or
# $DISKDIET_INSTALL_DIR).
#
# Options (environment variables):
#   DISKDIET_INSTALL_DIR  install the binary here instead
#   DISKDIET_VERSION      a version such as 0.6.3 instead of the latest
set -eu

REPO="roystondwork-dev/diskdiet"

say() { printf '%s\n' "$*"; }
fail() { printf 'diskdiet installer: %s\n' "$*" >&2; exit 1; }

fetch() { # url → stdout
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "$1"
  else
    fail "curl or wget is required"
  fi
}

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

os="$(uname -s)"
case "$os" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) fail "unsupported OS $os (on Windows, use install.ps1)" ;;
esac
arch="$(uname -m)"
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "unsupported CPU $arch" ;;
esac

version="${DISKDIET_VERSION:-}"
if [ -n "$version" ]; then
  base="https://github.com/$REPO/releases/download/v${version#v}"
else
  # No API call (rate-limited per IP): the latest release's checksums name its
  # archives, which carry the version.
  base="https://github.com/$REPO/releases/latest/download"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fetch "$base/checksums.txt" >"$tmp/checksums.txt"
archive="$(awk -v s="_${os}_${arch}.tar.gz" '{ n = $2; sub(/^\*/, "", n) } index(n, "diskdiet_") == 1 && substr(n, length(n) - length(s) + 1) == s { print n; exit }' "$tmp/checksums.txt")"
[ -n "$archive" ] || fail "no diskdiet build for ${os}/${arch} in this release"
version="${archive#diskdiet_}"
version="${version%"_${os}_${arch}.tar.gz"}"
say "Downloading diskdiet ${version} for ${os}/${arch}..."
fetch "$base/$archive" >"$tmp/$archive"
want="$(awk -v n="$archive" '$2 == n { print $1 }' "$tmp/checksums.txt")"
got="$(sha256 "$tmp/$archive")"
if [ -z "$want" ] || [ "$want" != "$got" ]; then
  fail "checksum mismatch for $archive"
fi
tar -xzf "$tmp/$archive" -C "$tmp" diskdiet

dir="${DISKDIET_INSTALL_DIR:-$HOME/.local/bin}"
mkdir -p "$dir"
install -m 0755 "$tmp/diskdiet" "$dir/diskdiet" 2>/dev/null || {
  cp "$tmp/diskdiet" "$dir/diskdiet"
  chmod 0755 "$dir/diskdiet"
}
say "Installed $dir/diskdiet"
case ":$PATH:" in
  *":$dir:"*) ;;
  *) say "Note: $dir isn't on your PATH; add it to use 'diskdiet' in a terminal." ;;
esac
"$dir/diskdiet" version
