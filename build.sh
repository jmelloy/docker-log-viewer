#!/bin/bash
set -e

echo "Building Docker Log Viewer..."
go build -o docker-log-viewer cmd/viewer/main.go

echo "Building Comparison Tool..."
go build -o compare cmd/compare/main.go

echo ""
echo "Build complete!"
echo "  - ./docker-log-viewer - Web-based log viewer"
echo "  - ./compare - API comparison tool"
