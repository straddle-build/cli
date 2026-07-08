#!/bin/sh
# Straddle CLI installer — https://github.com/straddle-build/cli
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/straddle-build/cli/main/install.sh | sh
#
# Installs the latest released `straddle` binary into
# ${STRADDLE_INSTALL_DIR:-$HOME/.local/bin} after verifying its sha256
# against the release's checksums.txt.
set -eu

REPO="straddle-build/cli"
INSTALL_DIR="${STRADDLE_INSTALL_DIR:-$HOME/.local/bin}"

fail() {
    echo "install.sh: $*" >&2
    exit 1
}

command -v curl >/dev/null 2>&1 || fail "curl is required"

os=$(uname -s)
case "$os" in
    Darwin) os=darwin ;;
    Linux) os=linux ;;
    *) fail "unsupported OS: $os — download a binary from https://github.com/$REPO/releases" ;;
esac

arch=$(uname -m)
case "$arch" in
    x86_64 | amd64) arch=amd64 ;;
    arm64 | aarch64) arch=arm64 ;;
    *) fail "unsupported architecture: $arch — download a binary from https://github.com/$REPO/releases" ;;
esac

# Resolve the latest release tag via GitHub's /releases/latest redirect.
latest_url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest")
tag=${latest_url##*/}
if [ -z "$tag" ] || [ "$tag" = "latest" ]; then
    fail "could not resolve the latest release tag"
fi
version=${tag#v}

archive="straddle_${version}_${os}_${arch}.tar.gz"
base_url="https://github.com/$REPO/releases/download/$tag"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading straddle $tag ($os/$arch)..."
curl -fsSL -o "$tmp/$archive" "$base_url/$archive"
curl -fsSL -o "$tmp/checksums.txt" "$base_url/checksums.txt"

(
    cd "$tmp"
    grep " $archive\$" checksums.txt >checksum.line ||
        fail "no checksum for $archive in checksums.txt"
    if command -v shasum >/dev/null 2>&1; then
        shasum -a 256 -c checksum.line >/dev/null
    elif command -v sha256sum >/dev/null 2>&1; then
        sha256sum -c checksum.line >/dev/null
    else
        fail "need shasum or sha256sum to verify the download"
    fi
)

tar -xzf "$tmp/$archive" -C "$tmp" straddle
mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/straddle" "$INSTALL_DIR/straddle"

echo "Installed $("$INSTALL_DIR/straddle" --version) to $INSTALL_DIR/straddle"

case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *)
        echo ""
        echo "note: $INSTALL_DIR is not on your PATH. Add it, e.g.:"
        echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
        ;;
esac
