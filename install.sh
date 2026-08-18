#!/bin/sh
set -eu

repository="yanpgwang/mango-terminal"
release="${MANGO_VERSION:-}"

if [ -z "$release" ]; then
  latest_url="$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${repository}/releases/latest")"
  release="${latest_url##*/}"
fi

case "$(uname -s)" in
  Darwin) platform="darwin" ;;
  Linux) platform="linux" ;;
  *)
    echo "mango: unsupported operating system: $(uname -s)" >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  arm64|aarch64) architecture="arm64" ;;
  x86_64|amd64) architecture="amd64" ;;
  *)
    echo "mango: unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

asset="mango_${release}_${platform}_${architecture}.tar.gz"
download_base="https://github.com/${repository}/releases/download/${release}"
temporary_directory="$(mktemp -d)"
trap 'rm -rf "$temporary_directory"' EXIT INT TERM

curl -fsSL "${download_base}/${asset}" -o "${temporary_directory}/${asset}"
curl -fsSL "${download_base}/checksums.txt" -o "${temporary_directory}/checksums.txt"

expected="$(awk -v asset="$asset" '$2 == asset { print $1 }' "${temporary_directory}/checksums.txt")"
if [ -z "$expected" ]; then
  echo "mango: ${asset} is missing from checksums.txt" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "${temporary_directory}/${asset}" | awk '{ print $1 }')"
else
  actual="$(shasum -a 256 "${temporary_directory}/${asset}" | awk '{ print $1 }')"
fi
if [ "$actual" != "$expected" ]; then
  echo "mango: checksum verification failed" >&2
  exit 1
fi

tar -xzf "${temporary_directory}/${asset}" -C "$temporary_directory"

if [ -n "${MANGO_INSTALL_DIR:-}" ]; then
  install_directory="$MANGO_INSTALL_DIR"
elif [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
  install_directory="/usr/local/bin"
else
  install_directory="${HOME}/.local/bin"
fi

mkdir -p "$install_directory"
install -m 0755 "${temporary_directory}/mango" "${install_directory}/mango"

echo "Installed mango ${release} to ${install_directory}/mango"
case ":${PATH}:" in
  *":${install_directory}:"*) echo "Run 'mango' to connect or 'mango --demo' to explore locally." ;;
  *) echo "Add ${install_directory} to PATH before running mango." ;;
esac
