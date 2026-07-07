#!/usr/bin/env sh
# FreeX Claw installer for macOS / Linux.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/CooDdk/freexclaw/master/install.sh | sh
#
# Environment overrides:
#   FREEXCLAW_VERSION       Install a specific tag (default: latest release).
#   FREEXCLAW_INSTALL_DIR   Install directory (default: $HOME/.local/bin).

set -eu

REPO="CooDdk/freexclaw"
BINARY="freexclaw"
INSTALL_DIR="${FREEXCLAW_INSTALL_DIR:-$HOME/.local/bin}"

err() {
    printf 'error: %s\n' "$1" >&2
    exit 1
}

need() {
    command -v "$1" >/dev/null 2>&1 || err "missing required command: $1"
}

detect_platform() {
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)
    case "$os" in
        linux|darwin) ;;
        *) err "unsupported OS: $os" ;;
    esac
    case "$arch" in
        x86_64|amd64) arch=amd64 ;;
        arm64|aarch64) arch=arm64 ;;
        *) err "unsupported architecture: $arch" ;;
    esac
    printf '%s-%s' "$os" "$arch"
}

resolve_version() {
    if [ -n "${FREEXCLAW_VERSION:-}" ]; then
        printf '%s' "$FREEXCLAW_VERSION"
        return
    fi
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' \
        | head -n1 \
        | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/'
}

download_binary() {
    version="$1"
    platform="$2"
    url="https://github.com/${REPO}/releases/download/${version}/${BINARY}-${platform}"
    tmp=$(mktemp)
    trap 'rm -f "$tmp"' EXIT
    printf 'Downloading %s\n' "$url"
    curl -fsSL "$url" -o "$tmp" || err "download failed"
    chmod +x "$tmp"
    mkdir -p "$INSTALL_DIR"
    mv "$tmp" "$INSTALL_DIR/$BINARY"
    trap - EXIT
}

ensure_path() {
    case ":${PATH:-}:" in
        *":$INSTALL_DIR:"*) return ;;
    esac
    shell_name=$(basename "${SHELL:-sh}")
    case "$shell_name" in
        bash) rc="$HOME/.bashrc" ;;
        zsh)  rc="$HOME/.zshrc" ;;
        fish) rc="$HOME/.config/fish/config.fish" ;;
        *)    rc="$HOME/.profile" ;;
    esac
    marker="# added by freexclaw installer"
    if [ -f "$rc" ] && grep -Fq "$marker" "$rc" 2>/dev/null; then
        return
    fi
    mkdir -p "$(dirname "$rc")"
    {
        printf '\n%s\n' "$marker"
        if [ "$shell_name" = "fish" ]; then
            printf 'set -gx PATH %s $PATH\n' "$INSTALL_DIR"
        else
            printf 'export PATH="%s:$PATH"\n' "$INSTALL_DIR"
        fi
    } >> "$rc"
    printf 'Added %s to PATH in %s\n' "$INSTALL_DIR" "$rc"
}

main() {
    need curl
    need uname
    platform=$(detect_platform)
    version=$(resolve_version)
    [ -n "$version" ] || err "failed to resolve version (set FREEXCLAW_VERSION to override)"
    printf 'Installing freexclaw %s for %s\n' "$version" "$platform"
    download_binary "$version" "$platform"
    ensure_path
    printf '\nInstalled: %s/%s\n' "$INSTALL_DIR" "$BINARY"
    printf 'Open a new terminal, then run: %s\n' "$BINARY"
}

main "$@"
