#!/bin/sh

# Get Git information
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "unknown")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(git log -1 --format=%cd --date=iso | date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "unknown")

# Build with ldflags
go build -ldflags "-s -w -X github.com/nicholas-fedor/shoutrrr/internal/meta.Version=$VERSION -X github.com/nicholas-fedor/shoutrrr/internal/meta.Commit=$COMMIT -X github.com/nicholas-fedor/shoutrrr/internal/meta.Date=$DATE" -o shoutrrr ./shoutrrr
