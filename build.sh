#!/bin/bash

# Exit on error
set -e

# Load versions
source version.txt

# Set Go version and path
export PATH=$HOME/sdk/go${GO_VERSION}/bin:$PATH

# Check Go version
check_go_version() {
    CURRENT_GO=$(go version | awk '{print $3}' | sed 's/go//')
    if [ "$CURRENT_GO" != "$GO_VERSION" ]; then
        echo "Current Go version ($CURRENT_GO) doesn't match required version ($GO_VERSION)"
        echo "Please ensure Go $GO_VERSION is installed and in your PATH"
        echo "You can install it using: go install golang.org/dl/go${GO_VERSION}@latest && go${GO_VERSION} download"
        exit 1
    fi
    echo "Using Go version: $GO_VERSION"
}

# Print banner with versions
print_banner() {
    if [ -f "banner/banner.txt" ]; then
        sed "s/\${APP_VERSION}/$APP_VERSION/g; s/\${GO_VERSION}/$GO_VERSION/g" banner/banner.txt
        echo ""
    fi
}

# Create directories
mkdir -p build
mkdir -p banner

# Print banner
print_banner

# Check Go version
check_go_version

# Clean Go cache and modcache
echo "Cleaning Go cache..."
go clean -cache -modcache -i -r

# Install or update garble
echo "Installing/updating garble..."
go install mvdan.cc/garble@latest

# Verify garble installation
if ! command -v garble >/dev/null 2>&1; then
    echo "Failed to install garble. Aborting." >&2
    exit 1
fi

# Ensure UPX is installed
command -v upx >/dev/null 2>&1 || { echo "UPX is required but not installed. Please install UPX first. Aborting." >&2; exit 1; }

# Build function with error handling
build() {
    local OS=$1
    local ARCH=$2
    local SUFFIX=$3
    
    echo "Building for $OS/$ARCH..."
    
    export GOOS=$OS
    export GOARCH=$ARCH
    export CGO_ENABLED=0
    
    # Clean the Go cache before each build
    go clean -cache -modcache
    
    # Try regular build first
    echo "Attempting regular build..."
    if go build -trimpath -ldflags="-s -w" -o "build/bluelink$SUFFIX" ./cmd/bluelink; then
        echo "Regular build successful"
        # Only run UPX on executables that support it
        if [ "$OS" != "darwin" ]; then
            echo "Applying UPX compression..."
            if ! upx --best --lzma "build/bluelink$SUFFIX"; then
                echo "UPX compression failed for $OS/$ARCH, continuing with uncompressed binary..."
            fi
        fi
        return 0
    fi
    
    echo "Regular build failed, attempting garble build..."
    # If regular build fails, try garble
    if garble -tiny -literals -seed=random build -trimpath -ldflags="-s -w" -o "build/bluelink$SUFFIX" ./cmd/bluelink; then
        echo "Garble build successful"
        # Only run UPX on executables that support it
        if [ "$OS" != "darwin" ]; then
            echo "Applying UPX compression..."
            if ! upx --best --lzma "build/bluelink$SUFFIX"; then
                echo "UPX compression failed for $OS/$ARCH, continuing with uncompressed binary..."
            fi
        fi
        return 0
    fi
    
    echo "Both regular and garble builds failed for $OS/$ARCH"
    return 1
}

# Clean build directory
echo "Cleaning build directory..."
rm -rf build/*

# Install project dependencies
echo "Installing project dependencies..."
go mod download
go mod tidy

# Build for different platforms
build "linux" "amd64" "_linux_amd64"
build "linux" "arm64" "_linux_arm64"
build "windows" "amd64" "_windows_amd64.exe"
build "darwin" "amd64" "_darwin_amd64"
build "darwin" "arm64" "_darwin_arm64"

# Make the script executable
chmod +x build/bluelink*

echo "Build complete! Binaries are in the build directory."
