import { createNavigation } from "./shared/navigation.js";
import { API } from "./shared/api.js";
import { formatSQL } from "./utils.js";

const { createApp } = Vue;

const app = createApp({
  data() {
    return {
      requestDetail: null,
      loading: true,
      error: null,
      explainPlanData: null,
      showExplainPlanPanel: false,
      pev2App: null,
      showRequestModal: false,
      showResponseModal: false,
    };
  },

  computed: {
    filteredRequestLogs() {
      if (!this.requestDetail || !this.requestDetail.logs) {
        return [];
      }
      // Filter out TRACE level logs (case-insensitive)
      return this.requestDetail.logs.filter((log) => {
        const level = (log.level || "").toUpperCase();
        return level !== "TRC" && level !== "TRACE";
      });
    },

    requestViewerLink() {
      if (
        !this.requestDetail ||
        !this.requestDetail.execution.requestIdHeader
      ) {
        return null;
      }
      const requestId = this.requestDetail.execution.requestIdHeader;
      return `/?request_id=${encodeURIComponent(requestId)}`;
    },

    statusClass() {
      if (!this.requestDetail) return "";
      const statusCode = this.requestDetail.execution.statusCode;
      if (!statusCode || statusCode === 0) return "pending";
      return statusCode >= 200 && statusCode < 300 ? "success" : "error";
    },

    requestData() {
      if (!this.requestDetail?.execution.requestBody)
        return "(no request data)";
      try {
        const data = JSON.parse(this.requestDetail.execution.requestBody);
        return JSON.stringify(data, null, 2);
      } catch (e) {
        return this.requestDetail.execution.requestBody;
      }
    },

    responseBody() {
      if (!this.requestDetail?.execution.responseBody) return "(no response)";
      try {
        const data = JSON.parse(this.requestDetail.execution.responseBody);
        return JSON.stringify(data, null, 2);
      } catch (e) {
        return this.requestDetail.execution.responseBody;
      }
    },

    sqlAnalysisData() {
      if (
        !this.requestDetail?.sqlQueries ||
        this.requestDetail.sqlQueries.length === 0
      ) {
        return null;
      }

      const queries = this.requestDetail.sqlQueries.map((q) => ({
        query: q.query,
        duration: q.durationMs,
        table: q.tableName || "unknown",
        operation: q.operation || "SELECT",
        rows: q.rows || 0,
        variables: q.variables
          ? typeof q.variables === "string"
            ? JSON.parse(q.variables)
            : q.variables
          : {},
        normalized: this.normalizeQuery(q.query),
      }));

      const totalQueries = queries.length;
      const totalDuration = queries.reduce((sum, q) => sum + q.duration, 0);
      const avgDuration = totalDuration / totalQueries;

      const queryGroups = {};
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

      const slowestQueries = [...queries]
        .sort((a, b) => b.duration - a.duration)
        .slice(0, 5);

      const frequentQueries = Object.entries(queryGroups)
        .map(([normalized, data]) => ({
          normalized,
          count: data.count,
          example: data.queries[0],
          avgDuration:
            data.queries.reduce((sum, q) => sum + q.duration, 0) / data.count,
        }))
        .sort((a, b) => b.count - a.count)
        .slice(0, 5);

      const nPlusOne = frequentQueries.filter((item) => item.count > 5);

      const tables = {};
      queries.forEach((q) => {
        if (!tables[q.table]) {
          tables[q.table] = 0;
        }
        tables[q.table]++;
      });

      const tablesList = Object.entries(tables)
        .sort((a, b) => b[1] - a[1])
        .map(([table, count]) => ({ table, count }));

      return {
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
  },

  async mounted() {
    const params = new URLSearchParams(window.location.search);
    const requestId = params.get("id");

    if (!requestId) {
      this.error = "No request ID provided";
      this.loading = false;
      return;
    }

    await this.loadRequestDetail(requestId);
  },

  methods: {
    async loadRequestDetail(requestId) {
      try {
        this.requestDetail = await API.get(`/api/executions/${requestId}`);
        this.loading = false;
      } catch (error) {
        console.error("Failed to load request detail:", error);
        this.error = error.message;
        this.loading = false;
      }
    },

    goBack() {
      window.history.back();
    },

    handleExplainClick(queryIdx) {
      const query = this.requestDetail.sqlQueries[queryIdx];

      if (query.explainPlan && query.explainPlan.length > 0) {
        // Show saved plan
        try {
          const plan = JSON.parse(query.explainPlan);
          this.displayExplainPlan(plan, query.query);
        } catch (err) {
          alert("Error parsing saved plan: " + err.message);
        }
      } else {
        // Run new EXPLAIN
        const variables = query.variables ? JSON.parse(query.variables) : {};
        this.runExplain(
          query.query,
          variables,
          this.requestDetail?.server?.defaultDatabase?.connectionString
        );
      }
    },

    displayExplainPlan(planData, query) {
      this.explainPlanData = {
        planData,
        query,
        error: null,
      };

      this.showExplainPlanPanel = true;

      // Need to mount PEV2 after panel is visible
      this.$nextTick(() => {
        this.mountPEV2(planData, query);
      });
    },

    mountPEV2(planData, query) {
      const pev2Container = document.getElementById("pev2ExplainApp");
      if (!pev2Container) return;

      try {
        // Unmount any existing Vue app
        if (this.pev2App) {
          this.pev2App.unmount();
        }

        // Create new Vue app with PEV2
        const { createApp } = Vue;
        this.pev2App = createApp({
          data() {
            return {
              planSource: JSON.stringify(planData, null, 2),
              planQuery: query,
            };
          },
          template:
            '<pev2 :plan-source="planSource" :plan-query="planQuery"></pev2>',
        });

        this.pev2App.component("pev2", pev2.Plan);
        this.pev2App.mount(pev2Container);
      } catch (err) {
        this.explainPlanData.error = `Failed to display plan: ${err.message}`;
      }
    },

    closeExplainPlanModal() {
      this.showExplainPlanPanel = false;
      if (this.pev2App) {
        this.pev2App.unmount();
        this.pev2App = null;
      }
    },

    async shareExplainPlan() {
      if (!this.explainPlanData) return;
      try {
        const form = document.createElement("form");
        form.method = "POST";
        form.action = "https://explain.dalibo.com/new";
        form.target = "_blank";

        // Build descriptive title from query and request context
        let title = "Query Plan";
        if (this.requestDetail) {
          const parts = [];

          // Add request name if available
          if (this.requestDetail.request?.name) {
            parts.push(this.requestDetail.request.name);
          }

          // Extract table name or operation from the query
          const query = this.explainPlanData.query.toLowerCase();
          if (query.includes("from")) {
            const match = query.match(/from\s+([a-z_][a-z0-9_]*)/i);
            if (match) {
              parts.push(`on ${match[1]}`);
            }
          }

          if (parts.length > 0) {
            title = parts.join(" - ");
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
        planInput.value = JSON.stringify(
          this.explainPlanData.planData,
          null,
          2
        );
        form.appendChild(planInput);

        const queryInput = document.createElement("input");
        queryInput.type = "hidden";
        queryInput.name = "query";
        queryInput.value =
          this.formatSQL(this.explainPlanData.query) ||
          this.explainPlanData.query;
        form.appendChild(queryInput);

        document.body.appendChild(form);
        form.submit();
        document.body.removeChild(form);
      } catch (error) {
        console.error("Error sharing plan:", error);
        alert(`Failed to share plan: ${error.message}`);
      }
    },

    async runExplain(query, variables = {}, connectionString) {
      try {
        const vars = {};
        if (Array.isArray(variables)) {
          for (let i = 0; i < variables.length; i++) {
            vars[`${i + 1}`] = variables[i];
          }
        } else {
          for (const [key, value] of Object.entries(variables)) {
            vars[key] = value;
          }
        }

        const payload = {
          query: query,
          variables: vars,
          connectionString: connectionString,
        };

        const result = await API.post("/api/explain", payload);

        if (result.error) {
          alert(`EXPLAIN Error: ${result.error}`);
        } else {
          const displayQuery = result.formattedQuery || result.query || query;
          this.displayExplainPlan(result.queryPlan, displayQuery);
        }
      } catch (error) {
        alert(`Failed to run EXPLAIN: ${error.message}`);
      }
    },

    // Wrapper for global formatSQL function
    formatSQL(sql) {
      return formatSQL(sql);
    },

    normalizeQuery(query) {
      return query
        .replace(/\$\d+/g, "$N")
        .replace(/'[^']*'/g, "'?'")
        .replace(/\d+/g, "N")
        .replace(/\s+/g, " ")
        .trim();
    },

    async copyToClipboard(text) {
      try {
        await navigator.clipboard.writeText(text);
        // Show a brief notification
        const notification = document.createElement("div");
        notification.textContent = "Copied to clipboard!";
        notification.style.cssText =
          "position: fixed; top: 20px; right: 20px; background: #238636; color: white; padding: 0.75rem 1rem; border-radius: 4px; z-index: 10000; font-size: 0.875rem;";
        document.body.appendChild(notification);
        setTimeout(() => notification.remove(), 2000);
      } catch (err) {
        console.error("Failed to copy:", err);
        alert("Failed to copy to clipboard");
      }
    },

    viewBigger(type) {
      if (type === "request") {
        this.showRequestModal = true;
      } else {
        this.showResponseModal = true;
      }
    },
  },

  template: `
    <div class="app-container">
      <header class="app-header">
        <div style="display: flex; align-items: center; gap: 1rem">
          <h1 style="margin: 0">🔱 Logseidon</h1>
          <app-nav></app-nav>
        </div>
      </header>

      <div class="main-layout">
        <main class="content" style="margin: 0; padding: 2rem;">
          <!-- Loading State -->
          <div v-if="loading" style="text-align: center; padding: 3rem;">
            <p>Loading request details...</p>
          </div>

          <!-- Error State -->
          <div v-if="error" style="text-align: center; padding: 3rem;">
            <div class="alert alert-danger">{{ error }}</div>
            <button @click="goBack" class="btn-secondary">Go Back</button>
          </div>

          <!-- Request Detail Content -->
          <div v-if="!loading && !error && requestDetail">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem;">
              <div>
                <button @click="goBack" class="btn-secondary" style="margin-bottom: 0.5rem;">← Back to Requests</button>
                <h2 style="margin: 0;">{{ requestDetail.request?.name || '(unnamed)' }}</h2>
                <p style="color: #8b949e; margin: 0.25rem 0 0 0;">{{ requestDetail.server?.name || requestDetail.execution.server?.name || 'N/A' }}</p>
              </div>
            </div>

            <div class="execution-overview">
              <div class="stat-item">
                <span class="stat-label">Status Code</span>
                <span class="stat-value" :class="statusClass">{{ requestDetail.execution.statusCode || 'Error' }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">Duration</span>
                <span class="stat-value">{{ requestDetail.execution.durationMs }}ms</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">Request ID</span>
                <span class="stat-value">{{ requestDetail.execution.requestIdHeader }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">Executed At</span>
                <span class="stat-value">{{ new Date(requestDetail.execution.executedAt).toLocaleString() }}</span>
              </div>
              <div class="stat-item" v-if="requestDetail.server?.devId || requestDetail.execution.server?.devId">
                <span class="stat-label">Dev ID</span>
                <span class="stat-value">{{ requestDetail.server?.devId || requestDetail.execution.server?.devId || 'N/A' }}</span>
              </div>
            </div>

            <div class="modal-section">
              <div style="display: flex; gap: 1rem; flex-wrap: wrap;">
                <div style="flex: 1; min-width: 300px;">
                  <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem;">
                    <h4 style="margin: 0;">Request Body</h4>
                    <div style="display: flex; gap: 0.5rem;">
                      <button @click="copyToClipboard(requestData)" class="btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;" title="Copy request">📋 Copy</button>
                      <button @click="viewBigger('request')" class="btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;" title="View bigger">🔍 View</button>
                    </div>
                  </div>
                  <pre class="json-display" style="max-height: 20em;">{{ requestData }}</pre>
                </div>
                <div style="flex: 1; min-width: 300px;">
                  <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem;">
                    <h4 style="margin: 0;">Response</h4>
                    <div style="display: flex; gap: 0.5rem;">
                      <button @click="copyToClipboard(responseBody)" class="btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;" title="Copy response">📋 Copy</button>
                      <button @click="viewBigger('response')" class="btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;" title="View bigger">🔍 View</button>
                    </div>
                  </div>
                  <pre class="json-display" style="max-height: 20em;">{{ responseBody }}</pre>
                </div>
              </div>
            </div>

            <div v-if="requestDetail.execution.error" class="modal-section">
              <h4>Error</h4>
              <pre class="error-display">{{ requestDetail.execution.error }}</pre>
            </div>

            <div v-if="sqlAnalysisData" class="modal-section">
              <h4>SQL Query Analyzer</h4>
              
              <div class="analyzer-subsection" style="margin-bottom: 1.5rem;">
                <h5 style="color: #8b949e; font-size: 0.9rem; margin-bottom: 0.5rem; text-transform: uppercase; letter-spacing: 0.05em;">Overview</h5>
                <div class="stats-grid-compact">
                  <div class="stat-item">
                    <span class="stat-label">Total Queries</span>
                    <span class="stat-value">{{ sqlAnalysisData.totalQueries }}</span>
                  </div>
                  <div class="stat-item">
                    <span class="stat-label">Unique</span>
                    <span class="stat-value">{{ sqlAnalysisData.uniqueQueries }}</span>
                  </div>
                  <div class="stat-item">
                    <span class="stat-label">Avg Duration</span>
                    <span class="stat-value">{{ sqlAnalysisData.avgDuration.toFixed(2) }}ms</span>
                  </div>
                  <div class="stat-item">
                    <span class="stat-label">Total Duration</span>
                    <span class="stat-value">{{ sqlAnalysisData.totalDuration.toFixed(2) }}ms</span>
                  </div>
                </div>
              </div>

              <div style="display: flex; gap: 1rem; flex-wrap: wrap; margin-bottom: 1.5rem;">
                <div v-if="sqlAnalysisData.slowestQueries.length > 0" class="analyzer-subsection" style="flex: 1; min-width: 280px;">
                  <h5 style="color: #8b949e; font-size: 0.9rem; margin-bottom: 0.5rem; text-transform: uppercase; letter-spacing: 0.05em;">Slowest Queries</h5>
                  <div class="query-list-compact">
                    <div v-for="(q, index) in sqlAnalysisData.slowestQueries.slice(0, 5)" :key="index" class="query-item-compact">
                      <div class="query-header-compact">
                        <span class="query-duration" :class="{ 'query-slow': q.duration > 10 }">{{ q.duration.toFixed(2) }}ms</span>
                        <span class="query-meta-inline">{{ q.table }} · {{ q.operation }}</span>
                      </div>
                      <div class="query-text-compact">{{ q.query.substring(0, 100) }}{{ q.query.length > 100 ? '...' : '' }}</div>
                      <button class="btn-explain-compact" @click="handleExplainClick(Math.max(0, requestDetail.sqlQueries.findIndex(sq => sq.query === q.query)))">EXPLAIN</button>
                    </div>
                  </div>
                </div>

                <div v-if="sqlAnalysisData.frequentQueries.length > 0" class="analyzer-subsection" style="flex: 1; min-width: 280px;">
                  <h5 style="color: #8b949e; font-size: 0.9rem; margin-bottom: 0.5rem; text-transform: uppercase; letter-spacing: 0.05em;">Most Frequent</h5>
                  <div class="query-list-compact">
                    <div v-for="(item, index) in sqlAnalysisData.frequentQueries.slice(0, 5)" :key="index" class="query-item-compact">
                      <div class="query-header-compact">
                        <span class="query-count">{{ item.count }}x</span>
                        <span class="query-meta-inline">{{ item.example.table }} · {{ item.avgDuration.toFixed(2) }}ms</span>
                      </div>
                      <div class="query-text-compact">{{ item.example.query.substring(0, 100) }}{{ item.example.query.length > 100 ? '...' : '' }}</div>
                      <button class="btn-explain-compact" @click="handleExplainClick(Math.max(0, requestDetail.sqlQueries.findIndex(sq => sq.query === item.example.query)))">EXPLAIN</button>
                    </div>
                  </div>
                </div>

                <div v-if="requestDetail.indexAnalysis && requestDetail.indexAnalysis.recommendations && requestDetail.indexAnalysis.recommendations.length > 0" class="analyzer-subsection" style="flex: 1; min-width: 280px;">
                  <h5 style="color: #8b949e; font-size: 0.9rem; margin-bottom: 0.5rem; text-transform: uppercase; letter-spacing: 0.05em;">Index Recommendations</h5>
                  <div class="query-list-compact">
                    <div v-for="(rec, index) in requestDetail.indexAnalysis.recommendations.slice(0, 5)" :key="index" class="query-item-compact">
                      <div class="query-header-compact">
                        <span class="index-priority-badge" :class="'priority-' + rec.priority" style="font-size: 0.65rem; padding: 0.15rem 0.3rem;">{{ rec.priority.toUpperCase() }}</span>
                        <span class="query-meta-inline">{{ rec.tableName }}</span>
                      </div>
                      <div style="font-size: 0.7rem; color: #c9d1d9; margin-bottom: 0.25rem;">{{ rec.reason }}</div>
                      <div style="font-size: 0.65rem; color: #8b949e; margin-bottom: 0.25rem;">{{ rec.columns.join(', ') }}</div>
                      <div style="font-size: 0.65rem; color: #7ee787;">{{ rec.estimatedImpact }}</div>
                    </div>
                  </div>
                </div>
              </div>

              <div v-if="sqlAnalysisData.tables.length > 0" class="analyzer-subsection" style="margin-bottom: 1.5rem;">
                <h5 style="color: #8b949e; font-size: 0.9rem; margin-bottom: 0.5rem; text-transform: uppercase; letter-spacing: 0.05em;">Tables</h5>
                <div class="table-list-compact">
                  <span v-for="(item, index) in sqlAnalysisData.tables" :key="index" class="table-badge-compact">
                    {{ item.table }}<span class="table-count">({{ item.count }})</span>
                  </span>
                </div>
              </div>

              <div class="analyzer-subsection">
                <h5 style="color: #8b949e; font-size: 0.9rem; margin-bottom: 0.5rem; text-transform: uppercase; letter-spacing: 0.05em;">All Queries</h5>
                <div class="sql-queries-list">
                  <div v-for="(q, idx) in requestDetail.sqlQueries" :key="idx" class="sql-query-item">
                    <div class="sql-query-header">
                      <span>{{ q.tableName || 'unknown' }} - {{ q.operation || 'SELECT' }}</span>
                      <span class="sql-query-duration">{{ q.durationMs.toFixed(2) }}ms</span>
                    </div>
                    <div class="sql-query-text">{{ formatSQL(q.query) }}</div>
                    <div class="query-actions">
                      <button v-if="!q.explainPlan || q.explainPlan.length === 0" 
                              class="btn-explain" 
                              @click="handleExplainClick(idx)">Run EXPLAIN</button>
                      <button v-if="q.explainPlan && q.explainPlan.length > 0" 
                              class="btn-explain btn-secondary" 
                              @click="handleExplainClick(idx)">View Saved Plan</button>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div v-if="requestDetail.indexAnalysis && (requestDetail.indexAnalysis.sequentialScans.length > 0 || requestDetail.indexAnalysis.recommendations.length > 0)" class="modal-section">
              <h4>Index Analysis</h4>
              <div class="stats-grid">
                <div class="stat-item">
                  <span class="stat-label">Sequential Scans</span>
                  <span class="stat-value" :class="{ 'text-warning': requestDetail.indexAnalysis.summary.sequentialScans > 0 }">{{ requestDetail.indexAnalysis.summary.sequentialScans }}</span>
                </div>
                <div class="stat-item">
                  <span class="stat-label">Index Scans</span>
                  <span class="stat-value">{{ requestDetail.indexAnalysis.summary.indexScans }}</span>
                </div>
                <div class="stat-item">
                  <span class="stat-label">Recommendations</span>
                  <span class="stat-value" :class="{ 'text-warning': requestDetail.indexAnalysis.summary.highPriorityRecs > 0 }">{{ requestDetail.indexAnalysis.summary.totalRecommendations }}</span>
                </div>
                <div class="stat-item">
                  <span class="stat-label">High Priority</span>
                  <span class="stat-value" :class="{ 'text-danger': requestDetail.indexAnalysis.summary.highPriorityRecs > 0 }">{{ requestDetail.indexAnalysis.summary.highPriorityRecs }}</span>
                </div>
              </div>
              
              <div v-if="requestDetail.indexAnalysis.sequentialScans.length > 0" style="margin-top: 1rem;">
                <h5 style="color: #8b949e; font-size: 0.9rem; margin-bottom: 0.5rem;">Sequential Scan Issues</h5>
                <div class="index-issues-list">
                  <div v-for="(issue, idx) in requestDetail.indexAnalysis.sequentialScans.slice(0, 5)" :key="idx" class="index-issue-item">
                    <div class="index-issue-header">
                      <span class="index-issue-table">{{ issue.tableName }}</span>
                      <span class="index-issue-stats">{{ issue.occurrences }}x · {{ issue.durationMs.toFixed(2) }}ms · cost: {{ issue.cost.toFixed(0) }}</span>
                    </div>
                    <div v-if="issue.filterCondition" class="index-issue-filter">Filter: {{ issue.filterCondition }}</div>
                  </div>
                </div>
              </div>

              <div v-if="requestDetail.indexAnalysis.recommendations.length > 0" style="margin-top: 1rem;">
                <h5 style="color: #8b949e; font-size: 0.9rem; margin-bottom: 0.5rem;">Index Recommendations</h5>
                <div class="index-recommendations-list">
                  <div v-for="(rec, idx) in requestDetail.indexAnalysis.recommendations.slice(0, 5)" :key="idx" class="index-recommendation-item">
                    <div class="index-recommendation-header">
                      <span class="index-priority-badge" :class="'priority-' + rec.priority">{{ rec.priority.toUpperCase() }}</span>
                      <span class="index-recommendation-table">{{ rec.tableName }}</span>
                    </div>
                    <div class="index-recommendation-reason">{{ rec.reason }}</div>
                    <div class="index-recommendation-columns">Columns: {{ rec.columns.join(', ') }}</div>
                    <div class="index-recommendation-impact">Impact: {{ rec.estimatedImpact }}</div>
                    <div class="index-recommendation-sql">
                      <code>{{ rec.sqlCommand }}</code>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div class="modal-section">
              <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem;">
                <h4 style="margin: 0;">Logs (<span>{{ filteredRequestLogs.length }}</span>) <span v-if="requestDetail.logs.length > filteredRequestLogs.length" style="color: #8b949e; font-size: 0.85rem; font-weight: normal;">({{ requestDetail.logs.length - filteredRequestLogs.length }} TRACE filtered)</span></h4>
                <a v-if="requestViewerLink" :href="requestViewerLink" target="_blank" class="btn-primary" style="padding: 0.35rem 0.75rem; font-size: 0.85rem; text-decoration: none;">View in Log Viewer →</a>
              </div>
              <div class="logs-list">
                <p v-if="filteredRequestLogs.length === 0" style="color: #6c757d;">No logs captured (or all logs are TRACE level)</p>
                <div v-for="(log, idx) in filteredRequestLogs" :key="idx" class="log-entry">
                  <div class="log-entry-header">
                    <span class="log-level" :class="log.level || 'INFO'">{{ log.level || 'INFO' }}</span>
                    <span class="log-timestamp">{{ new Date(log.timestamp).toLocaleTimeString() }}</span>
                  </div>
                  <div class="log-message">{{ log.message || log.rawLog }}</div>
                </div>
              </div>
            </div>
          </div>
        </main>
      </div>
    </div>

    <!-- Request Body Modal -->
    <div v-if="showRequestModal" class="modal" @click="showRequestModal = false">
      <div class="modal-content" @click.stop style="max-width: 900px;">
        <div class="modal-header">
          <h3>Request Body</h3>
          <div style="display: flex; gap: 0.5rem;">
            <button @click="copyToClipboard(requestData)" class="btn-secondary" style="padding: 0.5rem 1rem;">📋 Copy</button>
            <button @click="showRequestModal = false">✕</button>
          </div>
        </div>
        <div class="modal-body">
          <pre style="background: #0d1117; border: 1px solid #30363d; border-radius: 4px; padding: 1rem; overflow: auto; font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace; font-size: 0.875rem; line-height: 1.5; color: #c9d1d9; white-space: pre-wrap; word-break: break-word;">{{ requestData }}</pre>
        </div>
      </div>
    </div>

    <!-- Response Body Modal -->
    <div v-if="showResponseModal" class="modal" @click="showResponseModal = false">
      <div class="modal-content" @click.stop style="max-width: 900px;">
        <div class="modal-header">
          <h3>Response Body</h3>
          <div style="display: flex; gap: 0.5rem;">
            <button @click="copyToClipboard(responseBody)" class="btn-secondary" style="padding: 0.5rem 1rem;">📋 Copy</button>
            <button @click="showResponseModal = false">✕</button>
          </div>
        </div>
        <div class="modal-body">
          <pre style="background: #0d1117; border: 1px solid #30363d; border-radius: 4px; padding: 1rem; overflow: auto; font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace; font-size: 0.875rem; line-height: 1.5; color: #c9d1d9; white-space: pre-wrap; word-break: break-word;">{{ responseBody }}</pre>
        </div>
      </div>
    </div>

    <!-- EXPLAIN Plan Side Panel -->
    <div v-if="showExplainPlanPanel" class="side-panel-overlay" @click="closeExplainPlanModal">
      <div class="side-panel" @click.stop>
        <div class="side-panel-header">
          <h3>SQL Query EXPLAIN Plan (PEV2)</h3>
          <div style="display: flex; gap: 0.5rem;">
            <button @click="shareExplainPlan" class="btn-secondary" style="padding: 0.5rem 1rem;">
              📋 Share
            </button>
            <button @click="closeExplainPlanModal">✕</button>
          </div>
        </div>
        <div class="side-panel-body">
          <div
            v-if="explainPlanData && explainPlanData.error"
            class="alert alert-danger"
            style="display: block; margin: 1rem;">
            {{ explainPlanData.error }}
          </div>
          <div
            id="pev2ExplainApp"
            class="d-flex flex-column"
            style="height: 100%;"
          ></div>
        </div>
      </div>
    </div>
  `,
});

// Register components
app.component("app-nav", createNavigation("request-detail"));
app.component("pev2", pev2.Plan);

app.mount("#app");
