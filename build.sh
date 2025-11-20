#!/bin/bash
set -e

mkdir -p bin

echo "Building Frontend (TypeScript + Vue)..."
cd web
if [ ! -d "node_modules" ]; then
  echo "Installing npm dependencies..."
  npm install
fi
npm run build
cd ..

echo ""
echo "Building Docker Log Viewer..."
go build -o bin/docker-log-viewer cmd/viewer/main.go

echo "Building GraphQL Tester..."
go build -o bin/graphql-tester cmd/graphql-tester/main.go

echo "Building Analyze Tool..."
go build -o bin/analyze cmd/analyze/main.go

echo "Building Test Parser..."
go build -o bin/test-parser cmd/test-parser/main.go

echo ""
echo "Build complete!"
echo "  - ./bin/docker-log-viewer - Web-based log viewer (frontend: web/dist/)"
echo "  - ./bin/graphql-tester - GraphQL request manager CLI"
echo "  - ./bin/analyze - Query analysis tool for two execution IDs"
