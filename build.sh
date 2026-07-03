#!/bin/bash

set -e

VERSION="0.1.0"
DIST_DIR="dist"

echo "========== FreeX Claw 多平台构建 =========="

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

echo ""
echo "Building Windows amd64..."
GOOS=windows GOARCH=amd64 go build -o "$DIST_DIR/freexclaw-windows-amd64.exe" ./cmd/
echo "✅ freexclaw-windows-amd64.exe"

echo ""
echo "Building macOS amd64..."
GOOS=darwin GOARCH=amd64 go build -o "$DIST_DIR/freexclaw-darwin-amd64" ./cmd/
echo "✅ freexclaw-darwin-amd64"

echo ""
echo "Building macOS arm64..."
GOOS=darwin GOARCH=arm64 go build -o "$DIST_DIR/freexclaw-darwin-arm64" ./cmd/
echo "✅ freexclaw-darwin-arm64"

echo ""
echo "Building Linux amd64..."
GOOS=linux GOARCH=amd64 go build -o "$DIST_DIR/freexclaw-linux-amd64" ./cmd/
echo "✅ freexclaw-linux-amd64"

echo ""
echo "Building Linux arm64..."
GOOS=linux GOARCH=arm64 go build -o "$DIST_DIR/freexclaw-linux-arm64" ./cmd/
echo "✅ freexclaw-linux-arm64"

echo ""
echo "========== 构建完成 =========="
echo ""
ls -la "$DIST_DIR/"
echo ""
echo "文件位于: $(pwd)/$DIST_DIR"