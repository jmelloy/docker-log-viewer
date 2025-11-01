const { createApp } = Vue;

const app = createApp({
  data() {
    return {
      requestDetail: null,
      loading: true,
      error: null,
      explainPlanData: null,
      showExplainPlanModal: false,
      pev2App: null,
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
      if (!this.requestDetail?.execution.responseBody)
        return "(no response)";
      try {
        const data = JSON.parse(this.requestDetail.execution.responseBody);
        return JSON.stringify(data, null, 2);
      } catch (e) {
        return this.requestDetail.execution.responseBody;
      }
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
        const response = await fetch(`/api/executions/${requestId}`);
        if (!response.ok) {
          throw new Error(`Failed to load request: ${response.statusText}`);
        }
        this.requestDetail = await response.json();
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

      this.showExplainPlanModal = true;

      // Need to mount PEV2 after modal is visible
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
      this.showExplainPlanModal = false;
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

        const response = await fetch("/api/explain", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify(payload),
        });

        const result = await response.json();

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
  },

  template: `
    <div class="app-container">
      <header class="app-header">
        <div style="display: flex; align-items: center; gap: 1rem">
          <h1 style="margin: 0">🔱 Logseidon</h1>
          <nav style="display: flex; gap: 1rem; align-items: center">
            <a href="/">Log Viewer</a>
            <a href="/requests.html">Request Manager</a>
            <a href="/settings.html">Settings</a>
          </nav>
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
              <h4>Request Body</h4>
              <pre class="json-display">{{ requestData }}</pre>
            </div>

            <div class="modal-section">
              <h4>Response</h4>
              <pre class="json-display">{{ responseBody }}</pre>
            </div>

            <div v-if="requestDetail.execution.error" class="modal-section">
              <h4>Error</h4>
              <pre class="error-display">{{ requestDetail.execution.error }}</pre>
            </div>

            <div v-if="requestDetail.sqlAnalysis && requestDetail.sqlAnalysis.totalQueries > 0" class="modal-section">
              <h4>SQL Analysis</h4>
              <div class="stats-grid">
                <div class="stat-item">
                  <span class="stat-label">Total Queries</span>
                  <span class="stat-value">{{ requestDetail.sqlAnalysis.totalQueries }}</span>
                </div>
                <div class="stat-item">
                  <span class="stat-label">Unique Queries</span>
                  <span class="stat-value">{{ requestDetail.sqlAnalysis.uniqueQueries }}</span>
                </div>
                <div class="stat-item">
                  <span class="stat-label">Avg Duration</span>
                  <span class="stat-value">{{ requestDetail.sqlAnalysis.avgDuration.toFixed(2) }}ms</span>
                </div>
              </div>
              <br/>

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

    <!-- EXPLAIN Plan Modal -->
    <div v-if="showExplainPlanModal" class="modal">
      <div class="modal-content explain-modal-content">
        <div class="modal-header">
          <h3>SQL Query EXPLAIN Plan (PEV2)</h3>
          <div style="display: flex; gap: 0.5rem;">
            <button @click="shareExplainPlan" class="btn-secondary" style="padding: 0.5rem 1rem;">
              📋 Share
            </button>
            <button @click="closeExplainPlanModal">✕</button>
          </div>
        </div>
        <div class="modal-body">
          <div
            v-if="explainPlanData && explainPlanData.error"
            class="alert alert-danger"
            style="display: block; margin: 1rem;">
            {{ explainPlanData.error }}
          </div>
          <div
            id="pev2ExplainApp"
            class="d-flex flex-column"
          ></div>
        </div>
      </div>
    </div>
  `,
});

app.mount("#app");
