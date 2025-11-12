export interface Container {
  ID: string;
  Name: string;
  Ports?: Port[];
}

export interface Port {
  publicPort?: number;
  privatePort?: number;
}

export interface LogEntry {
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
