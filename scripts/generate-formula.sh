#!/usr/bin/env bash
set -euo pipefail

if [ $# -ne 1 ]; then
  echo "Usage: $0 <tag>  (e.g. v0.1.0)" >&2
  exit 1
fi

TAG="$1"
VERSION="${TAG#v}"
BASE_URL="https://github.com/shonenada-vibe/piece-cli/releases/download/${TAG}"

fetch_sha256() {
  local goos="$1"
  local goarch="$2"
  local archive="piece_${TAG}_${goos}_${goarch}.tar.gz"
  local sha
  sha=$(curl -fsSL "${BASE_URL}/checksums.txt" | awk -v archive="$archive" '$2 == archive { print $1 }')
  if [ -z "$sha" ]; then
    echo "Error: failed to fetch checksum for ${archive}" >&2
    exit 1
  fi
  echo "$sha"
}

SHA_DARWIN_ARM64=$(fetch_sha256 "darwin" "arm64")
SHA_DARWIN_AMD64=$(fetch_sha256 "darwin" "amd64")
SHA_LINUX_ARM64=$(fetch_sha256 "linux" "arm64")
SHA_LINUX_AMD64=$(fetch_sha256 "linux" "amd64")

cat <<EOF
class Piece < Formula
  desc "Command-line recorder and uploader for MindNote terminal recordings"
  homepage "https://github.com/shonenada-vibe/piece-cli"
  version "${VERSION}"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "${BASE_URL}/piece_${TAG}_darwin_arm64.tar.gz"
      sha256 "${SHA_DARWIN_ARM64}"
    elsif Hardware::CPU.intel?
      url "${BASE_URL}/piece_${TAG}_darwin_amd64.tar.gz"
      sha256 "${SHA_DARWIN_AMD64}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "${BASE_URL}/piece_${TAG}_linux_arm64.tar.gz"
      sha256 "${SHA_LINUX_ARM64}"
    elsif Hardware::CPU.intel?
      url "${BASE_URL}/piece_${TAG}_linux_amd64.tar.gz"
      sha256 "${SHA_LINUX_AMD64}"
    end
  end

  def install
    bin.install "piece"
  end

  test do
    assert_match "MindNote CLI", shell_output("#{bin}/piece help")
  end
end
EOF
