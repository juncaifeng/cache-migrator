#!/usr/bin/env bash
set -euo pipefail

REPO="juncaifeng/cache-migrator"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY="cache-migrator"

# 默认下载 latest release；可通过 VERSION=v0.1.0 指定
VERSION="${VERSION:-latest}"

# 探测系统和架构
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "不支持的架构: $ARCH"; exit 1 ;;
esac

case "$OS" in
    linux|darwin) ;;
    *) echo "不支持的操作系统: $OS"; exit 1 ;;
esac

if [ "$VERSION" = "latest" ]; then
    URL="https://github.com/${REPO}/releases/latest/download/${BINARY}-${OS}-${ARCH}"
else
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY}-${OS}-${ARCH}"
fi

echo "下载 ${BINARY} ${VERSION} for ${OS}/${ARCH} ..."
echo "URL: ${URL}"

tmpfile=$(mktemp)
trap 'rm -f "$tmpfile"' EXIT

curl -fsSL -o "$tmpfile" "$URL" || {
    echo "下载失败，请检查网络或版本号是否存在"
    exit 1
}

chmod +x "$tmpfile"

echo "安装到 ${INSTALL_DIR}/${BINARY} ..."
if [ -w "$INSTALL_DIR" ]; then
    mv "$tmpfile" "${INSTALL_DIR}/${BINARY}"
else
    sudo mv "$tmpfile" "${INSTALL_DIR}/${BINARY}"
fi

echo "安装完成: $(${INSTALL_DIR}/${BINARY} --help | head -1)"
