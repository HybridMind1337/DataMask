#!/usr/bin/env bash
# DataMask one-liner installer for Linux and macOS
# Usage: curl -sSL https://raw.githubusercontent.com/HybridMind1337/DataMask/main/scripts/install.sh | bash

set -euo pipefail

REPO="HybridMind1337/DataMask"
BIN_NAME="datamask"
INSTALL_DIR="/usr/local/bin"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "${OS}" in
  linux*)  OS="linux" ;;
  darwin*) OS="darwin" ;;
  *) echo "Unsupported operating system: ${OS}"; exit 1 ;;
esac

# Detect Architecture
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64)  ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Unsupported CPU architecture: ${ARCH}"; exit 1 ;;
esac

echo "=> Fetching latest release for ${OS}/${ARCH}..."
TAG=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "${TAG}" ]; then
  TAG="v1.0.0"
fi

VERSION="${TAG#v}"
ARCHIVE="${BIN_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

echo "=> Downloading ${URL}..."
if ! curl -sSL "${URL}" -o "${TMP_DIR}/${ARCHIVE}"; then
  echo "Error downloading DataMask binary."
  exit 1
fi

echo "=> Extracting archive..."
tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "${TMP_DIR}"

echo "=> Installing to ${INSTALL_DIR}..."
if [ -w "${INSTALL_DIR}" ]; then
  mv "${TMP_DIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
else
  sudo mv "${TMP_DIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
fi
chmod +x "${INSTALL_DIR}/${BIN_NAME}"

echo ""
echo "=================================================="
echo " DataMask successfully installed! (${TAG})"
echo " Run 'datamask --help' or 'datamask wizard' to start."
echo "=================================================="
