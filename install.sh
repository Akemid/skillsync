#!/bin/sh
set -e

REPO="Akemid/skillsync"
BINARY="skillsync"

# Detect OS
OS="$(uname -s)"
case "${OS}" in
  Linux*)   OS="linux" ;;
  Darwin*)  OS="darwin" ;;
  *)
    echo "Error: unsupported operating system: ${OS}" >&2
    exit 1
    ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64)         ARCH="amd64" ;;
  arm64|aarch64)  ARCH="arm64" ;;
  *)
    echo "Error: unsupported architecture: ${ARCH}" >&2
    exit 1
    ;;
esac

# Detect download tool
if command -v curl > /dev/null 2>&1; then
  DOWNLOAD="curl -fsSL"
elif command -v wget > /dev/null 2>&1; then
  DOWNLOAD="wget -qO-"
else
  echo "Error: curl or wget is required" >&2
  exit 1
fi

# Get latest version from GitHub API
VERSION=$(${DOWNLOAD} "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' \
  | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')

if [ -z "${VERSION}" ]; then
  echo "Error: could not determine latest version" >&2
  exit 1
fi

VERSION_NO_V="${VERSION#v}"

# Build download URL
ARCHIVE="${BINARY}_${VERSION_NO_V}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
ARCHIVE_URL="${BASE_URL}/${ARCHIVE}"
CHECKSUMS_URL="${BASE_URL}/checksums.txt"

# Download to temp dir
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

echo "Downloading ${BINARY} ${VERSION} (${OS}/${ARCH})..."
${DOWNLOAD} "${ARCHIVE_URL}" > "${TMP_DIR}/${ARCHIVE}"
${DOWNLOAD} "${CHECKSUMS_URL}" > "${TMP_DIR}/checksums.txt"

# Verify checksum
cd "${TMP_DIR}"
if command -v sha256sum > /dev/null 2>&1; then
  grep "${ARCHIVE}" checksums.txt | sha256sum --check --status
elif command -v shasum > /dev/null 2>&1; then
  grep "${ARCHIVE}" checksums.txt | shasum -a 256 --check --status
else
  echo "Warning: sha256sum/shasum not found, skipping checksum verification" >&2
fi

# Extract binary
tar -xzf "${ARCHIVE}" "${BINARY}"

# Install
INSTALL_DIR="/usr/local/bin"
if [ -w "${INSTALL_DIR}" ]; then
  mv "${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
  INSTALL_DIR="${HOME}/.local/bin"
  mkdir -p "${INSTALL_DIR}"
  mv "${BINARY}" "${INSTALL_DIR}/${BINARY}"
  echo ""
  echo "Note: installed to ${INSTALL_DIR}/${BINARY}"
  echo "Make sure ${INSTALL_DIR} is in your PATH:"
  echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

echo ""
echo "skillsync ${VERSION} installed successfully → ${INSTALL_DIR}/${BINARY}"
