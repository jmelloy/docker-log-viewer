package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"docker-log-parser/pkg/logs"

	_ "github.com/mattn/go-sqlite3"
)

// Store manages the SQLite database for request tracking
type Store struct {
	db *sql.DB
}

// Request represents a saved GraphQL/API request template
type Request struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	RequestData string    `json:"requestData"`
	BearerToken string    `json:"bearerToken,omitempty"`
	DevID       string    `json:"devId,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Execution represents a single execution of a request
type Execution struct {
	ID              int64     `json:"id"`
	RequestID       int64     `json:"requestId"`
	RequestIDHeader string    `json:"requestIdHeader"`
	StatusCode      int       `json:"statusCode"`
	DurationMS      int64     `json:"durationMs"`
	ResponseBody    string    `json:"responseBody,omitempty"`
	Error           string    `json:"error,omitempty"`
	ExecutedAt      time.Time `json:"executedAt"`
}

// ExecutionLog represents a log entry from an execution
type ExecutionLog struct {
	ID          int64     `json:"id"`
	ExecutionID int64     `json:"executionId"`
	ContainerID string    `json:"containerId"`
	Timestamp   time.Time `json:"timestamp"`
	Level       string    `json:"level"`
	Message     string    `json:"message"`
	RawLog      string    `json:"rawLog"`
	Fields      string    `json:"fields"`
}

// SQLQuery represents a SQL query extracted from logs
type SQLQuery struct {
	ID              int64   `json:"id"`
	ExecutionID     int64   `json:"executionId"`
	Query           string  `json:"query"`
	NormalizedQuery string  `json:"normalizedQuery"`
	DurationMS      float64 `json:"durationMs"`
	TableName       string  `json:"tableName"`
	Operation       string  `json:"operation"`
	Rows            int     `json:"rows"`
}

// ExecutionDetail includes execution with related logs and SQL analysis
type ExecutionDetail struct {
	Execution   Execution      `json:"execution"`
	Request     Request        `json:"request"`
	Logs        []ExecutionLog `json:"logs"`
	SQLQueries  []SQLQuery     `json:"sqlQueries"`
	SQLAnalysis *SQLAnalysis   `json:"sqlAnalysis,omitempty"`
}

// SQLAnalysis provides statistics about SQL queries
type SQLAnalysis struct {
	TotalQueries   int                `json:"totalQueries"`
	UniqueQueries  int                `json:"uniqueQueries"`
	AvgDuration    float64            `json:"avgDuration"`
	TotalDuration  float64            `json:"totalDuration"`
	TablesAccessed map[string]int     `json:"tablesAccessed"`
	NPlusOneIssues []QueryGroupResult `json:"nPlusOneIssues,omitempty"`
}

// QueryGroupResult represents grouped query statistics
type QueryGroupResult struct {
	NormalizedQuery string  `json:"normalizedQuery"`
	Count           int     `json:"count"`
	AvgDuration     float64 `json:"avgDuration"`
	Example         string  `json:"example"`
}

// NewStore creates a new store and initializes the database
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Create schema
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// CreateRequest creates a new request template
func (s *Store) CreateRequest(req *Request) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO requests (name, url, request_data, bearer_token, dev_id) 
		 VALUES (?, ?, ?, ?, ?)`,
		req.Name, req.URL, req.RequestData, req.BearerToken, req.DevID,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return id, nil
}

// GetRequest retrieves a request by ID
func (s *Store) GetRequest(id int64) (*Request, error) {
	req := &Request{}
	err := s.db.QueryRow(
		`SELECT id, name, url, request_data, bearer_token, dev_id, created_at 
		 FROM requests WHERE id = ?`,
		id,
	).Scan(&req.ID, &req.Name, &req.URL, &req.RequestData, &req.BearerToken, &req.DevID, &req.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get request: %w", err)
	}

	return req, nil
}

// ListRequests retrieves all requests
func (s *Store) ListRequests() ([]Request, error) {
	rows, err := s.db.Query(
		`SELECT id, name, url, request_data, bearer_token, dev_id, created_at 
		 FROM requests ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list requests: %w", err)
	}
	defer rows.Close()

	requests := []Request{}
	for rows.Next() {
		var req Request
		if err := rows.Scan(&req.ID, &req.Name, &req.URL, &req.RequestData, &req.BearerToken, &req.DevID, &req.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan request: %w", err)
		}
		requests = append(requests, req)
	}

	return requests, nil
}

// DeleteRequest deletes a request and all its executions
func (s *Store) DeleteRequest(id int64) error {
	_, err := s.db.Exec(`DELETE FROM requests WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete request: %w", err)
	}
	return nil
}

// CreateExecution creates a new execution record
func (s *Store) CreateExecution(exec *Execution) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO executions (request_id, request_id_header, status_code, duration_ms, response_body, error) 
		 VALUES (?, ?, ?, ?, ?, ?)`,
		exec.RequestID, exec.RequestIDHeader, exec.StatusCode, exec.DurationMS, exec.ResponseBody, exec.Error,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create execution: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return id, nil
}

// GetExecution retrieves an execution by ID
func (s *Store) GetExecution(id int64) (*Execution, error) {
	exec := &Execution{}
	err := s.db.QueryRow(
		`SELECT id, request_id, request_id_header, status_code, duration_ms, response_body, error, executed_at 
		 FROM executions WHERE id = ?`,
		id,
	).Scan(&exec.ID, &exec.RequestID, &exec.RequestIDHeader, &exec.StatusCode, &exec.DurationMS, &exec.ResponseBody, &exec.Error, &exec.ExecutedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}

	return exec, nil
}

// ListExecutions retrieves all executions for a request
func (s *Store) ListExecutions(requestID int64) ([]Execution, error) {
	rows, err := s.db.Query(
		`SELECT id, request_id, request_id_header, status_code, duration_ms, response_body, error, executed_at 
		 FROM executions WHERE request_id = ? ORDER BY executed_at DESC`,
		requestID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list executions: %w", err)
	}
	defer rows.Close()

	executions := []Execution{}
	for rows.Next() {
		var exec Execution
		if err := rows.Scan(&exec.ID, &exec.RequestID, &exec.RequestIDHeader, &exec.StatusCode, &exec.DurationMS, &exec.ResponseBody, &exec.Error, &exec.ExecutedAt); err != nil {
			return nil, fmt.Errorf("failed to scan execution: %w", err)
		}
		executions = append(executions, exec)
	}

	return executions, nil
}

// SaveExecutionLogs saves log entries for an execution
func (s *Store) SaveExecutionLogs(executionID int64, logMessages []logs.LogMessage) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO execution_logs (execution_id, container_id, timestamp, level, message, raw_log, fields) 
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, msg := range logMessages {
		var level, message, rawLog string
		var fieldsJSON []byte

		if msg.Entry != nil {
			level = msg.Entry.Level
			message = msg.Entry.Message
			rawLog = msg.Entry.Raw
			if msg.Entry.Fields != nil {
				fieldsJSON, _ = json.Marshal(msg.Entry.Fields)
			}
		}

		_, err := stmt.Exec(
			executionID,
			msg.ContainerID,
			msg.Timestamp,
			level,
			message,
			rawLog,
			string(fieldsJSON),
		)
		if err != nil {
			return fmt.Errorf("failed to insert log: %w", err)
		}
	}

	return tx.Commit()
}

// GetExecutionLogs retrieves logs for an execution
func (s *Store) GetExecutionLogs(executionID int64) ([]ExecutionLog, error) {
	rows, err := s.db.Query(
		`SELECT id, execution_id, container_id, timestamp, level, message, raw_log, fields 
		 FROM execution_logs WHERE execution_id = ? ORDER BY timestamp`,
		executionID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get execution logs: %w", err)
	}
	defer rows.Close()

	logs := []ExecutionLog{}
	for rows.Next() {
		var log ExecutionLog
		if err := rows.Scan(&log.ID, &log.ExecutionID, &log.ContainerID, &log.Timestamp, &log.Level, &log.Message, &log.RawLog, &log.Fields); err != nil {
			return nil, fmt.Errorf("failed to scan log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// SaveSQLQueries saves SQL queries for an execution
func (s *Store) SaveSQLQueries(executionID int64, queries []SQLQuery) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO sql_queries (execution_id, query, normalized_query, duration_ms, table_name, operation, rows) 
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, q := range queries {
		_, err := stmt.Exec(
			executionID,
			q.Query,
			q.NormalizedQuery,
			q.DurationMS,
			q.TableName,
			q.Operation,
			q.Rows,
		)
		if err != nil {
			return fmt.Errorf("failed to insert query: %w", err)
		}
	}

	return tx.Commit()
}

// GetSQLQueries retrieves SQL queries for an execution
func (s *Store) GetSQLQueries(executionID int64) ([]SQLQuery, error) {
	rows, err := s.db.Query(
		`SELECT id, execution_id, query, normalized_query, duration_ms, table_name, operation, rows 
		 FROM sql_queries WHERE execution_id = ? ORDER BY id`,
		executionID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get SQL queries: %w", err)
	}
	defer rows.Close()

	queries := []SQLQuery{}
	for rows.Next() {
		var q SQLQuery
		if err := rows.Scan(&q.ID, &q.ExecutionID, &q.Query, &q.NormalizedQuery, &q.DurationMS, &q.TableName, &q.Operation, &q.Rows); err != nil {
			return nil, fmt.Errorf("failed to scan query: %w", err)
		}
		queries = append(queries, q)
	}

	return queries, nil
}

// GetExecutionDetail retrieves full execution details with logs and SQL analysis
func (s *Store) GetExecutionDetail(executionID int64) (*ExecutionDetail, error) {
	exec, err := s.GetExecution(executionID)
	if err != nil {
		return nil, err
	}
	if exec == nil {
		return nil, nil
	}

	req, err := s.GetRequest(exec.RequestID)
	if err != nil {
		return nil, err
	}

	logs, err := s.GetExecutionLogs(executionID)
	if err != nil {
		return nil, err
	}

	sqlQueries, err := s.GetSQLQueries(executionID)
	if err != nil {
		return nil, err
	}

	detail := &ExecutionDetail{
		Execution:  *exec,
		Request:    *req,
		Logs:       logs,
		SQLQueries: sqlQueries,
	}

	// Calculate SQL analysis
	if len(sqlQueries) > 0 {
		detail.SQLAnalysis = s.analyzeSQLQueries(sqlQueries)
	}

	return detail, nil
}

// analyzeSQLQueries performs SQL query analysis
func (s *Store) analyzeSQLQueries(queries []SQLQuery) *SQLAnalysis {
	if len(queries) == 0 {
		return &SQLAnalysis{
			TablesAccessed: make(map[string]int),
		}
	}

	analysis := &SQLAnalysis{
		TotalQueries:   len(queries),
		TablesAccessed: make(map[string]int),
	}

	// Calculate totals
	for _, q := range queries {
		analysis.TotalDuration += q.DurationMS
		if q.TableName != "" {
			analysis.TablesAccessed[q.TableName]++
		}
	}
	analysis.AvgDuration = analysis.TotalDuration / float64(len(queries))

	// Group by normalized query
	queryGroups := make(map[string]*QueryGroupResult)
	for _, q := range queries {
		if _, exists := queryGroups[q.NormalizedQuery]; !exists {
			queryGroups[q.NormalizedQuery] = &QueryGroupResult{
				NormalizedQuery: q.NormalizedQuery,
				Example:         q.Query,
			}
		}
		group := queryGroups[q.NormalizedQuery]
		group.Count++
		group.AvgDuration = (group.AvgDuration*float64(group.Count-1) + q.DurationMS) / float64(group.Count)
	}

	// Count unique queries
	analysis.UniqueQueries = len(queryGroups)

	// Detect N+1 issues (queries executed more than 5 times)
	for _, group := range queryGroups {
		if group.Count > 5 {
			analysis.NPlusOneIssues = append(analysis.NPlusOneIssues, *group)
		}
	}

	return analysis
}
