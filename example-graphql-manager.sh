#!/bin/bash

# Example: Using the GraphQL Request Manager
# This demonstrates how to save and execute GraphQL requests

echo "GraphQL Request Manager - Example Usage"
echo "========================================"
echo ""

# Build the tool if not already built
if [ ! -f "./graphql-tester" ]; then
    echo "Building graphql-tester..."
    go build -o graphql-tester cmd/graphql-tester/main.go
fi

# Example 1: Import GraphQL operations from the samples
echo "Example 1: Importing GraphQL operations"
echo "----------------------------------------"
echo ""

SAMPLE_DIR="graphql-operations-unique"
if [ -d "$SAMPLE_DIR" ]; then
    echo "Importing operations from $SAMPLE_DIR/"
    
    # Import a few sample operations
    for file in "$SAMPLE_DIR/AuthConfig.json" "$SAMPLE_DIR/FetchUsers.json" "$SAMPLE_DIR/ThreadList.json"; do
        if [ -f "$file" ]; then
            name=$(basename "$file" .json)
            echo "  Importing $name..."
            ./graphql-tester -url "https://api.example.com/graphql" \
                             -data "$file" \
                             -name "$name" 2>&1 | grep "Saved"
        fi
    done
    
    echo ""
fi

# Example 2: List saved requests
echo "Example 2: Listing saved requests"
echo "----------------------------------"
echo ""
./graphql-tester -list
echo ""

# Example 3: Execute a request (commented out - requires live API)
echo "Example 3: Executing a request"
echo "-------------------------------"
echo ""
echo "To execute a request:"
echo "  ./graphql-tester -url \"https://your-api.com/graphql\" \\"
echo "                   -data graphql-operations-unique/AuthConfig.json \\"
echo "                   -token \"your-token\" \\"
echo "                   -execute"
echo ""
echo "Or use the Web UI at: http://localhost:9000/requests.html"
echo ""

# Example 4: Before/After Analysis
echo "Example 4: Before/After Analysis Workflow"
echo "------------------------------------------"
echo ""
echo "1. Execute a baseline request:"
echo "   ./graphql-tester -url \"https://staging.api.com/graphql\" \\"
echo "                    -data operations/query.json \\"
echo "                    -execute"
echo ""
echo "2. Make your code changes and deploy"
echo ""
echo "3. Execute the same request again:"
echo "   ./graphql-tester -url \"https://staging.api.com/graphql\" \\"
echo "                    -data operations/query.json \\"
echo "                    -execute"
echo ""
echo "4. Compare in Web UI:"
echo "   - Open http://localhost:9000/requests.html"
echo "   - Select your request"
echo "   - Click on each execution to compare:"
echo "     * SQL query counts"
echo "     * Query durations"
echo "     * Response times"
echo "     * Log patterns"
echo ""

# Example 5: Bulk import
echo "Example 5: Bulk Import All Operations"
echo "--------------------------------------"
echo ""
echo "To import all GraphQL operations:"
echo ""
cat <<'EOF'
for file in graphql-operations-unique/*.json; do
  name=$(basename "$file" .json)
  ./graphql-tester -url "https://api.example.com/graphql" \
                   -data "$file" \
                   -name "$name"
done
EOF
echo ""

echo "Tips:"
echo "-----"
echo "- Use descriptive names for requests"
echo "- Set BEARER_TOKEN environment variable for default auth"
echo "- Increase timeout for slow endpoints: -timeout 30s"
echo "- Use the web UI for visual comparison and analysis"
echo "- SQL queries are automatically extracted from logs"
echo ""
