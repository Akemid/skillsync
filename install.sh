#!/bin/sh
set -e

REPO="Akemid/skillsync"
BINARY="skillsync"

# Parse flags
WITH_SKILL=0
for arg in "$@"; do
  case "$arg" in
    --with-skill) WITH_SKILL=1 ;;
  esac
done

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
if command -v sha256sum > /dev/null 2>&1 || command -v shasum > /dev/null 2>&1; then
  # Extract expected checksum from checksums.txt
  EXPECTED_CHECKSUM=$(grep "${ARCHIVE}" checksums.txt | awk '{print $1}')

  # Calculate actual checksum
  if command -v sha256sum > /dev/null 2>&1; then
    ACTUAL_CHECKSUM=$(sha256sum "${ARCHIVE}" | awk '{print $1}')
  else
    ACTUAL_CHECKSUM=$(shasum -a 256 "${ARCHIVE}" | awk '{print $1}')
  fi

  # Compare checksums
  if [ "${EXPECTED_CHECKSUM}" != "${ACTUAL_CHECKSUM}" ]; then
    echo "Error: checksum verification failed" >&2
    echo "Expected: ${EXPECTED_CHECKSUM}" >&2
    echo "Got:      ${ACTUAL_CHECKSUM}" >&2
    exit 1
  fi
  echo "Checksum verified ✓"
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

if [ "${WITH_SKILL}" = "1" ]; then
  echo ""
  echo "Installing skillsync skill..."
  "${INSTALL_DIR}/${BINARY}" self-skill install --yes || {
    echo "Warning: could not install skillsync skill (binary installed successfully)" >&2
  }
fi
