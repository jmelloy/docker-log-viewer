#!/bin/bash

# Example usage script for the analyze tool
# This demonstrates various use cases

echo "=== Query Analysis Tool - Example Usage ==="
echo ""

# Check if the tool is built
if [ ! -f "../../bin/analyze" ]; then
    echo "Error: analyze tool not found. Please run ./build.sh first"
    exit 1
fi

ANALYZE="../../bin/analyze"

echo "1. Basic Usage:"
echo "   Compare queries from two executions"
echo "   Command: $ANALYZE -exec1 1 -exec2 2"
echo ""

echo "2. Save to File:"
echo "   Save analysis report to a file"
echo "   Command: $ANALYZE -exec1 1 -exec2 2 -output report.txt"
echo ""

echo "3. Verbose Mode:"
echo "   Include detailed query lists"
echo "   Command: $ANALYZE -exec1 1 -exec2 2 -verbose"
echo ""

echo "4. Custom Database:"
echo "   Use a specific database file"
echo "   Command: $ANALYZE -db /path/to/custom.db -exec1 1 -exec2 2"
echo ""

echo "5. Real-World Workflow:"
echo ""
echo "   a. Execute a request and note its ID:"
echo "      ./bin/graphql-tester -url https://api.example.com/graphql \\"
echo "                           -data query.json -execute"
echo "      # Note the execution ID, e.g., 100"
echo ""
echo "   b. Make code changes (optimize queries, add indexes, etc.)"
echo ""
echo "   c. Execute the same request again:"
echo "      ./bin/graphql-tester -url https://api.example.com/graphql \\"
echo "                           -data query.json -execute"
echo "      # Note the new execution ID, e.g., 101"
echo ""
echo "   d. Compare the two executions:"
echo "      $ANALYZE -exec1 100 -exec2 101 -output before-after.txt"
echo ""
echo "   e. Review the report to see performance changes"
echo ""

echo "6. Use Cases:"
echo "   - Performance regression testing"
echo "   - Query optimization validation"
echo "   - Index effectiveness analysis"
echo "   - A/B testing different implementations"
echo ""

echo "For more information, see cmd/analyze/README.md"
