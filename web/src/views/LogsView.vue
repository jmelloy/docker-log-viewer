<template>
  <div class="app-container">
    <app-header activePage="viewer">
      <div class="header-controls">
        <div class="trace-filter-display" v-if="hasTraceFilters">
          <span
            v-for="([key, value], index) in Array.from(traceFilters.entries())"
            :key="key"
            class="trace-filter-badge"
          >
            <span class="filter-key">{{ key }}</span
            >=<span class="filter-value">{{ value }}</span>
            <button @click="removeTraceFilter(key)" class="filter-remove" title="Remove filter">×</button>
          </span>
          <button @click="saveTrace" class="btn-star" title="Save trace to request manager">⭐</button>
          <button @click="clearTraceFilters" class="clear-btn" title="Clear all filters">✕</button>
        </div>
      </div>
    </app-header>

    <div class="main-layout">
      <aside class="sidebar">
        <!-- Search Box -->
        <div class="section">
          <div class="search-box">
            <input
              type="text"
              v-model="searchQuery"
              placeholder="Search logs... (use quotes for exact phrases)"
              title="Search logs. Use quotes for exact phrase matching, e.g., error &quot;database connection&quot; failed"
            />
            <button
              @click="
                searchQuery = '';
                updateURL();
              "
              class="clear-btn"
              title="Clear search"
            >
              ✕
            </button>
          </div>
        </div>

        <!-- SQL Query Analyzer Section -->
        <div v-if="showAnalyzer" class="section analyzer-section-container">
          <div class="section-header">
            <h3>SQL Query Analyzer</h3>
            <button @click="showAnalyzer = false" class="close-analyzer-btn">✕</button>
          </div>
          <div v-if="sqlAnalysis" class="analyzer-content-compact">
            <div class="analyzer-subsection">
              <h4>Overview</h4>
              <div class="stats-grid-compact">
                <div class="stat-item">
                  <span class="stat-label">Total Queries</span>
                  <span class="stat-value">{{ sqlAnalysis.totalQueries }}</span>
                </div>
                <div class="stat-item">
                  <span class="stat-label">Unique</span>
                  <span class="stat-value">{{ sqlAnalysis.uniqueQueries }}</span>
                </div>
                <div class="stat-item">
                  <span class="stat-label">Avg Duration</span>
                  <span class="stat-value">{{ sqlAnalysis.avgDuration.toFixed(2) }}ms</span>
                </div>
                <div class="stat-item">
                  <span class="stat-label">Total Duration</span>
                  <span class="stat-value">{{ sqlAnalysis.totalDuration.toFixed(2) }}ms</span>
                </div>
              </div>
            </div>

            <div class="analyzer-subsection">
              <h4>Slowest Queries</h4>
              <div class="query-list-compact">
                <div v-if="sqlAnalysis.slowestQueries.length === 0" class="query-item-compact">No SQL queries</div>
                <div
                  v-for="(q, index) in sqlAnalysis.slowestQueries.slice(0, 3)"
                  :key="index"
                  class="query-item-compact"
                >
                  <div class="query-header-compact">
                    <span class="query-duration" :class="{ 'query-slow': q.duration > 10 }"
                      >{{ q.duration.toFixed(2) }}ms</span
                    >
                    <span class="query-meta-inline">{{ q.table }} · {{ q.operation }}</span>
                  </div>
                  <div class="query-text-compact">
                    {{ q.query.substring(0, 60) }}{{ q.query.length > 60 ? "..." : "" }}
                  </div>
                  <button
                    class="btn-explain-compact"
                    @click="
                      runExplain(q.query, q.variables, { table: q.table, operation: q.operation, type: 'slowest' })
                    "
                  >
                    EXPLAIN
                  </button>
                </div>
              </div>
            </div>

            <div class="analyzer-subsection">
              <h4>Most Frequent</h4>
              <div class="query-list-compact">
                <div
                  v-for="(item, index) in sqlAnalysis.frequentQueries.slice(0, 3)"
                  :key="index"
                  class="query-item-compact"
                >
                  <div class="query-header-compact">
                    <span class="query-count">{{ item.count }}x</span>
                    <span class="query-meta-inline"
                      >{{ item.example.table }} · {{ item.avgDuration.toFixed(2) }}ms</span
                    >
                  </div>
                  <div class="query-text-compact">
                    {{ item.example.query.substring(0, 60) }}{{ item.example.query.length > 60 ? "..." : "" }}
                  </div>
                  <button
                    class="btn-explain-compact"
                    @click="
                      runExplain(item.example.query, item.example.variables, {
                        table: item.example.table,
                        operation: item.example.operation,
                        type: 'frequent',
                      })
                    "
                  >
                    EXPLAIN
                  </button>
                </div>
              </div>
            </div>

            <div v-if="sqlAnalysis.nPlusOne.length > 0" class="analyzer-subsection">
              <h4>N+1 Issues ({{ sqlAnalysis.nPlusOne.length }})</h4>
              <div class="query-list-compact">
                <div
                  v-for="(item, index) in sqlAnalysis.nPlusOne.slice(0, 2)"
                  :key="index"
                  class="query-item-compact nplusone"
                >
                  <div class="query-header-compact">
                    <span class="query-count">{{ item.count }}x</span>
                    <span class="query-meta-inline">{{ item.example.table }}</span>
                  </div>
                  <div class="query-text-compact">{{ item.example.query.substring(0, 60) }}...</div>
                </div>
              </div>
            </div>

            <div class="analyzer-subsection">
              <h4>Tables</h4>
              <div class="table-list-compact">
                <span v-for="(item, index) in sqlAnalysis.tables" :key="index" class="table-badge-compact">
                  {{ item.table }}<span class="table-count">({{ item.count }})</span>
                </span>
              </div>
            </div>
          </div>
        </div>

        <div class="section">
          <h3>Containers</h3>
          <div class="container-list">
            <div v-for="project in projectNames" :key="project" class="project-section">
              <div class="project-header" @click="toggleProjectCollapse(project)">
                <span class="disclosure-arrow" :class="{ collapsed: isProjectCollapsed(project) }">▼</span>
                <div
                  class="checkbox"
                  :class="{ checked: isProjectSelected(project), indeterminate: isProjectIndeterminate(project) }"
                  @click.stop="toggleProject(project)"
                ></div>
                <span class="project-name">{{ project }}</span>
                <span class="project-count">({{ containersByProject[project].length }})</span>
              </div>
              <div class="project-containers" :class="{ collapsed: isProjectCollapsed(project) }">
                <div
                  v-for="container in containersByProject[project]"
                  :key="container.Name"
                  class="container-item"
                  :class="{ selected: isContainerSelected(container.Name) }"
                >
                  <div @click="toggleContainer(container.Name)" style="display: flex; flex: 1; align-items: center">
                    <div class="checkbox" :class="{ checked: isContainerSelected(container.Name) }"></div>
                    <div class="container-info">
                      <div class="container-name">{{ getShortContainerName(container.ID) }}</div>
                      <div class="container-id">{{ container.ID.substring(0, 12) }}</div>
                    </div>
                  </div>
                  <div
                    class="log-count-badge"
                    @click.stop="openRetentionModal(container.Name)"
                    :title="getRetentionTooltip(container.Name)"
                  >
                    {{ logCounts[container.Name] || 0 }}
                    <span v-if="retentions[container.Name]" class="retention-indicator">⏱</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div v-if="recentRequests.length > 0" class="section">
          <h3>Recent Requests</h3>
          <div class="recent-requests-list">
            <div
              v-for="req in recentRequests"
              :key="req.requestId"
              class="recent-request-item"
              @click="setTraceFilter('request_id', req.requestId, null)"
              title="Click to filter by this request"
            >
              <div class="request-header-line">
                <span class="request-method">{{ req.method }}</span>
                <span class="request-path">{{ req.path }}</span>
                <span
                  v-if="req.latency"
                  class="request-latency"
                  :class="{ 'latency-slow': req.latency && req.latency > 1000 }"
                  >{{ req.latency }}ms</span
                >
                <span
                  v-if="req.statusCode"
                  class="request-status"
                  :class="{
                    'status-success': req.statusCode >= 200 && req.statusCode < 300,
                    'status-error': req.statusCode >= 400,
                  }"
                  >{{ req.statusCode }}</span
                >
              </div>
              <div v-if="req.operations.length > 0" class="request-operations">
                <span v-for="op in req.operations" :key="op" class="operation-badge">{{ op }}</span>
              </div>
              <div class="request-footer">
                <span class="request-timestamp">{{ req.timestamp }}</span>
                <span class="request-id">{{ req.requestId.substring(0, 8) }}...</span>
              </div>
            </div>
          </div>
        </div>

        <div class="section">
          <h3>Log Levels</h3>
          <div class="level-filters">
            <label class="level-filter">
              <input type="checkbox" value="TRC" :checked="isLevelSelected('TRC')" @change="toggleLevel('TRC')" />
              <span class="level-badge level-trc">TRC</span>
            </label>
            <label class="level-filter">
              <input type="checkbox" value="DBG" :checked="isLevelSelected('DBG')" @change="toggleLevel('DBG')" />
              <span class="level-badge level-dbg">DBG</span>
            </label>
            <label class="level-filter">
              <input type="checkbox" value="INF" :checked="isLevelSelected('INF')" @change="toggleLevel('INF')" />
              <span class="level-badge level-inf">INF</span>
            </label>
            <label class="level-filter">
              <input type="checkbox" value="WRN" :checked="isLevelSelected('WRN')" @change="toggleLevel('WRN')" />
              <span class="level-badge level-wrn">WRN</span>
            </label>
            <label class="level-filter">
              <input type="checkbox" value="ERR" :checked="isLevelSelected('ERR')" @change="toggleLevel('ERR')" />
              <span class="level-badge level-err">ERR</span>
            </label>
            <label class="level-filter">
              <input type="checkbox" value="NONE" :checked="isLevelSelected('NONE')" @change="toggleLevel('NONE')" />
              <span class="level-badge level-none">NONE</span>
            </label>
          </div>
        </div>

        <div class="sidebar-footer">
          <div class="status">
            <span :style="{ color: statusColor }">{{ statusText }}</span>
            <span>{{ logCountText }}</span>
          </div>
        </div>
      </aside>

      <main class="log-viewer">
        <div ref="logsContainer" class="logs">
          <div v-for="(log, index) in filteredLogs" :key="index" class="log-line" @click="openLogDetails(log)">
            <span class="log-container">{{ getShortContainerName(log.containerId) }}</span>
            <span v-if="log.entry?.timestamp" class="log-timestamp">{{ formatTimestamp(log.entry.timestamp) }}</span>
            <span v-if="log.entry?.level" class="log-level" :class="log.entry.level">{{ log.entry.level }}</span>
            <span v-if="log.entry?.file" class="log-file">{{ log.entry.file }}</span>
            <span v-if="log.entry?.message" class="log-message">{{ log.entry.message }}</span>
            <span v-for="([key, value], idx) in Object.entries(log.entry?.fields || {})" :key="idx" class="log-field">
              <span class="log-field-key">{{ key }}</span
              >=<span
                :class="{ 'log-field-value': !isJsonField(value) }"
                @click.stop="!isJsonField(value) && setTraceFilter(key, value, $event)"
                >{{ formatFieldValue(key, value) }}</span
              >
            </span>
          </div>
        </div>
      </main>
    </div>
  </div>

  <!-- Log Details Modal -->
  <div v-if="showLogModal" class="modal" @click="showLogModal = false">
    <div class="modal-content" @click.stop>
      <div class="modal-header">
        <h3>Log Details</h3>
        <button @click="showLogModal = false">✕</button>
      </div>
      <div v-if="selectedLog" class="modal-body">
        <div class="modal-section">
          <h4>Raw Log</h4>
          <pre
            style="white-space: pre-wrap"
            v-html="
              convertAnsiToHtml(
                selectedLog.entry?.raw.replaceAll('\\n', '\n').replaceAll('\\t', '    ') || 'No raw log available'
              )
            "
          ></pre>
        </div>
        <div class="modal-section">
          <h4>Parsed Fields</h4>
          <div>
            <!-- Timestamp and Level on same line -->
            <div
              v-if="selectedLog.entry?.timestamp || selectedLog.entry?.level"
              class="parsed-field"
              style="display: flex; gap: 1rem; align-items: center"
            >
              <div v-if="selectedLog.entry?.timestamp" style="display: flex; gap: 0.5rem">
                <div class="parsed-field-key">Timestamp</div>
                <div class="parsed-field-value">{{ selectedLog.entry.timestamp }}</div>
              </div>
              <div v-if="selectedLog.entry?.level" style="display: flex; gap: 0.5rem">
                <div class="parsed-field-key">Level</div>
                <div class="parsed-field-value">{{ selectedLog.entry.level }}</div>
              </div>
              <div v-if="selectedLog.entry?.fields?.request_id" style="display: flex; gap: 0.5rem">
                <div class="parsed-field-key">request_id</div>
                <div class="parsed-field-value">{{ selectedLog.entry.fields.request_id }}</div>
              </div>
            </div>

            <!-- request_id, trace_id, and gql.operationName on same line -->
            <div
              v-if="selectedLog.entry?.fields?.['trace_id'] || selectedLog.entry?.fields?.['gql.operationName']"
              class="parsed-field"
              style="display: flex; gap: 1rem; align-items: center; flex-wrap: wrap"
            >
              <div v-if="selectedLog.entry?.fields?.['gql.operationName']" style="display: flex; gap: 0.5rem">
                <div class="parsed-field-key">gql.operationName</div>
                <div class="parsed-field-value">{{ selectedLog.entry.fields["gql.operationName"] }}</div>
              </div>
              <div v-if="selectedLog.entry?.fields?.['trace_id']" style="display: flex; gap: 0.5rem">
                <div class="parsed-field-key">trace_id</div>
                <div class="parsed-field-value">{{ selectedLog.entry.fields["trace_id"] }}</div>
              </div>
            </div>
            <!-- Message on its own line -->
            <div v-if="selectedLog.entry?.message" class="parsed-field">
              <div class="parsed-field-key">Message</div>
              <div v-if="isSQLMessage(selectedLog.entry.message)" class="parsed-field-value">
                <pre ref="sqlMessageRef" class="sql-query-text" style="white-space: pre-wrap; margin: 0">{{
                  formatMessage(selectedLog.entry.message)
                }}</pre>
              </div>
              <div v-else class="parsed-field-value">{{ selectedLog.entry.message }}</div>
            </div>
            <!-- File -->
            <div v-if="selectedLog.entry?.file" class="parsed-field">
              <div class="parsed-field-key">File</div>
              <div class="parsed-field-value">{{ selectedLog.entry.file }}</div>
            </div>

            <div
              v-if="
                selectedLog.entry?.fields?.['db.rows'] ||
                selectedLog.entry?.fields?.['db.table'] ||
                selectedLog.entry?.fields?.['duration']
              "
              class="parsed-field"
              style="display: flex; gap: 1rem; align-items: center; flex-wrap: wrap"
            >
              <div v-if="selectedLog.entry?.fields?.['db.rows']" style="display: flex; gap: 0.5rem">
                <div class="parsed-field-key">db.rows</div>
                <div class="parsed-field-value">{{ selectedLog.entry.fields["db.rows"] }}</div>
              </div>
              <div v-if="selectedLog.entry?.fields?.['db.table']" style="display: flex; gap: 0.5rem">
                <div class="parsed-field-key">db.table</div>
                <div class="parsed-field-value">{{ selectedLog.entry.fields["db.table"] }}</div>
              </div>
              <div v-if="selectedLog.entry?.fields?.['duration']" style="display: flex; gap: 0.5rem">
                <div class="parsed-field-key">duration</div>
                <div class="parsed-field-value">{{ selectedLog.entry.fields["duration"] }}</div>
              </div>
            </div>

            <!-- Rest of the fields -->
            <div
              v-for="([key, value], idx) in getRemainingFields(selectedLog.entry?.fields)"
              :key="idx"
              class="parsed-field"
            >
              <div class="parsed-field-key">{{ key }}</div>
              <div v-if="isJsonField(value)" class="parsed-field-value">
                <pre>{{ formatJsonField(value) }}</pre>
              </div>
              <div v-else class="parsed-field-value">{{ value }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>

  <!-- EXPLAIN Plan Side Panel -->
  <div v-if="showExplainModal" class="side-panel-overlay" @click="closeExplainPlanModal">
    <div class="side-panel" @click.stop>
      <div class="side-panel-header">
        <h3>SQL Query EXPLAIN Plan (PEV2)</h3>
        <div style="display: flex; gap: 0.5rem">
          <button
            v-if="!explainData.error"
            @click="shareExplainPlan"
            class="btn-secondary"
            style="padding: 0.5rem 1rem"
          >
            📋 Share
          </button>
          <button @click="closeExplainPlanModal">✕</button>
        </div>
      </div>
      <div class="side-panel-body">
        <div v-if="explainData.error" class="alert alert-danger" style="display: block; margin: 1rem">
          {{ explainData.error }}
        </div>
        <div v-if="!explainData.error" id="pev2App" class="d-flex flex-column" style="height: 100%">
          <pev2 :plan-source="explainData.planSource" :plan-query="explainData.planQuery"></pev2>
        </div>
      </div>
    </div>
  </div>

  <!-- Retention Modal -->
  <div v-if="showRetentionModal" class="modal" @click="showRetentionModal = false">
    <div class="modal-content" @click.stop style="max-width: 500px">
      <div class="modal-header">
        <h3>Log Retention - {{ retentionContainer }}</h3>
        <button @click="showRetentionModal = false">✕</button>
      </div>
      <div class="modal-body" style="padding: 1.5rem">
        <div style="margin-bottom: 1rem">
          <label style="display: block; margin-bottom: 0.5rem; font-weight: 500">Retention Type:</label>
          <select
            v-model="retentionForm.type"
            style="
              width: 100%;
              padding: 0.5rem;
              border: 1px solid #30363d;
              background: #0d1117;
              color: #c9d1d9;
              border-radius: 6px;
            "
          >
            <option value="count">By Count (number of logs)</option>
            <option value="time">By Time (seconds)</option>
          </select>
        </div>
        <div style="margin-bottom: 1rem">
          <label style="display: block; margin-bottom: 0.5rem; font-weight: 500">
            {{ retentionForm.type === "count" ? "Maximum Logs:" : "Maximum Age (seconds):" }}
          </label>
          <input
            v-model.number="retentionForm.value"
            type="number"
            min="1"
            style="
              width: 100%;
              padding: 0.5rem;
              border: 1px solid #30363d;
              background: #0d1117;
              color: #c9d1d9;
              border-radius: 6px;
            "
            :placeholder="retentionForm.type === 'count' ? 'e.g., 1000' : 'e.g., 3600 (1 hour)'"
          />
        </div>
        <div style="display: flex; gap: 0.5rem; justify-content: flex-end">
          <button
            @click="saveRetention"
            class="btn-primary"
            style="
              padding: 0.5rem 1rem;
              background: #238636;
              color: white;
              border: none;
              border-radius: 6px;
              cursor: pointer;
            "
          >
            Save
          </button>
          <button
            v-if="retentions[retentionContainer]"
            @click="deleteRetention"
            class="btn-danger"
            style="
              padding: 0.5rem 1rem;
              background: #da3633;
              color: white;
              border: none;
              border-radius: 6px;
              cursor: pointer;
            "
          >
            Remove
          </button>
          <button
            @click="showRetentionModal = false"
            style="
              padding: 0.5rem 1rem;
              background: #21262d;
              color: #c9d1d9;
              border: 1px solid #30363d;
              border-radius: 6px;
              cursor: pointer;
            "
          >
            Cancel
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import { API } from "@/utils/api";
import {
  convertAnsiToHtml as convertAnsiToHtmlUtil,
  formatSQL as formatSQLUtil,
  applySyntaxHighlighting,
} from "@/utils/ui-utils";
import { Plan } from "pev2";
import type {
  Container,
  LogMessage,
  SQLAnalysis,
  ExplainData,
  RecentRequest,
  RetentionSettings,
  WebSocketMessage,
  ContainerData,
  SQLQuery,
  FrequentQuery,
  SaveTraceResponse,
  ExplainResponse,
  RetentionResponse,
} from "@/types";

export default defineComponent({
  components: {
    pev2: Plan,
  },
  data() {
    // Load persisted container state from localStorage (by name, not ID)
    let selectedContainers = new Set();
    try {
      const saved = localStorage.getItem("selectedContainers");
      if (saved) {
        selectedContainers = new Set(JSON.parse(saved));
      }
    } catch (e) {
      console.warn("Failed to load container state:", e);
    }

    return {
      containers: [] as Container[],
      selectedContainers,
      logs: [] as LogMessage[],
      searchQuery: "",
      traceFilters: new Map(), // Map<fieldName, fieldValue>
      selectedLevels: new Set([
        "DBG",
        "DEBUG",
        "TRC",
        "TRACE",
        "INF",
        "INFO",
        "WRN",
        "WARN",
        "ERR",
        "ERROR",
        "FATAL",
        "NONE",
      ]),
      ws: null,
      wsConnected: false,
      showLogModal: false,
      showExplainModal: false,
      showAnalyzer: false,
      selectedLog: null as LogMessage | null,
      explainData: {
        planSource: "",
        planQuery: "",
        error: null,
        metadata: null,
      },
      sqlAnalysis: null as SQLAnalysis | null,
      collapsedProjects: new Set(),
      portToServerMap: {} as Record<number, string>, // Map of port -> connectionString
      recentRequests: [] as RecentRequest[], // Last 5 unique request IDs with paths
      logCounts: {} as Record<string, number>, // Map of container name -> log count
      retentions: {} as Record<string, RetentionSettings>, // Map of container name -> retention settings
      showRetentionModal: false,
      retentionContainer: null,
      retentionForm: {
        type: "count",
        value: 1000,
      },
    };
  },

  watch: {
    searchQuery() {
      this.sendFilterUpdate();
      this.updateURL();
    },
  },

  computed: {
    filteredLogs() {
      return this.logs;
    },

    containersByProject() {
      const groups = {};
      this.containers.forEach((container) => {
        // Use Project field from container if available, otherwise calculate it
        const project = container.Project || "";
        if (!groups[project]) {
          groups[project] = [];
        }
        groups[project].push(container);
      });
      // Sort containers within each project by name
      Object.keys(groups).forEach((project) => {
        groups[project].sort((a, b) => a.Name.localeCompare(b.Name));
      });
      return groups;
    },

    projectNames() {
      return Object.keys(this.containersByProject).sort();
    },

    statusText() {
      return this.wsConnected ? "Connected" : "Connecting...";
    },

    statusColor() {
      return this.wsConnected ? "#7ee787" : "#f85149";
    },

    hasTraceFilters() {
      return this.traceFilters.size > 0;
    },

    logCountText() {
      return `${this.filteredLogs.length} logs`;
    },
  },

  mounted() {
    this.parseURLParameters();
    this.init();
  },

  updated() {
    // Apply syntax highlighting after DOM updates
    this.$nextTick(() => {
      this.applySyntaxHighlighting();
    });
  },

  methods: {
    applySyntaxHighlighting() {
      applySyntaxHighlighting({ sqlSelector: ".query-text-compact, .sql-query-text" });
    },
    parseURLParameters() {
      const params = new URLSearchParams(window.location.search);

      // Parse search query parameter
      const queryParam = params.get("query");
      if (queryParam) {
        this.searchQuery = queryParam;
      }

      for (const [key, value] of params.entries()) {
        if (key !== "query") {
          this.traceFilters.set(key, value);
        }
      }
    },

    updateURL() {
      const params = new URLSearchParams();

      // Add search query
      if (this.searchQuery) {
        params.set("query", this.searchQuery);
      }

      // Add trace filters
      for (const [key, value] of this.traceFilters.entries()) {
        params.set(key, value);
      }

      const newURL = params.toString() ? `${window.location.pathname}?${params.toString()}` : window.location.pathname;

      // Update URL without reloading
      window.history.replaceState({}, "", newURL);
    },

    async init() {
      await this.loadContainers();
      this.connectWebSocket();
      // Initial logs will come via WebSocket after filter is sent
    },

    getLevelVariants(level) {
      const variants = {
        TRC: ["TRC", "TRACE"],
        DBG: ["DBG", "DEBUG"],
        INF: ["INF", "INFO"],
        WRN: ["WRN", "WARN"],
        ERR: ["ERR", "ERROR", "FATAL"],
        NONE: ["NONE"],
      };
      return variants[level] || [level];
    },

    toggleLevel(level) {
      const levelVariants = this.getLevelVariants(level);
      const hasAll = levelVariants.every((v) => this.selectedLevels.has(v));

      if (hasAll) {
        levelVariants.forEach((v) => this.selectedLevels.delete(v));
      } else {
        levelVariants.forEach((v) => this.selectedLevels.add(v));
      }
      this.sendFilterUpdate();
    },

    isLevelSelected(level) {
      const levelVariants = this.getLevelVariants(level);
      return levelVariants.every((v) => this.selectedLevels.has(v));
    },

    async loadContainers() {
      try {
        const data = await API.get<ContainerData | Container[]>("/api/containers");

        // Handle both old format (array) and new format (object with containers and portToServerMap)
        if (Array.isArray(data)) {
          this.containers = data;
        } else {
          this.containers = data.containers || [];
          this.portToServerMap = data.portToServerMap || {};
          this.logCounts = data.logCounts || {};
          this.retentions = data.retentions || {};
          console.log("Loaded port to server map:", this.portToServerMap);
        }

        // Get valid container names
        const validNames = new Set(this.containers.map((c) => c.Name));

        // Check if this is first load (no localStorage key exists)
        const isFirstLoad = !localStorage.getItem("selectedContainers");

        if (isFirstLoad && this.selectedContainers.size === 0) {
          // First load with no saved state, select all
          this.containers.forEach((c) => this.selectedContainers.add(c.Name));
          this.saveContainerState();
        } else {
          // Validate saved state - remove invalid entries (old IDs)
          const validSelections = new Set();
          for (const name of this.selectedContainers) {
            if (validNames.has(name)) {
              validSelections.add(name);
            }
          }

          // If everything was invalid, select all
          if (validSelections.size === 0) {
            this.containers.forEach((c) => validSelections.add(c.Name));
          }

          this.selectedContainers = validSelections;
          this.saveContainerState();
        }

        // Don't send filter update here - will be sent when WebSocket connects
      } catch (error) {
        console.error("Failed to load containers:", error);
      }
    },

    async loadInitialLogs() {
      try {
        const logs = await API.get<LogMessage[]>("/api/logs");
        this.logs = logs;
        this.$nextTick(() => this.scrollToBottom());
      } catch (error) {
        console.error("Failed to load logs:", error);
      }
    },

    connectWebSocket() {
      const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
      const wsUrl = `${protocol}//${window.location.host}/api/ws`;

      this.ws = new WebSocket(wsUrl);

      this.ws.onopen = () => {
        this.wsConnected = true;
        this.sendFilterUpdate();
      };

      this.ws.onmessage = (event) => {
        const message = JSON.parse(event.data) as WebSocketMessage;
        if (message.type === "log") {
          this.handleNewLog(message.data as LogMessage);
        } else if (message.type === "logs") {
          this.handleNewLogs(message.data as LogMessage[]);
        } else if (message.type === "logs_initial") {
          this.handleInitialLogs(message.data as LogMessage[]);
        } else if (message.type === "containers") {
          this.handleContainerUpdate(message.data as ContainerData);
        }
      };

      this.ws.onclose = () => {
        this.wsConnected = false;
        setTimeout(() => this.connectWebSocket(), 5000);
      };

      this.ws.onerror = (error) => {
        console.error("WebSocket error:", error);
      };
    },

    handleNewLog(log: LogMessage) {
      this.logs.push(log);
      if (this.logs.length > 100000) {
        this.logs = this.logs.slice(-50000);
      }
      this.updateRecentRequests(log);
      this.$nextTick(() => this.scrollToBottom());
    },

    handleNewLogs(logs: LogMessage[]) {
      // Handle batched logs from backend
      this.logs.push(...logs);
      if (this.logs.length > 100000) {
        this.logs = this.logs.slice(-50000);
      }
      logs.forEach((log) => this.updateRecentRequests(log));
      this.$nextTick(() => this.scrollToBottom());
    },

    handleInitialLogs(logs: LogMessage[]) {
      // Replace all logs with initial filtered set
      console.log(`Received ${logs.length} initial filtered logs`);
      this.logs = logs;
      logs.forEach((log) => this.updateRecentRequests(log));
      this.$nextTick(() => this.scrollToBottom());
    },

    updateRecentRequests(log: LogMessage) {
      const requestId = log.entry?.fields?.request_id;
      const path = log.entry?.fields?.path;
      const operationName =
        log.entry?.fields?.operation_name ||
        log.entry?.fields?.operationName ||
        log.entry?.fields?.["gql.operationName"];
      const method = log.entry?.fields?.method;
      const statusCode = log.entry?.fields?.status_code || log.entry?.fields?.statusCode;
      const latency =
        log.entry?.fields?.latency || log.entry?.fields?.duration || log.entry?.fields?.["request.duration"];
      const timestamp = log.entry?.timestamp || log.timestamp;

      if (!requestId || !path) return;

      if (latency && Number.parseFloat(latency) < 5) return;

      // Check if request ID already exists
      const existingIndex = this.recentRequests.findIndex((r) => r.requestId === requestId);

      if (existingIndex !== -1) {
        // Update existing request with new info
        const existing = this.recentRequests[existingIndex] as RecentRequest;

        // Add operation name if we have one and it's not already in the set
        if (operationName && !existing.operations.includes(operationName)) {
          existing.operations.push(operationName);
        }

        // Update status code if we have one
        if (statusCode) {
          existing.statusCode = statusCode;
        }

        // Update latency if we have one
        if (latency) {
          existing.latency = Number.parseFloat(latency);
        }

        // Update timestamp to latest
        if (timestamp) {
          existing.timestamp = timestamp;
        }

        // Move to front
        this.recentRequests.splice(existingIndex, 1);
        this.recentRequests.unshift(existing);
      } else {
        // Add new request to front
        this.recentRequests.unshift({
          requestId,
          path: path,
          operations: operationName ? [operationName] : [],
          method: method || "POST",
          statusCode: statusCode || null,
          latency: latency || null,
          timestamp: timestamp || new Date().toLocaleTimeString(),
        });

        // Keep only last 5
        if (this.recentRequests.length > 5) {
          this.recentRequests.pop();
        }
      }

      // Also check recent logs for this request_id to find gql.operationName
      if (requestId && !operationName) {
        const recentLogs = this.logs.slice(-100); // Check last 100 logs
        for (const recentLog of recentLogs) {
          if (recentLog.entry?.fields?.request_id === requestId) {
            const gqlOp = recentLog.entry?.fields?.["gql.operationName"];
            if (gqlOp) {
              const idx = this.recentRequests.findIndex((r) => r.requestId === requestId);
              if (idx !== -1 && !this.recentRequests[idx].operations.includes(gqlOp)) {
                this.recentRequests[idx].operations.push(gqlOp);
              }
            }
          }
        }
      }
    },

    sendFilterUpdate() {
      if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
        console.log("Cannot send filter update - WebSocket not connected");
        return;
      }

      const filter = {
        selectedContainers: Array.from(this.selectedContainers),
        selectedLevels: Array.from(this.selectedLevels),
        searchQuery: this.searchQuery,
        traceFilters: Array.from(this.traceFilters.entries()).map(([type, value]) => ({ type, value })),
      };

      console.log("Sending filter update:", filter);

      this.ws.send(
        JSON.stringify({
          type: "filter",
          data: filter,
        })
      );
    },

    handleContainerUpdate(data: ContainerData) {
      const newContainers = data.containers;
      const oldNames = new Set(this.containers.map((c: Container) => c.Name));
      const newNames = new Set(newContainers.map((c: Container) => c.Name));

      const added = newContainers.filter((c: Container) => !oldNames.has(c.Name));
      const removed = this.containers.filter((c: Container) => !newNames.has(c.Name));

      if (added.length > 0) {
        added.forEach((c: Container) => {
          this.selectedContainers.add(c.Name);
          console.log(`Container started: ${c.Name} (${c.ID})`);
        });
        this.sendFilterUpdate();
      }

      if (removed.length > 0) {
        removed.forEach((c) => {
          this.selectedContainers.delete(c.Name);
          console.log(`Container stopped: ${c.Name} (${c.ID})`);
        });
        this.sendFilterUpdate();
      }

      this.containers = newContainers;

      // Update port to server mapping
      if (data.portToServerMap) {
        this.portToServerMap = data.portToServerMap;
        console.log("Updated port to server map:", this.portToServerMap);
      }

      // Update log counts
      if (data.logCounts) {
        this.logCounts = data.logCounts;
        console.log("Updated log counts:", this.logCounts);
      }

      // Update retentions
      if (data.retentions) {
        this.retentions = data.retentions;
        console.log("Updated retentions:", this.retentions);
      }

      if (this.hasTraceFilters) {
        this.analyzeTrace();
      }
    },

    toggleContainer(containerName: Container["Name"]) {
      if (this.selectedContainers.has(containerName)) {
        this.selectedContainers.delete(containerName);
      } else {
        this.selectedContainers.add(containerName);
      }
      this.saveContainerState();
      this.sendFilterUpdate();
    },

    isContainerSelected(containerName: Container["Name"]) {
      return this.selectedContainers.has(containerName);
    },

    toggleProject(project: string) {
      const projectContainers = this.containersByProject[project];
      const allSelected = projectContainers.every((c) => this.selectedContainers.has(c.Name));

      projectContainers.forEach((c) => {
        if (allSelected) {
          this.selectedContainers.delete(c.Name);
        } else {
          this.selectedContainers.add(c.Name);
        }
      });
      this.saveContainerState();
      this.sendFilterUpdate();
    },

    isProjectSelected(project: string) {
      const projectContainers = this.containersByProject[project];
      return projectContainers.every((c) => this.selectedContainers.has(c.Name));
    },

    isProjectIndeterminate(project: string) {
      const projectContainers = this.containersByProject[project];
      const someSelected = projectContainers.some((c) => this.selectedContainers.has(c.Name));
      const allSelected = projectContainers.every((c) => this.selectedContainers.has(c.Name));
      return someSelected && !allSelected;
    },

    toggleProjectCollapse(project: string) {
      if (this.collapsedProjects.has(project)) {
        this.collapsedProjects.delete(project);
      } else {
        this.collapsedProjects.add(project);
      }
    },

    isProjectCollapsed(project: string) {
      return this.collapsedProjects.has(project);
    },

    getContainerName(containerId: Container["ID"]) {
      return this.containers.find((c) => c.ID === containerId)?.Name || containerId;
    },

    getShortContainerName(containerId: Container["ID"]) {
      const container = this.containers.find((c) => c.ID === containerId);
      if (container) {
        if (container.Project) {
          return container.Name.replace(`${container.Project}-`, "");
        }
        return container.Name;
      }
    },

    formatTimestamp(timestamp: string) {
      if (!timestamp) return "";

      // If timestamp is already in HH:MM:SS format, return it
      if (timestamp.match(/^\d{2}:\d{2}:\d{2}/)) {
        return timestamp.substring(0, 8); // Just HH:MM:SS
      }

      // Try to parse and format the timestamp
      try {
        // Handle various formats and extract time portion
        // Oct  3 19:57:52.076536 -> 19:57:52
        const timeMatch = timestamp.match(/(\d{2}):(\d{2}):(\d{2})(\.\d+)?/);
        if (timeMatch) {
          return `${timeMatch[1]}:${timeMatch[2]}:${timeMatch[3]}${timeMatch[4] ? `.${timeMatch[4].substring(1)}` : ""}`;
        }
      } catch {
        // If parsing fails, return original
      }

      return timestamp;
    },

    setTraceFilter(type, value, event) {
      if (event) event.stopPropagation();
      this.traceFilters.set(type, value);
      this.sendFilterUpdate();
      this.updateURL();
      this.analyzeTrace();
    },

    removeTraceFilter(type) {
      this.traceFilters.delete(type);
      this.sendFilterUpdate();
      this.updateURL();
      if (this.traceFilters.size === 0) {
        this.showAnalyzer = false;
      } else {
        this.analyzeTrace();
      }
    },

    clearTraceFilters() {
      this.traceFilters.clear();
      this.sendFilterUpdate();
      this.updateURL();
      this.showAnalyzer = false;
    },

    async saveTrace() {
      if (this.traceFilters.size === 0) return;

      const filterDesc = Array.from(this.traceFilters.entries())
        .map(([k, v]) => `${k}: ${v}`)
        .join(", ");
      const name = prompt(`Save trace as:`, filterDesc);
      if (!name) return;

      try {
        // Get all logs for this trace
        const traceLogs = this.logs.filter((log) => {
          const containerName = this.getContainerName(log.containerId);
          if (!this.selectedContainers.has(containerName)) {
            return false;
          }
          // Match ALL filters
          for (const [type, value] of this.traceFilters.entries()) {
            const val = log.entry?.fields?.[type];
            if (val !== value) return false;
          }
          return true;
        });

        // Extract SQL queries
        const sqlQueries = this.extractSQLQueries(traceLogs);

        const payload = {
          traceId: this.traceFilters.get("trace_id") || null,
          requestId: this.traceFilters.get("request_id") || null,
          name: name,
          logs: traceLogs,
          sqlQueries: sqlQueries,
        };

        const result = await API.post<SaveTraceResponse>("/api/save-trace", payload);
        alert(`Trace saved successfully! ID: ${result.id}`);
      } catch (error) {
        console.error("Error saving trace:", error);
        alert(`Failed to save trace: ${error.message}`);
      }
    },

    analyzeTrace() {
      if (this.traceFilters.size === 0) {
        this.showAnalyzer = false;
        return;
      }

      const traceLogs = this.logs.filter((log) => {
        const containerName = this.getContainerName(log.containerId);
        if (!this.selectedContainers.has(containerName)) {
          return false;
        }
        // Match ALL filters
        for (const [type, value] of this.traceFilters.entries()) {
          const val = log.entry?.fields?.[type];
          if (val !== value) return false;
        }
        return true;
      });

      const sqlQueries = this.extractSQLQueries(traceLogs);
      this.renderAnalysis(sqlQueries);
      this.showAnalyzer = true;
    },

    extractSQLQueries(logs) {
      const queries = [];

      logs.forEach((log) => {
        const message = log.entry?.message || "";
        if (message.includes("[sql]")) {
          const sqlMatch = message.match(/\[sql\]:\s*(.+)/i);
          if (sqlMatch) {
            const query = sqlMatch[1].trim();
            const duration = parseFloat(log.entry?.fields?.duration || 0);
            const table = log.entry?.fields?.["db.table"] || "unknown";
            const operation = log.entry?.fields?.["db.operation"] || "unknown";
            const rows = parseInt(log.entry?.fields?.["db.rows"] || 0);

            let variables = {};
            const dbVars = log.entry?.fields?.["db.vars"];
            if (dbVars) {
              try {
                const varsArray = typeof dbVars === "string" ? JSON.parse(dbVars) : dbVars;
                if (Array.isArray(varsArray)) {
                  varsArray.forEach((val, idx) => {
                    variables[String(idx + 1)] = String(val);
                  });
                }
              } catch (e) {
                console.warn("Failed to parse db.vars:", dbVars, e);
              }
            }

            queries.push({
              query,
              duration,
              table,
              operation,
              rows,
              variables,
              normalized: this.normalizeQuery(query),
            });
          }
        }
      });

      return queries;
    },

    normalizeQuery(query) {
      return query
        .replace(/\$\d+/g, "$N")
        .replace(/'[^']*'/g, "'?'")
        .replace(/\d+/g, "N")
        .replace(/\s+/g, " ")
        .trim();
    },

    renderAnalysis(queries: SQLQuery[]) {
      if (queries.length === 0) {
        this.sqlAnalysis = {
          totalQueries: 0,
          uniqueQueries: 0,
          avgDuration: 0,
          totalDuration: 0,
          slowestQueries: [],
          frequentQueries: [],
          nPlusOne: [],
          tables: [],
        } as SQLAnalysis;
        return;
      }

      const totalQueries = queries.length;
      const totalDuration = queries.reduce((sum, q) => sum + q.duration, 0);
      const avgDuration = totalDuration / totalQueries;

      const queryGroups: Record<string, { queries: SQLQuery[]; count: number }> = {};
      queries.forEach((q) => {
        if (!queryGroups[q.normalized]) {
          queryGroups[q.normalized] = {
            queries: [],
            count: 0,
          };
        }
        queryGroups[q.normalized].queries.push(q);
        queryGroups[q.normalized].count++;
      });

      const uniqueQueries = Object.keys(queryGroups).length;

      const slowestQueries = [...queries].sort((a, b) => b.duration - a.duration).slice(0, 5);

      const frequentQueries: FrequentQuery[] = Object.entries(queryGroups)
        .map(([normalized, data]) => ({
          normalized,
          count: data.count,
          example: data.queries[0],
          avgDuration: data.queries.reduce((sum, q) => sum + q.duration, 0) / data.count,
        }))
        .sort((a, b) => b.count - a.count)
        .slice(0, 5);

      const nPlusOne = frequentQueries.filter((item) => item.count > 5);

      const tables = {} as Record<string, number>;
      queries.forEach((q) => {
        if (!tables[q.table]) {
          tables[q.table] = 0;
        }
        tables[q.table]++;
      });

      const tablesList = Object.entries(tables)
        .sort((a, b) => b[1] - a[1])
        .map(([table, count]) => ({ table, count }));

      this.sqlAnalysis = {
        totalQueries,
        uniqueQueries,
        avgDuration,
        totalDuration,
        slowestQueries,
        frequentQueries,
        nPlusOne,
        tables: tablesList,
      };
    },

    openLogDetails(log) {
      this.selectedLog = log;
      this.showLogModal = true;
      // Apply syntax highlighting after modal opens
      this.$nextTick(() => {
        this.applySyntaxHighlighting();
      });
    },

    // Wrapper method for template usage (templates can't call imported functions directly)
    convertAnsiToHtml: convertAnsiToHtmlUtil,

    formatMessage(message) {
      if (!message) return message;
      const sqlIndex = message.indexOf("[sql]:");
      if (sqlIndex >= 0) {
        return formatSQLUtil(message.substring(sqlIndex + 6));
      }
      return message;
    },

    formatAndHighlightSQL(message) {
      if (!message) return message;
      if (message.includes("[sql]")) {
        const sqlMatch = message.match(/\[sql\]:\s*(.+)/i);
        if (sqlMatch) {
          const sql = sqlMatch[1].trim();
          const formatted = formatSQLUtil(sql);
          // Apply syntax highlighting if hljs is available
          if (typeof hljs !== "undefined") {
            try {
              const highlighted = hljs.highlight(formatted, { language: "sql" });
              return highlighted.value;
            } catch (e) {
              console.error("Error highlighting SQL:", e);
              return formatted;
            }
          }
          return formatted;
        }
      }
      return message;
    },

    isSQLMessage(message) {
      return message && message.includes("[sql]");
    },

    getRemainingFields(fields) {
      if (!fields) return [];
      const excluded = new Set(["request_id", "trace_id", "gql.operationName", "db.rows", "db.table", "duration"]);
      return Object.entries(fields).filter(([key]) => !excluded.has(key));
    },

    getDatabaseConnectionString() {
      // Find the first container port that maps to a database
      for (const container of this.containers) {
        if (!container.Ports) continue;

        for (const port of container.Ports) {
          if (port.publicPort && this.portToServerMap[port.publicPort]) {
            console.log(`Using database for port ${port.publicPort}:`, this.portToServerMap[port.publicPort]);
            return this.portToServerMap[port.publicPort];
          }
        }
      }

      // No matching port found, return empty to use default
      console.log("No port mapping found, using default database connection");
      return "";
    },

    async runExplain(query, variables = {}, metadata = null) {
      try {
        // Determine connection string based on container ports
        const connectionString = this.getDatabaseConnectionString();

        // Convert variables to string map (backend expects map[string]string)
        const varsStringMap = {};
        if (variables) {
          for (const [key, value] of Object.entries(variables)) {
            varsStringMap[key] = typeof value === "string" ? value : JSON.stringify(value);
          }
        }

        const payload = {
          query: query,
          variables: varsStringMap,
          connectionString: connectionString,
        };

        const result = await API.post<ExplainResponse>("/api/explain", payload);

        if (result.error) {
          this.explainData.error = result.error;
          this.explainData.planSource = "";
          this.explainData.planQuery = result.query || "";
          this.explainData.metadata = metadata;
        } else {
          let planText = "";
          if (result.queryPlan && result.queryPlan.length > 0) {
            planText = JSON.stringify(result.queryPlan, null, 2);
          }

          this.explainData.error = null;
          this.explainData.planSource = planText;
          this.explainData.planQuery = result.query || "";
          this.explainData.metadata = metadata;
        }

        this.showExplainModal = true;
      } catch (error) {
        this.explainData.error = `Failed to run EXPLAIN: ${error.message}`;
        this.explainData.planSource = "";
        this.explainData.planQuery = query;
        this.explainData.metadata = metadata;
        this.showExplainModal = true;
      }
    },

    closeExplainPlanModal() {
      this.showExplainModal = false;
    },

    openRetentionModal(containerName) {
      this.retentionContainer = containerName;
      const existing = this.retentions[containerName];
      if (existing) {
        this.retentionForm.type = existing.type;
        this.retentionForm.value = existing.value;
      } else {
        this.retentionForm.type = "count";
        this.retentionForm.value = 1000;
      }
      this.showRetentionModal = true;
    },

    getRetentionTooltip(containerName) {
      const retention = this.retentions[containerName];
      if (!retention) {
        return "Click to set retention policy";
      }
      if (retention.type === "count") {
        return `Retention: ${retention.value} logs`;
      } else {
        return `Retention: ${retention.value} seconds (${Math.floor(
          retention.value / 3600
        )}h ${Math.floor((retention.value % 3600) / 60)}m)`;
      }
    },

    async saveRetention() {
      try {
        const data = await API.post<RetentionResponse>("/api/retention", {
          containerName: this.retentionContainer,
          retentionType: this.retentionForm.type,
          retentionValue: this.retentionForm.value,
        });

        this.retentions[this.retentionContainer] = {
          type: data.retentionType,
          value: data.retentionValue,
        };
        this.showRetentionModal = false;
      } catch (error) {
        console.error("Error saving retention:", error);
      }
    },

    async deleteRetention() {
      try {
        await API.delete(`/api/retention/${encodeURIComponent(this.retentionContainer)}`);
        delete this.retentions[this.retentionContainer];
        this.showRetentionModal = false;
      } catch (error) {
        console.error("Error deleting retention:", error);
      }
    },

    async shareExplainPlan() {
      try {
        const form = document.createElement("form");
        form.method = "POST";
        form.action = "https://explain.dalibo.com/new";
        form.target = "_blank";

        // Build descriptive title from metadata
        let title = "Query Plan from Logseidon";
        if (this.explainData.metadata) {
          const parts = [];
          if (this.explainData.metadata.type) {
            parts.push(this.explainData.metadata.type);
          }
          if (this.explainData.metadata.operation) {
            parts.push(this.explainData.metadata.operation.toUpperCase());
          }
          if (this.explainData.metadata.table) {
            parts.push(`on ${this.explainData.metadata.table}`);
          }
          if (parts.length > 0) {
            title = parts.join(" ") + " - Logseidon";
          }
        }

        const titleInput = document.createElement("input");
        titleInput.type = "hidden";
        titleInput.name = "title";
        titleInput.value = title;
        form.appendChild(titleInput);

        const planInput = document.createElement("input");
        planInput.type = "hidden";
        planInput.name = "plan";
        planInput.value = this.explainData.planSource;
        form.appendChild(planInput);

        const queryInput = document.createElement("input");
        queryInput.type = "hidden";
        queryInput.name = "query";
        queryInput.value = this.explainData.planQuery;
        form.appendChild(queryInput);

        document.body.appendChild(form);
        form.submit();
        document.body.removeChild(form);
      } catch (error) {
        console.error("Error sharing plan:", error);
        alert(`Failed to share plan: ${error.message}`);
      }
    },

    scrollToBottom() {
      const logsEl = this.$refs.logsContainer;
      if (logsEl) {
        logsEl.scrollTop = logsEl.scrollHeight;
      }
    },

    escapeHtml(text) {
      const div = document.createElement("div");
      div.textContent = text;
      return div.innerHTML;
    },

    shouldShowField(key, value) {
      // Always show error field
      if (key === "error" || key === "stack_trace" || key === "db.error") return true;
      // Show fields less than 40 characters
      const s = String(value);
      return s.length < 40;
    },

    formatFieldValue(key, value) {
      const s = String(value);
      if (key === "stack_trace") {
        const ret = [];
        value.split("\\n").forEach((line, index) => {
          if (index < 5) {
            ret.push(line);
          }
        });
        return ret.join(" ").replaceAll("\\t", "    ");
      }
      if (key === "error" || key === "db.error") {
        return value;
      }

      return s.length > 50 ? s.substring(0, 20) + "..." : s;
    },

    isJsonField(value) {
      return value.trim().startsWith("{") || value.trim().startsWith("[");
    },

    formatJsonField(value) {
      try {
        const parsed = JSON.parse(value);
        return JSON.stringify(parsed, null, 2);
      } catch (e) {
        console.error("Error formatting JSON field:", e);
        return value;
      }
    },

    saveContainerState() {
      try {
        localStorage.setItem("selectedContainers", JSON.stringify([...this.selectedContainers]));
      } catch (e) {
        console.warn("Failed to save container state:", e);
      }
    },
  },
});
</script>
