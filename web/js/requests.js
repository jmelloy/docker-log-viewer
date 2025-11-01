const { createApp } = Vue;

const app = createApp({
  data() {
    return {
      sampleQueries: [],
      servers: [],
      selectedSampleQuery: null,
      requests: [],
      allRequests: [],
      // Modal visibility
      showNewSampleQueryModal: false,
      showExecuteQueryModal: false,
      showExecuteNewModal: false,
      showComparisonModal: false,
      // Form data
      newQueryForm: {
        name: "",
        requestData: "",
        serverId: "",
        createNewServer: false,
        url: "",
        bearerToken: "",
        devId: "",
      },
      executeForm: {
        serverId: "",
        requestDataOverride: "",
        urlOverride: "",
        tokenOverride: "",
        devIdOverride: "",
      },
      // Selected data
      comparisonData: null,
      selectedRequestIds: [],
    };
  },

  computed: {
    showEmptyState() {
      return !this.selectedSampleQuery;
    },

    showSampleQueryDetail() {
      return this.selectedSampleQuery !== null;
    },

    showNewServerFields() {
      return this.newQueryForm.createNewServer;
    },

    compareButtonVisible() {
      return this.selectedRequestIds.length === 2;
    },

    selectedSampleQueryName() {
      return this.selectedSampleQuery?.name || "";
    },

    selectedSampleQueryURL() {
      return this.selectedSampleQuery?.server?.url || "(no server configured)";
    },

    selectedSampleQueryCreated() {
      return this.selectedSampleQuery
        ? new Date(this.selectedSampleQuery.createdAt).toLocaleString()
        : "";
    },

    selectedSampleQueryData() {
      if (!this.selectedSampleQuery) return "";
      try {
        const data = JSON.parse(this.selectedSampleQuery.requestData);
        return JSON.stringify(data, null, 2);
      } catch (e) {
        return this.selectedSampleQuery.requestData;
      }
    },

    selectedSampleQueryVariables() {
      if (!this.selectedSampleQuery) return null;
      try {
        const data = JSON.parse(this.selectedSampleQuery.requestData);
        return data.variables ? JSON.stringify(data.variables, null, 2) : null;
      } catch (e) {
        return null;
      }
    },

  },

  async mounted() {
    await this.loadServers();
    await this.loadSampleQueries();
    await this.loadAllRequests();
  },

  methods: {
    async loadServers() {
      try {
        const response = await fetch("/api/servers");
        this.servers = await response.json();
      } catch (error) {
        console.error("Failed to load servers:", error);
        this.servers = [];
      }
    },

    async loadSampleQueries() {
      try {
        const response = await fetch("/api/requests");
        this.sampleQueries = await response.json();
      } catch (error) {
        console.error("Failed to load sample queries:", error);
      }
    },

    async loadAllRequests() {
      try {
        const response = await fetch("/api/all-executions");
        const executions = await response.json();

        // Fetch additional details for each request to get sample query name
        this.allRequests = await Promise.all(
          executions.slice(0, 20).map(async (req) => {
            try {
              const response = await fetch(`/api/executions/${req.id}`);
              const detail = await response.json();

              // Use sample query name if available, otherwise operation name from request body
              let displayName = "Unknown";
              if (detail.request && detail.request.name) {
                displayName = detail.request.name;
              } else if (detail.execution.requestBody) {
                try {
                  const requestData = JSON.parse(detail.execution.requestBody);
                  displayName = requestData.operationName || "Unknown";
                } catch (e) {
                  console.warn("Failed to parse request body:", e);
                }
              }

              return { ...req, displayName };
            } catch (e) {
              return { ...req, displayName: "Unknown" };
            }
          })
        );
      } catch (error) {
        console.error("Failed to load requests:", error);
      }
    },

    getSampleQueryDisplayName(sq) {
      try {
        const data = JSON.parse(sq.requestData);
        return sq.name || data.operationName;
      } catch (e) {
        return sq.name;
      }
    },

    isSampleQuerySelected(sqId) {
      return this.selectedSampleQuery?.id === sqId;
    },

    async selectSampleQuery(id) {
      const sampleQuery = this.sampleQueries.find((sq) => sq.id === id);
      if (!sampleQuery) return;

      this.selectedSampleQuery = sampleQuery;
      await this.loadRequests(id);
    },

    async loadRequests(sampleQueryId) {
      try {
        const response = await fetch(
          `/api/executions?request_id=${sampleQueryId}`
        );
        this.requests = await response.json();
      } catch (error) {
        console.error("Failed to load requests for sample query:", error);
        this.requests = [];
      }
    },

    getRequestStatusClass(req) {
      return req.statusCode >= 200 && req.statusCode < 300
        ? "success"
        : "error";
    },

    getExecutionStatusClass(req) {
      return req.statusCode >= 200 && req.statusCode < 300
        ? "success"
        : "error";
    },

    getExecutionTimeString(req) {
      return new Date(req.executedAt).toLocaleTimeString();
    },

    getExecutionServerUrl(req) {
      return req.server ? req.server.name || req.server.url : "N/A";
    },

    handleExecutionClick(req, event) {
      if (event.target.type !== "checkbox") {
        this.showRequestDetail(req.id);
      }
    },

    updateCompareButton(event) {
      const id = parseInt(event.target.dataset.id);
      if (event.target.checked) {
        if (!this.selectedRequestIds.includes(id)) {
          this.selectedRequestIds.push(id);
        }
      } else {
        this.selectedRequestIds = this.selectedRequestIds.filter(
          (i) => i !== id
        );
      }
    },

    showRequestDetail(requestId) {
      window.location.href = `/request-detail.html?id=${requestId}`;
    },

    async compareSelectedRequests() {
      if (this.selectedRequestIds.length !== 2) return;

      const ids = this.selectedRequestIds;

      // Fetch details for both requests
      const [detail1, detail2] = await Promise.all(
        ids.map(async (id) => {
          const response = await fetch(`/api/executions/${id}`);
          return await response.json();
        })
      );

      this.comparisonData = { detail1, detail2 };
      this.showComparisonModal = true;
    },

    getComparisonTimeDiff() {
      if (!this.comparisonData) return 0;
      const { detail1, detail2 } = this.comparisonData;
      return detail2.execution.durationMs - detail1.execution.durationMs;
    },

    getComparisonTimeDiffPercent() {
      if (!this.comparisonData) return "0.0";
      const diff = this.getComparisonTimeDiff();
      const { detail1 } = this.comparisonData;
      return ((diff / detail1.execution.durationMs) * 100).toFixed(1);
    },

    getComparisonTimeDiffClass() {
      return this.getComparisonTimeDiff() > 0 ? "diff-slower" : "diff-faster";
    },

    escapeHtml(text) {
      const div = document.createElement("div");
      div.textContent = text;
      return div.innerHTML;
    },
  },


  template: `
    <div class="app-container">
      <header class="app-header">
        <div style="display: flex; align-items: center; gap: 1rem">
          <h1 style="margin: 0">🔱 Logseidon</h1>
          <nav style="display: flex; gap: 1rem; align-items: center">
            <a href="/">Log Viewer</a>
            <a href="/requests.html" class="active">Request Manager</a>
            <a href="/settings.html">Settings</a>
          </nav>
        </div>
      </header>

      <div class="main-layout">
        <aside class="sidebar">
          <div class="section">
            <h3>Sample Queries</h3>
            <button @click="openNewSampleQueryModal" class="btn-primary">
              + New Sample Query
            </button>
            <div class="requests-list">
              <p v-if="sampleQueries.length === 0" style="padding: 1rem; color: #6c757d; text-align: center;">No sample queries yet</p>
              <div v-for="sq in sampleQueries" 
                   :key="sq.id"
                   class="request-item"
                   :class="{ 'selected': isSampleQuerySelected(sq.id) }"
                   @click="openExecuteModalWithSampleQuery(sq)">
                <div class="request-item-name">{{ getSampleQueryDisplayName(sq) }}</div>
                <div class="request-item-meta">
                  <span>{{ sq.server?.url || 'No server' }}</span>
                </div>
              </div>
            </div>
          </div>
        </aside>

        <main class="content">
          <div class="empty-state">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
              <h2 style="margin: 0">All Requests</h2>
              <div style="display: flex; gap: 0.5rem;">
                <button @click="openExecuteNewModal" class="btn-primary">
                  ▶ Execute Request
                </button>
                <button
                  v-if="compareButtonVisible"
                  @click="compareSelectedRequests"
                  class="btn-primary">
                  Compare Selected
                </button>
              </div>
            </div>
            <div class="executions-list">
              <p v-if="allRequests.length === 0" style="color: #6c757d;">No requests executed yet. Click "Execute Request" to get started.</p>
              <div v-for="req in allRequests" 
                   :key="req.id"
                   class="execution-item-compact"
                   @click="handleExecutionClick(req, $event)">
                <input type="checkbox" class="exec-compare-checkbox" :data-id="req.id" :checked="selectedRequestIds.includes(req.id)" @change="updateCompareButton">
                <span class="exec-status" :class="getExecutionStatusClass(req)">{{ req.statusCode || 'ERR' }}</span>
                <span class="exec-name">{{ req.displayName }}</span>
                <span class="exec-time">{{ getExecutionTimeString(req) }}</span>
                <span class="exec-server">{{ getExecutionServerUrl(req) }}</span>
                <span class="exec-duration">{{ req.durationMs }}ms</span>
                <span class="exec-id">{{ req.requestIdHeader }}</span>
              </div>
            </div>
          </div>
        </main>
      </div>
    </div>

    <!-- New Sample Query Modal -->
    <div v-if="showNewSampleQueryModal" class="modal">
      <div class="modal-content">
        <div class="modal-header">
          <h3>New Sample Query</h3>
          <button @click="showNewSampleQueryModal = false">✕</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label for="newSampleQueryName">Name:</label>
            <input
              type="text"
              id="newSampleQueryName"
              v-model="newQueryForm.name"
              placeholder="e.g., FetchUsers"
              required
            />
          </div>
          <div class="form-group">
            <label for="newSampleQueryServer">Server (optional):</label>
            <select id="newSampleQueryServer" v-model="newQueryForm.serverId" :disabled="newQueryForm.createNewServer">
              <option value="">-- No Server --</option>
              <option v-for="server in servers" :key="server.id" :value="server.id">
                {{ server.name }} ({{ server.url }})
              </option>
            </select>
          </div>
          <div class="form-group">
            <label style="display: flex; align-items: center; gap: 0.5rem;">
              <input type="checkbox" v-model="newQueryForm.createNewServer" @change="newQueryForm.createNewServer && (newQueryForm.serverId = '')">
              <span>Create New Server</span>
            </label>
          </div>
          <div v-if="showNewServerFields">
            <div class="form-group">
              <label for="newSampleQueryURL">URL (optional):</label>
              <input
                type="text"
                id="newSampleQueryURL"
                v-model="newQueryForm.url"
                placeholder="https://api.example.com/graphql"
              />
            </div>
            <div class="form-group">
              <label for="newSampleQueryToken">Bearer Token (optional):</label>
              <input
                type="text"
                id="newSampleQueryToken"
                v-model="newQueryForm.bearerToken"
                placeholder="your-token-here"
              />
            </div>
            <div class="form-group">
              <label for="newSampleQueryDevID">Dev ID (optional):</label>
              <input
                type="text"
                id="newSampleQueryDevID"
                v-model="newQueryForm.devId"
                placeholder="dev-user-id"
              />
            </div>
          </div>
          <div class="form-group">
            <label for="newSampleQueryData">Query Data (JSON):</label>
            <textarea
              id="newSampleQueryData"
              v-model="newQueryForm.requestData"
              rows="15"
              placeholder='{"query": "{ users { id name } }"}'
            ></textarea>
          </div>
        </div>
        <div class="modal-footer">
          <button @click="saveNewSampleQuery" class="btn-primary">
            Save Sample Query
          </button>
          <button @click="showNewSampleQueryModal = false" class="btn-secondary">
            Cancel
          </button>
        </div>
      </div>
    </div>

    <!-- Execute New Request Modal -->
    <div v-if="showExecuteNewModal" class="modal">
      <div class="modal-content">
        <div class="modal-header">
          <h3>Execute Request</h3>
          <button @click="showExecuteNewModal = false">✕</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label for="executeNewQuery">Sample Query:</label>
            <select id="executeNewQuery" v-model="selectedSampleQuery" @change="selectSampleQueryForExecution">
              <option :value="null">-- Select Sample Query --</option>
              <option v-for="sq in sampleQueries" :key="sq.id" :value="sq">
                {{ getSampleQueryDisplayName(sq) }}
              </option>
            </select>
          </div>
          <div class="form-group">
            <label for="executeRequestData">Request Data:</label>
            <textarea 
              id="executeRequestData" 
              v-model="executeForm.requestDataOverride" 
              placeholder="Enter or edit request data (JSON)"
              rows="10"
              style="font-family: monospace; font-size: 0.875rem;"
            ></textarea>
          </div>
          <div class="form-group">
            <label for="executeServer">Server:</label>
            <select id="executeServer" v-model="executeForm.serverId">
              <option value="">-- Select Server --</option>
              <option v-for="server in servers" :key="server.id" :value="server.id">
                {{ server.name }} ({{ server.url }})
              </option>
            </select>
          </div>
          <div class="form-group">
            <label for="executeUrl">URL Override (optional):</label>
            <input type="text" id="executeUrl" v-model="executeForm.urlOverride" placeholder="Override server URL" />
          </div>
          <div class="form-group">
            <label for="executeToken">Bearer Token Override (optional):</label>
            <input type="text" id="executeToken" v-model="executeForm.tokenOverride" placeholder="Override token" />
          </div>
          <div class="form-group">
            <label for="executeDevID">Dev ID Override (optional):</label>
            <input
              type="text"
              id="executeDevID"
              v-model="executeForm.devIdOverride"
              placeholder="Override dev ID"
            />
          </div>
        </div>
        <div class="modal-footer">
          <button @click="executeSelectedQuery" class="btn-primary">Execute</button>
          <button @click="showExecuteNewModal = false" class="btn-secondary">Cancel</button>
        </div>
      </div>
    </div>

    <!-- Request Comparison Modal -->
    <div v-if="showComparisonModal && comparisonData" class="modal">
      <div class="modal-content" style="max-width: 1400px">
        <div class="modal-header">
          <h3>Request Comparison</h3>
          <button @click="showComparisonModal = false">✕</button>
        </div>
        <div class="modal-body">
          <div class="comparison-grid">
            <div class="comparison-column">
              <h3>Request 1</h3>
              <div class="comparison-stats">
                <div><strong>Status:</strong> {{ comparisonData.detail1.execution.statusCode }}</div>
                <div><strong>Duration:</strong> {{ comparisonData.detail1.execution.durationMs }}ms</div>
                <div><strong>Request ID:</strong> {{ comparisonData.detail1.execution.requestIdHeader }}</div>
                <div><strong>Executed:</strong> {{ new Date(comparisonData.detail1.execution.executedAt).toLocaleString() }}</div>
              </div>
              <div class="comparison-section">
                <h4>Response</h4>
                <pre class="json-display">{{ comparisonData.detail1.execution.responseBody || 'No response' }}</pre>
              </div>
              <div class="comparison-section">
                <h4>SQL Queries ({{ comparisonData.detail1.sqlQueries.length }})</h4>
                <div class="comparison-queries">
                  <div v-for="(q, idx) in comparisonData.detail1.sqlQueries" :key="idx" class="comparison-query">
                    <div><strong>{{ q.tableName }}</strong> - {{ q.durationMs.toFixed(2) }}ms</div>
                    <div class="sql-query-text">{{ formatSQL(q.query) }}</div>
                  </div>
                </div>
              </div>
            </div>

            <div class="comparison-divider">
              <div class="comparison-diff">
                <div>Time Difference</div>
                <div :class="getComparisonTimeDiffClass()">
                  {{ getComparisonTimeDiff() > 0 ? '+' : '' }}{{ getComparisonTimeDiff() }}ms 
                  ({{ getComparisonTimeDiff() > 0 ? '+' : '' }}{{ getComparisonTimeDiffPercent() }}%)
                </div>
              </div>
            </div>

            <div class="comparison-column">
              <h3>Request 2</h3>
              <div class="comparison-stats">
                <div><strong>Status:</strong> {{ comparisonData.detail2.execution.statusCode }}</div>
                <div><strong>Duration:</strong> {{ comparisonData.detail2.execution.durationMs }}ms</div>
                <div><strong>Request ID:</strong> {{ comparisonData.detail2.execution.requestIdHeader }}</div>
                <div><strong>Executed:</strong> {{ new Date(comparisonData.detail2.execution.executedAt).toLocaleString() }}</div>
              </div>
              <div class="comparison-section">
                <h4>Response</h4>
                <pre class="json-display">{{ comparisonData.detail2.execution.responseBody || 'No response' }}</pre>
              </div>
              <div class="comparison-section">
                <h4>SQL Queries ({{ comparisonData.detail2.sqlQueries.length }})</h4>
                <div class="comparison-queries">
                  <div v-for="(q, idx) in comparisonData.detail2.sqlQueries" :key="idx" class="comparison-query">
                    <div><strong>{{ q.tableName }}</strong> - {{ q.durationMs.toFixed(2) }}ms</div>
                    <div class="sql-query-text">{{ formatSQL(q.query) }}</div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  `,
});

app.mount("#app");
