package store

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"docker-log-parser/pkg/logs"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// Store manages the SQLite database for request tracking
type Store struct {
	db *gorm.DB
}

// DatabaseURL represents a database connection configuration for EXPLAIN queries
type DatabaseURL struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	Name             string    `gorm:"not null" json:"name"`
	ConnectionString string    `gorm:"not null;column:connection_string" json:"connectionString"`
	DatabaseType     string    `gorm:"not null;column:database_type;default:postgresql" json:"databaseType"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// Server represents a server configuration with URL and authentication
type Server struct {
	ID                uint         `gorm:"primaryKey" json:"id"`
	Name              string       `gorm:"not null" json:"name"`
	URL               string       `gorm:"not null" json:"url"`
	BearerToken       string       `gorm:"column:bearer_token" json:"bearerToken,omitempty"`
	DevID             string       `gorm:"column:dev_id" json:"devId,omitempty"`
	DefaultDatabaseID *uint        `gorm:"column:default_database_id;index" json:"defaultDatabaseId,omitempty"`
	DefaultDatabase   *DatabaseURL `gorm:"foreignKey:DefaultDatabaseID" json:"defaultDatabase,omitempty"`
	CreatedAt         time.Time    `json:"createdAt"`
	UpdatedAt         time.Time    `json:"updatedAt"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

// Request represents a saved GraphQL/API request template (sample query)
type Request struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	ServerID    *uint     `gorm:"column:server_id;index" json:"serverId,omitempty"`
	Server      *Server   `gorm:"foreignKey:ServerID" json:"server,omitempty"`
	RequestData string    `gorm:"not null;column:request_data" json:"requestData"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for the Request model
func (Request) TableName() string {
	return "sample_queries"
}

// Execution represents a single execution of a request (executed request)
type Execution struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	RequestID       uint      `gorm:"not null;column:request_id;index" json:"requestId"`
	ServerID        *uint     `gorm:"column:server_id;index" json:"serverId,omitempty"`
	Server          *Server   `gorm:"foreignKey:ServerID" json:"server,omitempty"`
	RequestIDHeader string    `gorm:"not null;column:request_id_header" json:"requestIdHeader"`
	StatusCode      int       `gorm:"column:status_code" json:"statusCode"`
	DurationMS      int64     `gorm:"column:duration_ms" json:"durationMs"`
	ResponseBody    string    `gorm:"column:response_body" json:"responseBody,omitempty"`
	ResponseHeaders string    `gorm:"column:response_headers" json:"responseHeaders,omitempty"`
	Error           string    `json:"error,omitempty"`
	ExecutedAt      time.Time `gorm:"not null;column:executed_at;index" json:"executedAt"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for the Execution model
func (Execution) TableName() string {
	return "executed_requests"
}

// ExecutionLog represents a log entry from an execution
type ExecutionLog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ExecutionID uint      `gorm:"not null;column:execution_id;index" json:"executionId"`
	ContainerID string    `gorm:"not null;column:container_id" json:"containerId"`
	Timestamp   time.Time `gorm:"not null" json:"timestamp"`
	Level       string    `json:"level"`
	Message     string    `json:"message"`
	RawLog      string    `gorm:"column:raw_log" json:"rawLog"`
	Fields      string    `json:"fields"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// SQLQuery represents a SQL query extracted from logs
type SQLQuery struct {
	ID               uint    `gorm:"primaryKey" json:"id"`
	ExecutionID      uint    `gorm:"not null;column:execution_id;index" json:"executionId"`
	Query            string  `gorm:"not null" json:"query"`
	NormalizedQuery  string  `gorm:"not null;column:normalized_query" json:"normalizedQuery"`
	QueryHash        string  `gorm:"column:query_hash;index" json:"queryHash,omitempty"`
	DurationMS       float64 `gorm:"column:duration_ms" json:"durationMs"`
	TableName        string  `gorm:"column:table_name" json:"tableName"`
	Operation        string  `json:"operation"`
	Rows             int     `json:"rows"`
	Variables        string  `gorm:"column:variables" json:"variables,omitempty"` // Stored db.vars for EXPLAIN
	GraphQLOperation string  `gorm:"column:gql_operation" json:"graphqlOperation,omitempty"`
	ExplainPlan      string  `gorm:"column:explain_plan" json:"explainPlan,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// ExecutionDetail includes execution with related logs and SQL analysis
type ExecutionDetail struct {
	Execution   Execution      `json:"execution"`
	Request     *Request       `json:"request,omitempty"`
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
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Get underlying SQL DB for migrations
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get SQL DB: %w", err)
	}

	// Enable foreign keys
	_, err = sqlDB.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Run migrations using goose
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return nil, fmt.Errorf("failed to set goose dialect: %w", err)
	}

	if err := goose.Up(sqlDB, "migrations"); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// CreateRequest creates a new request template
func (s *Store) CreateRequest(req *Request) (int64, error) {
	result := s.db.Create(req)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to create request: %w", result.Error)
	}
	return int64(req.ID), nil
}

// GetRequest retrieves a request by ID
func (s *Store) GetRequest(id int64) (*Request, error) {
	var req Request
	result := s.db.Preload("Server").First(&req, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get request: %w", result.Error)
	}
	return &req, nil
}

// ListRequests retrieves all requests
func (s *Store) ListRequests() ([]Request, error) {
	var requests []Request
	result := s.db.Preload("Server").Order("created_at DESC").Find(&requests)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list requests: %w", result.Error)
	}
	return requests, nil
}

// DeleteRequest deletes a request and all its executions
func (s *Store) DeleteRequest(id int64) error {
	result := s.db.Delete(&Request{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete request: %w", result.Error)
	}
	return nil
}

// CreateServer creates a new server configuration
func (s *Store) CreateServer(server *Server) (int64, error) {
	result := s.db.Create(server)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to create server: %w", result.Error)
	}
	return int64(server.ID), nil
}

// GetServer retrieves a server by ID
func (s *Store) GetServer(id int64) (*Server, error) {
	var server Server
	result := s.db.Preload("DefaultDatabase").First(&server, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get server: %w", result.Error)
	}
	return &server, nil
}

// ListServers retrieves all servers
func (s *Store) ListServers() ([]Server, error) {
	var servers []Server
	result := s.db.Preload("DefaultDatabase").Order("name").Find(&servers)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list servers: %w", result.Error)
	}
	return servers, nil
}

// UpdateServer updates a server configuration
func (s *Store) UpdateServer(server *Server) error {
	result := s.db.Save(server)
	if result.Error != nil {
		return fmt.Errorf("failed to update server: %w", result.Error)
	}
	return nil
}

// DeleteServer deletes a server
func (s *Store) DeleteServer(id int64) error {
	result := s.db.Delete(&Server{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete server: %w", result.Error)
	}
	return nil
}

// CreateDatabaseURL creates a new database URL configuration
func (s *Store) CreateDatabaseURL(dbURL *DatabaseURL) (int64, error) {
	result := s.db.Create(dbURL)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to create database URL: %w", result.Error)
	}
	return int64(dbURL.ID), nil
}

// GetDatabaseURL retrieves a database URL by ID
func (s *Store) GetDatabaseURL(id int64) (*DatabaseURL, error) {
	var dbURL DatabaseURL
	result := s.db.First(&dbURL, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get database URL: %w", result.Error)
	}
	return &dbURL, nil
}

// ListDatabaseURLs retrieves all database URLs
func (s *Store) ListDatabaseURLs() ([]DatabaseURL, error) {
	var dbURLs []DatabaseURL
	result := s.db.Order("name").Find(&dbURLs)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list database URLs: %w", result.Error)
	}
	return dbURLs, nil
}

// UpdateDatabaseURL updates a database URL configuration
func (s *Store) UpdateDatabaseURL(dbURL *DatabaseURL) error {
	result := s.db.Save(dbURL)
	if result.Error != nil {
		return fmt.Errorf("failed to update database URL: %w", result.Error)
	}
	return nil
}

// DeleteDatabaseURL deletes a database URL
func (s *Store) DeleteDatabaseURL(id int64) error {
	result := s.db.Delete(&DatabaseURL{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete database URL: %w", result.Error)
	}
	return nil
}


// CreateExecution creates a new execution record
func (s *Store) CreateExecution(exec *Execution) (int64, error) {
	result := s.db.Create(exec)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to create execution: %w", result.Error)
	}
	return int64(exec.ID), nil
}

// GetExecution retrieves an execution by ID
func (s *Store) GetExecution(id int64) (*Execution, error) {
	var exec Execution
	result := s.db.First(&exec, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get execution: %w", result.Error)
	}
	return &exec, nil
}

// ListExecutions retrieves all executions for a request
func (s *Store) ListExecutions(requestID int64) ([]Execution, error) {
	var executions []Execution
	result := s.db.Where("request_id = ?", requestID).Order("executed_at DESC").Find(&executions)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list executions: %w", result.Error)
	}
	return executions, nil
}

// ListAllExecutions retrieves all executions across all requests
func (s *Store) ListAllExecutions() ([]Execution, error) {
	var executions []Execution
	result := s.db.Preload("Server").Order("executed_at DESC").Limit(100).Find(&executions)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list all executions: %w", result.Error)
	}
	return executions, nil
}

// SaveExecutionLogs saves log entries for an execution
func (s *Store) SaveExecutionLogs(executionID int64, logMessages []logs.LogMessage) error {
	var execLogs []ExecutionLog

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

		execLogs = append(execLogs, ExecutionLog{
			ExecutionID: uint(executionID),
			ContainerID: msg.ContainerID,
			Timestamp:   msg.Timestamp,
			Level:       level,
			Message:     message,
			RawLog:      rawLog,
			Fields:      string(fieldsJSON),
		})
	}

	if len(execLogs) > 0 {
		result := s.db.Create(&execLogs)
		if result.Error != nil {
			return fmt.Errorf("failed to insert logs: %w", result.Error)
		}
	}

	return nil
}

// GetExecutionLogs retrieves logs for an execution
func (s *Store) GetExecutionLogs(executionID int64) ([]ExecutionLog, error) {
	var logs []ExecutionLog
	result := s.db.Where("execution_id = ?", executionID).Order("timestamp").Find(&logs)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get execution logs: %w", result.Error)
	}
	return logs, nil
}

// SaveSQLQueries saves SQL queries for an execution
func (s *Store) SaveSQLQueries(executionID int64, queries []SQLQuery) error {
	if len(queries) == 0 {
		return nil
	}

	// Set the execution ID for all queries
	for i := range queries {
		queries[i].ExecutionID = uint(executionID)
	}

	result := s.db.Create(&queries)
	if result.Error != nil {
		return fmt.Errorf("failed to insert queries: %w", result.Error)
	}

	return nil
}

// UpdateQueryExplainPlan updates the explain plan for a query by its hash
func (s *Store) UpdateQueryExplainPlan(executionID int64, queryHash string, explainPlan string) error {
	result := s.db.Model(&SQLQuery{}).
		Where("execution_id = ? AND query_hash = ?", executionID, queryHash).
		Update("explain_plan", explainPlan)
	
	if result.Error != nil {
		return fmt.Errorf("failed to update explain plan: %w", result.Error)
	}
	
	return nil
}

// GetSQLQueries retrieves SQL queries for an execution
func (s *Store) GetSQLQueries(executionID int64) ([]SQLQuery, error) {
	var queries []SQLQuery
	result := s.db.Where("execution_id = ?", executionID).Order("id").Find(&queries)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get SQL queries: %w", result.Error)
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

	req, err := s.GetRequest(int64(exec.RequestID))
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
		Request:    req,
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

// ComputeQueryHash computes a SHA256 hash of the normalized query
func ComputeQueryHash(normalizedQuery string) string {
	hash := sha256.Sum256([]byte(normalizedQuery))
	return hex.EncodeToString(hash[:])
}
