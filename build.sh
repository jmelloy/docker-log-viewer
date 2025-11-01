#!/bin/bash
set -e

mkdir -p bin

echo "Building Docker Log Viewer..."
go build -o bin/docker-log-viewer cmd/viewer/main.go

echo "Building Comparison Tool..."
go build -o bin/compare cmd/compare/main.go

echo "Building GraphQL Tester..."
go build -o bin/graphql-tester cmd/graphql-tester/main.go

echo "Building Analyze Tool..."
go build -o bin/analyze cmd/analyze/main.go

echo "Building Test Parser..."
go build -o bin/test-parser cmd/test-parser/main.go

echo ""
echo "Build complete!"
echo "  - ./bin/docker-log-viewer - Web-based log viewer"
echo "  - ./bin/compare - API comparison tool"
echo "  - ./bin/graphql-tester - GraphQL request manager CLI"
echo "  - ./bin/analyze - Query analysis tool for two execution IDs"
