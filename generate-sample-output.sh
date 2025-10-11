#!/bin/bash

# Create a mock HTML output to demonstrate the compare tool
# This shows what the output would look like

echo "Generating sample HTML output..."

cat > sample-output.html <<'EOF'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>URL Comparison Report - Sample</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
            background: #0d1117;
            color: #c9d1d9;
            padding: 20px;
            line-height: 1.6;
        }
        .container { max-width: 1400px; margin: 0 auto; }
        h1 { color: #58a6ff; margin-bottom: 20px; font-size: 2rem; }
        h2 { color: #58a6ff; margin: 30px 0 15px; font-size: 1.5rem; }
        h3 { color: #8b949e; margin: 20px 0 10px; font-size: 1.2rem; }
        .comparison { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }
        .result {
            background: #161b22;
            border: 1px solid #30363d;
            border-radius: 6px;
            padding: 20px;
        }
        .header { margin-bottom: 20px; }
        .header .url {
            font-size: 1.1rem;
            font-weight: bold;
            color: #58a6ff;
            word-break: break-all;
        }
        .header .request-id {
            color: #79c0ff;
            font-family: monospace;
            font-size: 0.9rem;
            margin-top: 5px;
        }
        .metrics {
            display: grid;
            grid-template-columns: repeat(2, 1fr);
            gap: 10px;
            margin-bottom: 20px;
        }
        .metric {
            background: #0d1117;
            padding: 10px;
            border-radius: 4px;
            border: 1px solid #30363d;
        }
        .metric-label {
            font-size: 0.75rem;
            color: #8b949e;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }
        .metric-value {
            font-size: 1.25rem;
            font-weight: bold;
            color: #58a6ff;
            margin-top: 5px;
        }
        .metric-value.success { color: #3fb950; }
        .section {
            margin-top: 20px;
            padding: 15px;
            background: #0d1117;
            border-radius: 4px;
            border: 1px solid #30363d;
        }
        .section h4 {
            color: #8b949e;
            font-size: 0.85rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            margin-bottom: 10px;
        }
        .query-item {
            background: #161b22;
            border: 1px solid #30363d;
            border-radius: 4px;
            padding: 10px;
            margin-bottom: 10px;
        }
        .query-header {
            display: flex;
            justify-content: space-between;
            margin-bottom: 8px;
            font-size: 0.85rem;
        }
        .query-count { color: #58a6ff; font-weight: bold; }
        .query-duration { color: #79c0ff; }
        .query-slow { color: #f85149; }
        .query-text {
            font-family: monospace;
            font-size: 0.85rem;
            color: #c9d1d9;
            background: #0d1117;
            padding: 8px;
            border-radius: 3px;
            overflow-x: auto;
            margin-bottom: 8px;
        }
        .query-meta {
            display: flex;
            gap: 15px;
            font-size: 0.75rem;
            color: #8b949e;
        }
        .table-badge {
            display: inline-block;
            background: #1f6feb;
            color: #fff;
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 0.75rem;
            margin-right: 5px;
            margin-bottom: 5px;
        }
        .table-count {
            margin-left: 5px;
            opacity: 0.8;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(2, 1fr);
            gap: 10px;
            margin-bottom: 15px;
        }
        .timestamp {
            color: #8b949e;
            font-size: 0.75rem;
            margin-bottom: 20px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>URL Comparison Report</h1>
        <div class="timestamp">Generated: 2025-10-11 02:35:00 UTC</div>
        
        <div class="comparison">
            <!-- Result 1 - Production -->
            <div class="result">
                <div class="header">
                    <div class="url">URL 1: https://api.production.example.com/graphql</div>
                    <div class="request-id">Request ID: prod-req-abc123</div>
                </div>
                
                <div class="metrics">
                    <div class="metric">
                        <div class="metric-label">Status Code</div>
                        <div class="metric-value success">200</div>
                    </div>
                    <div class="metric">
                        <div class="metric-label">Duration</div>
                        <div class="metric-value">245ms</div>
                    </div>
                    <div class="metric">
                        <div class="metric-label">Logs Collected</div>
                        <div class="metric-value">42</div>
                    </div>
                    <div class="metric">
                        <div class="metric-label">SQL Queries</div>
                        <div class="metric-value">8</div>
                    </div>
                </div>
                
                <div class="section">
                    <h4>SQL Analysis</h4>
                    <div class="stats-grid">
                        <div class="metric">
                            <div class="metric-label">Total Queries</div>
                            <div class="metric-value">8</div>
                        </div>
                        <div class="metric">
                            <div class="metric-label">Unique Queries</div>
                            <div class="metric-value">5</div>
                        </div>
                        <div class="metric">
                            <div class="metric-label">Avg Duration</div>
                            <div class="metric-value">12.45ms</div>
                        </div>
                        <div class="metric">
                            <div class="metric-label">Total Duration</div>
                            <div class="metric-value">99.60ms</div>
                        </div>
                    </div>
                </div>
                
                <div class="section">
                    <h4>Slowest Queries</h4>
                    <div class="query-item">
                        <div class="query-header">
                            <span class="query-duration query-slow">24.50ms</span>
                        </div>
                        <div class="query-text">SELECT * FROM users WHERE id = $1</div>
                        <div class="query-meta">
                            <span>Table: users</span>
                            <span>Op: select</span>
                            <span>Rows: 1</span>
                        </div>
                    </div>
                    <div class="query-item">
                        <div class="query-header">
                            <span class="query-duration">18.30ms</span>
                        </div>
                        <div class="query-text">SELECT * FROM posts WHERE user_id = $1 ORDER BY created_at DESC</div>
                        <div class="query-meta">
                            <span>Table: posts</span>
                            <span>Op: select</span>
                            <span>Rows: 15</span>
                        </div>
                    </div>
                </div>
                
                <div class="section">
                    <h4>Tables Accessed</h4>
                    <span class="table-badge">users<span class="table-count">(3)</span></span>
                    <span class="table-badge">posts<span class="table-count">(2)</span></span>
                    <span class="table-badge">preferences<span class="table-count">(3)</span></span>
                </div>
            </div>
            
            <!-- Result 2 - Staging -->
            <div class="result">
                <div class="header">
                    <div class="url">URL 2: https://api.staging.example.com/graphql</div>
                    <div class="request-id">Request ID: stg-req-def456</div>
                </div>
                
                <div class="metrics">
                    <div class="metric">
                        <div class="metric-label">Status Code</div>
                        <div class="metric-value success">200</div>
                    </div>
                    <div class="metric">
                        <div class="metric-label">Duration</div>
                        <div class="metric-value">312ms</div>
                    </div>
                    <div class="metric">
                        <div class="metric-label">Logs Collected</div>
                        <div class="metric-value">51</div>
                    </div>
                    <div class="metric">
                        <div class="metric-label">SQL Queries</div>
                        <div class="metric-value">12</div>
                    </div>
                </div>
                
                <div class="section">
                    <h4>SQL Analysis</h4>
                    <div class="stats-grid">
                        <div class="metric">
                            <div class="metric-label">Total Queries</div>
                            <div class="metric-value">12</div>
                        </div>
                        <div class="metric">
                            <div class="metric-label">Unique Queries</div>
                            <div class="metric-value">6</div>
                        </div>
                        <div class="metric">
                            <div class="metric-label">Avg Duration</div>
                            <div class="metric-value">15.78ms</div>
                        </div>
                        <div class="metric">
                            <div class="metric-label">Total Duration</div>
                            <div class="metric-value">189.36ms</div>
                        </div>
                    </div>
                </div>
                
                <div class="section">
                    <h4>Slowest Queries</h4>
                    <div class="query-item">
                        <div class="query-header">
                            <span class="query-duration query-slow">45.20ms</span>
                        </div>
                        <div class="query-text">SELECT * FROM users WHERE id = $1</div>
                        <div class="query-meta">
                            <span>Table: users</span>
                            <span>Op: select</span>
                            <span>Rows: 1</span>
                        </div>
                    </div>
                    <div class="query-item">
                        <div class="query-header">
                            <span class="query-duration query-slow">32.10ms</span>
                        </div>
                        <div class="query-text">SELECT * FROM posts WHERE user_id = $1 ORDER BY created_at DESC</div>
                        <div class="query-meta">
                            <span>Table: posts</span>
                            <span>Op: select</span>
                            <span>Rows: 15</span>
                        </div>
                    </div>
                </div>
                
                <div class="section">
                    <h4>Potential N+1 Issues</h4>
                    <div class="query-item">
                        <div class="query-header">
                            <span class="query-count">6x executions</span>
                        </div>
                        <div class="query-text">SELECT * FROM preferences WHERE user_id = $1</div>
                        <div class="query-meta">
                            <span>Table: preferences</span>
                            <span>Consider batching or eager loading</span>
                        </div>
                    </div>
                </div>
                
                <div class="section">
                    <h4>Tables Accessed</h4>
                    <span class="table-badge">preferences<span class="table-count">(6)</span></span>
                    <span class="table-badge">users<span class="table-count">(3)</span></span>
                    <span class="table-badge">posts<span class="table-count">(3)</span></span>
                </div>
            </div>
        </div>
    </div>
</body>
</html>
EOF

echo "Sample HTML output created: sample-output.html"
echo "Open this file in a browser to see what the comparison report looks like"
