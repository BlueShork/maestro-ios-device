#!/bin/sh
#
# maestro-ios-device setup script
# Downloads the correct binary and runs setup
#

set -e

# This fork's releases: devicelab's binary only patches maestro 2.0.9-2.1.0.
REPO="BlueShork/maestro-ios-device"
BINARY_NAME="maestro-ios-device"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║  maestro-ios-device                                            ║"
echo "║                                                                ║"
echo "║  🚀 We built maestro-runner from scratch — 3.6x faster,        ║"
echo "║  real iOS device support, runs locally or on any Appium        ║"
echo "║  cloud, true parallel execution, no paywall.                   ║"
echo "║  Fixes 78% of Maestro's top issues. Same YAML.                 ║"
echo "║                                                                ║"
echo "║  https://github.com/devicelab-dev/maestro-runner               ║"
echo "║                                                                ║"
echo "║  Built by DeviceLab — https://devicelab.dev                    ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""
echo "This will:"
echo "  1. Download maestro-ios-device binary"
echo "  2. Download and extract patched JARs to ~/.maestro/lib"
echo "  3. Download and extract iOS runner to ~/.maestro/maestro-ios-xctest-runner"
echo ""

# -y / --yes, or no terminal (CI, provisioning over ssh): install without asking.
case "${1:-}" in
    -y | --yes) REPLY=y ;;
    *)
        if [ -t 0 ] || { [ -r /dev/tty ] && (: </dev/tty) 2>/dev/null; }; then
            printf "Continue with installation? [y/N] "
            read -r REPLY </dev/tty
        else
            REPLY=y
        fi
        ;;
esac
if [ "$REPLY" != "y" ] && [ "$REPLY" != "Y" ]; then
    echo "Installation cancelled."
    exit 1
fi

echo ""

# Check OS
OS=$(uname -s)
if [ "$OS" != "Darwin" ]; then
    printf "${RED}Error: Only macOS is supported${NC}\n"
    exit 1
fi

# Check architecture
ARCH=$(uname -m)
case $ARCH in
    x86_64)
        ARCH="amd64"
        ;;
    arm64)
        ARCH="arm64"
        ;;
    *)
        printf "${RED}Error: Unsupported architecture: $ARCH${NC}\n"
        exit 1
        ;;
esac

BINARY="maestro-ios-device-darwin-${ARCH}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${BINARY}"

echo "Detected: macOS ${ARCH}"
echo ""

# Find where maestro is installed
MAESTRO_PATH=$(which maestro 2>/dev/null || true)
if [ -z "$MAESTRO_PATH" ]; then
    printf "${RED}Error: Maestro not found in PATH. Install Maestro first: https://maestro.mobile.dev${NC}\n"
    exit 1
fi

INSTALL_DIR=$(dirname "$MAESTRO_PATH")
echo "Found Maestro at: $INSTALL_DIR"
echo ""

# Download binary
echo "📥 Downloading ${BINARY}..."
TMP_FILE=$(mktemp)
if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_FILE"; then
    printf "${RED}Error: Failed to download binary${NC}\n"
    echo "URL: $DOWNLOAD_URL"
    rm -f "$TMP_FILE"
    exit 1
fi

# Install binary
echo "📦 Installing to $INSTALL_DIR/$BINARY_NAME..."
mv "$TMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
chmod +x "$INSTALL_DIR/$BINARY_NAME"

printf "${GREEN}✅ Binary installed${NC}\n"
echo ""

# Run setup command
echo "🔧 Running maestro-ios-device setup..."
echo ""
"$INSTALL_DIR/$BINARY_NAME" setup

echo ""
printf "${GREEN}✅ Installation complete!${NC}\n"
