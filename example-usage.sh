#!/bin/bash

# Example script showing how to use the compare-tool
# This demonstrates comparing two API endpoints with the same GraphQL query

echo "Docker Log Viewer - URL Comparison Tool Example"
echo "================================================"
echo ""

# Check if compare-tool exists
if [ ! -f "./compare-tool" ]; then
    echo "Building compare-tool..."
    go build -o compare-tool ./cmd/compare-tool
    if [ $? -ne 0 ]; then
        echo "Failed to build compare-tool"
        exit 1
    fi
fi

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "Error: Docker is not running"
    exit 1
fi

# Example usage with dummy URLs (replace with actual endpoints)
echo "Example 1: Comparing production vs staging"
echo "-------------------------------------------"
echo ""
echo "Command:"
echo "./compare-tool \\"
echo "  -url1 https://api.production.example.com/graphql \\"
echo "  -url2 https://api.staging.example.com/graphql \\"
echo "  -data sample-request.json \\"
echo "  -output prod-vs-staging.html \\"
echo "  -timeout 15s"
echo ""

echo "Example 2: Comparing different GraphQL operations"
echo "-------------------------------------------------"
echo ""
echo "Command:"
echo "./compare-tool \\"
echo "  -url1 https://api.example.com/graphql \\"
echo "  -url2 https://api.example.com/graphql \\"
echo "  -data query1.json \\"
echo "  -output query-comparison.html"
echo ""

echo "Example 3: Using with environment variables"
echo "--------------------------------------------"
echo ""
echo "Command:"
cat <<'EOF'
export URL1="https://api.prod.example.com/graphql"
export URL2="https://api.dev.example.com/graphql"
./compare-tool \
  -url1 "$URL1" \
  -url2 "$URL2" \
  -data sample-request.json \
  -output comparison.html
EOF
echo ""

echo "Sample GraphQL Request (sample-request.json):"
echo "----------------------------------------------"
cat sample-request.json
echo ""
echo ""

echo "Notes:"
echo "------"
echo "1. Make sure your Docker containers are running and logging"
echo "2. Ensure your API returns the X-Request-Id header"
echo "3. Logs must include request_id field matching the header"
echo "4. SQL queries should be logged with [sql]: prefix"
echo "5. Increase -timeout if your requests take longer to process"
echo ""
echo "For more information, see COMPARE-TOOL.md"
