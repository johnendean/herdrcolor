#!/bin/sh
# Install step on macOS and Linux: fetch this release's binary and check it.
#
# Herdr runs this from [[build]] after cloning the plugin. It downloads the
# release asset whose tag is the manifest's `version`, and installs it only if
# its SHA-256 matches checksums.txt in this commit. A checkout that already has
# a binary (a developer's `go build`) is left alone. HERDRCOLOR_RELEASE_URL
# replaces the download base, for a mirror or for testing. See docs/adr/0001.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
dest="$root/bin/herdrcolor"
repo="johnendean/herdrcolor"

fail() {
	echo "herdrcolor: $*" >&2
	exit 1
}

# Colour what is already running, rather than waiting for the next server start
# to fire the startup hook: a sidebar that stays grey after install looks like
# a broken install. Build commands get no socket, so this relies on `herdr`
# finding the default session by itself, and a failure here never fails the
# install.
first_sync() {
	"$dest" sync || echo "herdrcolor: first sync failed; the startup hook will colour on next start" >&2
}

if [ -x "$dest" ]; then
	echo "herdrcolor: $dest already exists; not downloading"
	first_sync
	exit 0
fi

version=$(sed -n 's/^version *= *"\([^"]*\)".*/\1/p' "$root/herdr-plugin.toml" | head -n 1)
[ -n "$version" ] || fail "no version in herdr-plugin.toml"

case "$(uname -s)" in
	Darwin) os=darwin ;;
	Linux) os=linux ;;
	*) fail "unsupported system: $(uname -s)" ;;
esac
case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*) fail "unsupported architecture: $(uname -m)" ;;
esac

asset="herdrcolor-$os-$arch"
base=${HERDRCOLOR_RELEASE_URL:-https://github.com/$repo/releases/download/v$version}
url="$base/$asset"

want=$(awk -v a="$asset" '$2 == a { print $1 }' "$root/checksums.txt")
[ -n "$want" ] || fail "no checksum for $asset in checksums.txt"

tmp="$dest.download.$$"
trap 'rm -f "$tmp"' EXIT INT TERM

echo "herdrcolor: downloading $url"
if command -v curl >/dev/null 2>&1; then
	curl -fsSL --retry 3 -o "$tmp" "$url" || fail "download failed: $url"
elif command -v wget >/dev/null 2>&1; then
	wget -q -O "$tmp" "$url" || fail "download failed: $url"
else
	fail "neither curl nor wget is installed"
fi

if command -v sha256sum >/dev/null 2>&1; then
	got=$(sha256sum "$tmp" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
	got=$(shasum -a 256 "$tmp" | awk '{ print $1 }')
else
	fail "neither sha256sum nor shasum is installed"
fi
[ "$got" = "$want" ] || fail "checksum mismatch for $asset: expected $want, got $got; not installing"

chmod +x "$tmp"
mv "$tmp" "$dest"
trap - EXIT INT TERM
echo "herdrcolor: installed $asset v$version"
first_sync
