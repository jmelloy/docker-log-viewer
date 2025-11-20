# Phase 1b: Update cmd files to use new httputil and sqlutil packages

## Summary
We have successfully created two new utility packages (`pkg/httputil` and `pkg/sqlutil`) that consolidate duplicate code from across the `cmd/` directories. Now we need to update the cmd files to use these packages.

## Functions to Replace in cmd/viewer/main.go

### HTTP Utilities (from pkg/httputil)
1. **generateRequestID()** → **httputil.GenerateRequestID()**
   - Line 2869: Function definition to remove
   - Lines to update: Search for all calls

2. **makeHTTPRequest()** → **httputil.MakeHTTPRequest()**
   - Line 2913: Function definition to remove
   - Parameters are identical, direct replacement

3. **containsErrorsKey()** → **httputil.ContainsErrorsKey()**
   - Line 2876: Function definition to remove
   - Direct replacement in calls

4. **collectLogsForRequest()** → **httputil.CollectLogsForRequest()**
   - Line 2953: Method definition to remove
   - **IMPORTANT**: Function signature changed - need to add `wa.logStore` parameter
   - Old: `wa.collectLogsForRequest(requestID, timeout)`
   - New: `httputil.CollectLogsForRequest(requestID, wa.logStore, timeout)`

### SQL Utilities (from pkg/sqlutil)
1. **extractSQLQueries()** → **sqlutil.ExtractSQLQueries()**
   - Line 2976: Function definition to remove
   - Direct replacement

2. **interpolateSQLQuery()** → **sqlutil.InterpolateSQLQuery()**
   - Function definition around line 1736
   - Direct replacement

3. **formatSQLForDisplay()** → **sqlutil.FormatSQLForDisplay()**
   - Function definition around line 2315
   - Direct replacement

4. **formatExplainPlanForNotion()** → **sqlutil.FormatExplainPlanForNotion()**
   - Function definition around line 2402
   - Direct replacement

5. **substituteVariables()** → **sqlutil.SubstituteVariables()**
   - Function definition around line 1798
   - Direct replacement

6. **convertVariablesToMap()** → **sqlutil.ConvertVariablesToMap()**
   - Function definition around line 1776
   - Direct replacement

7. **formatSQLWithNpx()** and **formatSQLBasic()** 
   - Internal functions, can be removed (used by FormatSQLForDisplay)

## Step-by-Step Process

### 1. Update imports in cmd/viewer/main.go
Add to the import block:
```go
"docker-log-parser/pkg/httputil"
"docker-log-parser/pkg/sqlutil"
```

Remove if no longer used elsewhere:
```go
"crypto/rand"
"encoding/hex"
```

### 2. Replace function calls
Use global search and replace:
```bash
# HTTP utilities
sed -i 's/\bgenerateRequestID()/httputil.GenerateRequestID()/g' cmd/viewer/main.go
sed -i 's/\bmakeHTTPRequest(/httputil.MakeHTTPRequest(/g' cmd/viewer/main.go
sed -i 's/\bcontainsErrorsKey(/httputil.ContainsErrorsKey(/g' cmd/viewer/main.go

# Special case: collectLogsForRequest needs manual editing to add wa.logStore parameter

# SQL utilities
sed -i 's/\bextractSQLQueries(/sqlutil.ExtractSQLQueries(/g' cmd/viewer/main.go
sed -i 's/\binterpolateSQLQuery(/sqlutil.InterpolateSQLQuery(/g' cmd/viewer/main.go
sed -i 's/\bformatSQLForDisplay(/sqlutil.FormatSQLForDisplay(/g' cmd/viewer/main.go
sed -i 's/\bformatExplainPlanForNotion(/sqlutil.FormatExplainPlanForNotion(/g' cmd/viewer/main.go
sed -i 's/\bsubstituteVariables(/sqlutil.SubstituteVariables(/g' cmd/viewer/main.go
sed -i 's/\bconvertVariablesToMap(/sqlutil.ConvertVariablesToMap(/g' cmd/viewer/main.go
```

### 3. Manually update collectLogsForRequest calls
Find instances of:
```go
wa.collectLogsForRequest(requestIDHeader, 10*time.Second)
```

Replace with:
```go
httputil.CollectLogsForRequest(requestIDHeader, wa.logStore, 10*time.Second)
```

### 4. Remove function definitions
Remove these function definitions from cmd/viewer/main.go:
- generateRequestID() (line ~2869)
- containsErrorsKey() (line ~2876)
- makeHTTPRequest() (line ~2913)
- collectLogsForRequest() (line ~2953)
- extractSQLQueries() (line ~2976)
- interpolateSQLQuery() (line ~1736)
- convertVariablesToMap() (line ~1776)
- substituteVariables() (line ~1798)
- formatSQLForDisplay() (line ~2315)
- formatSQLWithNpx() (line ~2370)
- formatSQLBasic() (line ~2393)
- formatExplainPlanForNotion() (line ~2402)

### 5. Test compilation
```bash
go build ./cmd/viewer
```

### 6. Run tests
```bash
go test ./...
```

## Similar Updates for Other cmd Files

### cmd/compare/main.go
Update these functions:
- generateRequestID() → httputil.GenerateRequestID()
- extractSQLQueries() → sqlutil.ExtractSQLQueries()
- formatSQL() → sqlutil.FormatSQLForDisplay() (if similar)

### cmd/graphql-tester/main.go
Update these functions:
- generateRequestID() → httputil.GenerateRequestID()
- makeRequest() → httputil.MakeHTTPRequest()
- extractSQLQueries() → sqlutil.ExtractSQLQueries()
- collectLogs() → httputil.CollectLogsForRequest()

## Expected Results
- cmd/viewer/main.go should be ~250 lines shorter
- cmd/compare/main.go should be ~50 lines shorter
- cmd/graphql-tester/main.go should be ~100 lines shorter
- All builds should pass
- All existing tests should still pass
- No functionality should change

## Notes
- The existing test failure in cmd/compare is unrelated to this refactoring
- Two tests failures in pkg/logs and pkg/logstore are pre-existing
- All tests for the new httputil and sqlutil packages pass
