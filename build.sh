#!/bin/bash

# Set up Go environment
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin

# Add GOPATH/bin to PATH if not already present
if [[ ":$PATH:" != *":$HOME/go/bin:"* ]]; then
    export PATH="$PATH:$HOME/go/bin"
fi

# Check if go1.23.4 is installed
if ! command -v go1.23.4 &> /dev/null; then
    echo "Installing Go 1.23.4 for garble compatibility..."
    go install golang.org/dl/go1.23.4@latest
    go1.23.4 download
fi

# Check if upx is installed
if ! command -v upx &> /dev/null; then
    echo "UPX is required but not installed."
    echo "Please install UPX:"
    echo "  - macOS: brew install upx"
    echo "  - Linux: sudo apt-get install upx"
    echo "  - Windows: choco install upx"
    exit 1
fi

# Build directory
mkdir -p build

# Create a temporary directory for Go 1.23.4 module
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

# Initialize a new module
go1.23.4 mod init temp

# Get a specific version of garble
echo "Installing garble v0.7.1 with Go 1.23.4..."
go1.23.4 get mvdan.cc/garble@v0.7.1
go1.23.4 install mvdan.cc/garble@v0.7.1

# Return to the original directory
cd - > /dev/null

# Verify garble installation
if ! command -v garble &> /dev/null; then
    echo "Failed to install garble, using regular build..."
    USE_GARBLE=false
else
    echo "Garble installed successfully, will use for obfuscation"
    USE_GARBLE=true
fi

# Build for different platforms
PLATFORMS=("darwin/amd64" "darwin/arm64" "linux/amd64" "windows/amd64")
OUTPUT_NAMES=("bluelink-darwin-amd64" "bluelink-darwin-arm64" "bluelink-linux-amd64" "bluelink-windows-amd64.exe")

for i in "${!PLATFORMS[@]}"; do
    platform=${PLATFORMS[$i]}
    output=${OUTPUT_NAMES[$i]}
    
    echo "Building for $platform..."
    
    # Split platform into OS and architecture
    IFS="/" read -r -a array <<< "$platform"
    GOOS=${array[0]}
    GOARCH=${array[1]}
    
    # Build command
    if [ "$USE_GARBLE" = true ]; then
        echo "Building with garble using Go 1.23.4..."
        # Clean the build cache before each build
        go1.23.4 clean -cache
        if ! GOOS=$GOOS GOARCH=$GOARCH PATH=$GOPATH/bin:$PATH garble -tiny -literals build \
            -trimpath \
            -ldflags="-s -w" \
            -o "build/$output" \
            ./cmd/bluelink; then
            echo "Garble build failed, falling back to regular build..."
            GOOS=$GOOS GOARCH=$GOARCH go build \
                -trimpath \
                -ldflags="-s -w" \
                -o "build/$output" \
                ./cmd/bluelink
        fi
    else
        echo "Using regular build..."
        GOOS=$GOOS GOARCH=$GOARCH go build \
            -trimpath \
            -ldflags="-s -w" \
            -o "build/$output" \
            ./cmd/bluelink
    fi
    
    # Apply UPX compression if not on macOS ARM64 (UPX doesn't support it yet)
    if [ "$platform" != "darwin/arm64" ]; then
        echo "Applying UPX compression to $output..."
        if [ "$GOOS" = "darwin" ]; then
            # Skip UPX for macOS binaries
            echo "Skipping UPX for macOS binaries (not supported)"
        else
            upx --brute "build/$output"
        fi
    fi
done

# Clean up temporary directory
rm -rf "$TEMP_DIR"

echo "Build complete! Binaries are in the build directory." 