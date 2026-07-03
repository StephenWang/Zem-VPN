#!/bin/bash
# download-wintun.sh - 自动下载 wintun.dll

set -e

WINTUN_VERSION="0.14.1"
ARCH="amd64"  # 或 arm64

URL="https://www.wintun.net/builds/wintun-${WINTUN_VERSION}.zip"

# 获取项目根目录（脚本所在目录的父目录）
PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "Downloading wintun ${WINTUN_VERSION}..."
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

curl -L -o "${TMP_DIR}/wintun.zip" "${URL}"

unzip -o "${TMP_DIR}/wintun.zip" -d "${TMP_DIR}"

mkdir -p "${PROJECT_DIR}/build/windows"
cp "${TMP_DIR}/wintun/bin/${ARCH}/wintun.dll" "${PROJECT_DIR}/build/windows/"

echo "wintun.dll downloaded to build/windows/"
