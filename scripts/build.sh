#!/bin/bash

# Build script with version injection (Phase 4)

set -e

# Detect version
if [ -n "$(git describe --tags --always 2>/dev/null)" ]; then
    VERSION=$(git describe --tags --always --dirty)
else
    VERSION="dev"
fi

# Git commit
if [ -n "$(git rev-parse HEAD 2>/dev/null)" ]; then
    COMMIT=$(git rev-parse HEAD)
else
    COMMIT="unknown"
fi

# Build date
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Go version
GO_VERSION=$(go version | awk '{print $3}')

# Build flags
LDFLAGS="-X reel/internal/version.Version=${VERSION} \
         -X reel/internal/version.GitCommit=${COMMIT} \
         -X reel/internal/version.BuildDate=${DATE} \
         -X reel/internal/version.GoVersion=${GO_VERSION}"

echo "Building reel..."
echo "  Version:    ${VERSION}"
echo "  Commit:     ${COMMIT}"
echo "  Build Date: ${DATE}"
echo "  Go Version: ${GO_VERSION}"
echo ""

# Build
go build -ldflags="${LDFLAGS}" -o reel ./cmd/reel

echo "✅ Build complete: ./reel"
echo ""
echo "Run './reel --version' to verify"
