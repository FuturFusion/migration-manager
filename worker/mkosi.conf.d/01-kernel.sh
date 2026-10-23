#!/bin/sh -eu

repo="FuturFusion/kernel-builds"

case "$(uname -m)" in
    x86_64)
        arch="amd64"
        ;;
    aarch64)
        arch="arm64"
        ;;
    *)
        echo "error: unsupported architecture: $(uname -m)" >&2
        exit 1
        ;;
esac
prefix="debian-13-${arch}-virtio"

release_json="$(curl -fsSL "https://api.github.com/repos/${repo}/releases/latest")"

find_asset() {
    printf '%s' "$release_json" |
        grep -oP '"browser_download_url"\s*:\s*"\K[^"]+' |
        grep -E "/${prefix}-linux-${1}-[^/]+\.deb\$" |
        grep -v -- '-dbg' |
        head -n1
}

image_url="$(find_asset image)"
if [ -z "$image_url" ]; then
    echo "error: no linux-image asset for '${prefix}' found in the latest ${repo} release" >&2
    exit 1
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT
curl -fsSL -o "${tmpdir}/linux-image.deb" "$image_url"

if [ "$1" = "final" ]; then
    apt-get install --yes "${tmpdir}/linux-image.deb"
fi