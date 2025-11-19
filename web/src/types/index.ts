export interface Container {
  ID: string;
  Name: string;
  Image?: string;
  Ports?: Port[];
  Project?: string; // Docker Compose project name
  Service?: string; // Docker Compose service name
}

export interface Port {
  publicPort?: number;
  privatePort?: number;
}

export interface LogMessage {
  containerId: string;
  timestamp: string;
  entry?: {
    timestamp?: string;
    level?: string;
    file?: string;
    message?: string;
    raw?: string;
    fields?: Record<string, any>;
  };
}

export interface SQLQuery {
  query: string;
  duration: number;
  table: string;
  operation: string;
  rows: number;
  variables: Record<string, string>;
  normalized: string;
}

export interface SQLAnalysis {
  totalQueries: number;
  uniqueQueries: number;
  avgDuration: number;
  totalDuration: number;
  slowestQueries: SQLQuery[];
  frequentQueries: FrequentQuery[];
  nPlusOne: FrequentQuery[];
  tables: TableInfo[];
}

export interface FrequentQuery {
  normalized: string;
  count: number;
  example: SQLQuery;
  avgDuration: number;
}

export interface TableInfo {
  table: string;
  count: number;
}

export interface ExplainData {
  planSource: string;
  planQuery: string;
  error: string | null;
  metadata: ExplainMetadata | null;
}

export interface ExplainMetadata {
  type?: string;
  operation?: string;
  table?: string;
  requestId?: string;
  operationName?: string;
}

export interface RecentRequest {
  requestId: string;
  path: string;
  operations: string[];
  method: string;
  statusCode: number | null;
  latency: number | null;
  timestamp: string;
}

export interface RetentionSettings {
  type: "count" | "time";
  value: number;
}

export interface WebSocketMessage {
  type: "log" | "logs" | "logs_initial" | "containers" | "filter";
  data: any;
}

export interface FilterData {
  selectedContainers: string[];
  selectedLevels: string[];
  searchQuery: string;
  traceFilters: { type: string; value: string }[];
}

export interface ContainerData {
  containers: Container[];
  portToServerMap?: Record<number, string>;
  logCounts?: Record<string, number>;
  retentions?: Record<string, RetentionSettings>;
}

export interface Server {
  id: number;
  name: string;
  url: string;
  bearerToken?: string;
  devId?: string;
  experimentalMode?: string;
  defaultDatabaseId?: number | null;
  defaultDatabase?: DatabaseURL | null;
  createdAt: string;
  updatedAt: string;
}

export interface DatabaseURL {
  id: number;
  name: string;
  connectionString: string;
  databaseType: string;
  createdAt: string;
  updatedAt: string;
}

export interface CreateServerResponse {
  id: number;
}

export interface CreateDatabaseURLResponse {
  id: number;
}

export interface ExplainResponse {
  queryPlan?: any[];
  query?: string;
  error?: string;
}

export interface SaveTraceResponse {
  id: number;
}

export interface RetentionResponse {
  retentionType: "count" | "time";
  retentionValue: number;
}

export interface SampleQuery {
  id: number;
  name: string;
  serverId?: number | null;
  server?: Server | null;
  requestData: string;
  displayName?: string;
  createdAt: string;
  updatedAt: string;
}

export interface ExecutedRequest {
  id: number;
  sampleId?: number | null;
  serverId?: number | null;
  server?: Server | null;
  requestIdHeader: string;
  requestBody?: string;
  statusCode: number;
  durationMs: number;
  responseBody?: string;
  responseHeaders?: string;
  error?: string;
  isSync: boolean;
  displayName?: string;
  executedAt: string;
  createdAt: string;
  updatedAt: string;
}

export interface ExecutionLog {
  id: number;
  executionId: number;
  containerId: string;
  timestamp: string;
  level: string;
  message: string;
  rawLog: string;
  fields: string;
  createdAt: string;
  updatedAt: string;
}

export interface ExecutionSQLQuery {
  id: number;
  executionId: number;
  query: string;
  normalizedQuery: string;
  queryHash?: string;
  durationMs: number;
  tableName: string;
  operation: string;
  rows: number;
  variables?: string;
  graphqlOperation?: string;
  explainPlan?: string;
  createdAt: string;
  updatedAt: string;
}

export interface ExecutionDetail {
  execution: ExecutedRequest;
  request?: SampleQuery | null;
  logs: ExecutionLog[];
  sqlQueries: ExecutionSQLQuery[];
  sqlAnalysis?: any;
  indexAnalysis?: any;
  server?: Server | null;
  displayName: string;
}

export interface ExecuteResponse {
  executionId: number;
}

export interface AllExecutionsResponse {
  executions: ExecutedRequest[];
  total: number;
  limit: number;
  offset: number;
}

export interface ExecutionReference {
  id: number;
  displayName: string;
  requestIdHeader: string;
  durationMs: number;
  executedAt: string;
  statusCode: number;
}

export interface SQLQueryDetail {
  queryHash: string;
  query: string;
  normalizedQuery: string;
  operation: string;
  tableName: string;
  totalExecutions: number;
  avgDuration: number;
  minDuration: number;
  maxDuration: number;
  explainPlan?: string;
  variables?: string;
  indexAnalysis?: any;
  relatedExecutions: ExecutionReference[];
}

export interface PlanNodeType {
  "Node Type": string;
  "Relation Name"?: string;
  "Startup Cost"?: number;
  "Total Cost"?: number;
  "Plan Rows"?: number;
  "Plan Width"?: number;
  "Actual Rows"?: number;
  "Actual Loops"?: number;
  "Index Name"?: string;
  "Scan Direction"?: string;
  Filter?: string;
  Plans?: PlanNodeType[];
  "Actual Startup Time"?: number;
  "Actual Total Time"?: number;
  Alias?: string;
  "Async Capable"?: boolean;
  "Index Cond"?: string;
  [key: string]: any;
}

export interface Planning {
  "Local Dirtied Blocks": number;
  "Local Hit Blocks": number;
  "Local I/O Read Time": number;
  "Local I/O Write Time": number;
  "Local Read Blocks": number;
  "Local Written Blocks": number;
}
export interface Execution {
  "Shared Dirtied Blocks": number;
  "Shared Hit Blocks": number;
  "Shared I/O Read Time": number;
  "Shared I/O Write Time": number;
  "Shared Read Blocks": number;
  "Shared Written Blocks": number;
}
export interface Triggers {
  "Temp I/O Read Time": number;
  "Temp I/O Write Time": number;
  "Temp Read Blocks": number;
  "Temp Written Blocks": number;
}
export interface Plan {
  Plan: PlanNodeType;
  "Planning Time"?: number;
  "Execution Time"?: number;
  "Query Identifier"?: string;
  Planning?: Planning;
  Execution?: Execution;
  Triggers?: Triggers;
  [key: string]: any;
}
