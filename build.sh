#!/bin/bash
# build.sh - 跨平台构建脚本

set -e

APP_NAME="Zem"
VERSION="1.0.0"

echo "Building ${APP_NAME} v${VERSION}..."

# 安装依赖
echo "Installing dependencies..."
go mod tidy
cd frontend && npm install && cd ..

# Windows (需要 mingw-w64)
echo "Building for Windows..."
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc wails build -platform windows/amd64 -ldflags "-s -w -buildid=" -tags "with_utls with_quic with_gvisor" -o "dist/${APP_NAME}-windows-amd64.exe"
if [ -f "build/bin/dist/${APP_NAME}-windows-amd64.exe" ]; then
    cp "build/bin/dist/${APP_NAME}-windows-amd64.exe" "dist/${APP_NAME}-windows-amd64.exe"
fi

# macOS
echo "Building for macOS..."
GOOS=darwin GOARCH=amd64 wails build -platform darwin/amd64 -ldflags "-s -w -buildid=" -tags "with_utls with_quic with_gvisor" -o "dist/${APP_NAME}-darwin-amd64"
if [ -f "build/bin/dist/${APP_NAME}-darwin-amd64" ]; then
    cp "build/bin/dist/${APP_NAME}-darwin-amd64" "dist/${APP_NAME}-darwin-amd64"
fi

GOOS=darwin GOARCH=arm64 wails build -platform darwin/arm64 -ldflags "-s -w -buildid=" -tags "with_utls with_quic with_gvisor" -o "dist/${APP_NAME}-darwin-arm64"
if [ -f "build/bin/dist/${APP_NAME}-darwin-arm64" ]; then
    cp "build/bin/dist/${APP_NAME}-darwin-arm64" "dist/${APP_NAME}-darwin-arm64"
fi

# Linux
echo "Building for Linux..."
GOOS=linux GOARCH=amd64 wails build -platform linux/amd64 -ldflags "-s -w -buildid=" -tags "with_utls with_quic with_gvisor" -o "dist/${APP_NAME}-linux-amd64"
if [ -f "build/bin/dist/${APP_NAME}-linux-amd64" ]; then
    cp "build/bin/dist/${APP_NAME}-linux-amd64" "dist/${APP_NAME}-linux-amd64"
fi

echo "Build complete!"
