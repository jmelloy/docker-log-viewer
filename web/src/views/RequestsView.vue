<template>
<div class="app-container">
  <app-header activePage="requests"></app-header>

  <div class="main-layout">
    <aside class="sidebar">
      <div class="section">
        <h3>Sample Queries</h3>
        <button @click="openNewSampleQueryModal" class="btn-primary">+ New Sample Query</button>
        <div class="requests-list">
          <p v-if="sampleQueries.length === 0" style="padding: 1rem; color: #6c757d; text-align: center">
            No sample queries yet
          </p>
          <div
            v-for="sq in sampleQueries"
            :key="sq.id"
            class="request-item"
            :class="{ 'selected': isSampleQuerySelected(sq.id) }"
            @click="openExecuteModalWithSampleQuery(sq)"
          >
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
        <div class="flex-between mb-1">
          <h2 class="m-0">All Requests</h2>
          <div class="flex-center">
            <button @click="openExecuteNewModal" class="btn-primary">▶ Execute Request</button>
            <button v-if="compareButtonVisible" @click="compareSelectedRequests" class="btn-primary">
              Compare Selected
            </button>
          </div>
        </div>
        <div class="flex-center mb-1">
          <input
            type="text"
            v-model="searchQuery"
            @input="handleSearchChange"
            placeholder="Search requests..."
            class="search-input-full"
          />
        </div>
        <div class="executions-list">
          <p v-if="allRequests.length === 0" class="text-muted">
            No requests executed yet. Click "Execute Request" to get started.
          </p>
          <div
            v-for="req in allRequests"
            :key="req.id"
            class="execution-item-compact"
            @click="handleExecutionClick(req, $event)"
          >
            <input
              type="checkbox"
              class="exec-compare-checkbox"
              :data-id="req.id"
              :checked="selectedRequestIds.includes(req.id)"
              @change="updateCompareButton"
            />
            <span class="exec-status" :class="getExecutionStatusClass(req)">{{ req.statusCode ?? 'EXC' }}</span>
            <span class="exec-name">{{ req.displayName }}</span>
            <span class="exec-time">{{ getExecutionTimeString(req) }}</span>
            <span class="exec-server">{{ getExecutionServerUrl(req) }}</span>
            <span class="exec-duration">{{ req.durationMs }}ms</span>
            <span class="exec-id">{{ req.requestIdHeader }}</span>
          </div>
        </div>
        <div v-if="totalPages > 1" class="pagination">
          <button @click="changePage(1)" :disabled="!hasPrevPage" class="btn-secondary">« First</button>
          <button @click="changePage(currentPage - 1)" :disabled="!hasPrevPage" class="btn-secondary">‹ Prev</button>
          <span>Page {{ currentPage }} of {{ totalPages }}</span>
          <button @click="changePage(currentPage + 1)" :disabled="!hasNextPage" class="btn-secondary">Next ›</button>
          <button @click="changePage(totalPages)" :disabled="!hasNextPage" class="btn-secondary">Last »</button>
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
        <label style="display: flex; align-items: center; gap: 0.5rem">
          <input
            type="checkbox"
            v-model="newQueryForm.createNewServer"
            @change="newQueryForm.createNewServer && (newQueryForm.serverId = '')"
          />
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
          <input type="text" id="newSampleQueryDevID" v-model="newQueryForm.devId" placeholder="dev-user-id" />
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
      <button @click="saveNewSampleQuery" class="btn-primary">Save Sample Query</button>
      <button @click="showNewSampleQueryModal = false" class="btn-secondary">Cancel</button>
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
          <option v-for="sq in sampleQueries" :key="sq.id" :value="sq">{{ getSampleQueryDisplayName(sq) }}</option>
        </select>
      </div>
      <div class="form-group">
        <label for="executeRequestData">Request Data:</label>
        <textarea
          id="executeRequestData"
          v-model="executeForm.requestDataOverride"
          placeholder="Enter or edit request data (JSON)"
          rows="10"
          style="font-family: monospace; font-size: 0.875rem"
        ></textarea>
      </div>
      <div class="form-group">
        <label for="executeServer">Server:</label>
        <select id="executeServer" v-model="executeForm.serverId" @change="selectSampleQueryForExecution">
          <option value="">-- Select Server --</option>
          <option v-for="server in servers" :key="server.id" :value="server.id">
            {{ server.name }} ({{ server.url }})
          </option>
        </select>
      </div>
      <div class="form-group">
        <label for="executeToken">Bearer Token Override (optional):</label>
        <input type="text" id="executeToken" v-model="executeForm.tokenOverride" placeholder="Override token" />
      </div>
      <div class="form-group">
        <label for="executeDevID">Dev ID Override (optional):</label>
        <input type="text" id="executeDevID" v-model="executeForm.devIdOverride" placeholder="Override dev ID" />
      </div>

      <div v-if="Object.keys(executeForm.graphqlVariables).length > 0" class="form-group" style="margin-top: 1.5rem">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem">
          <label style="margin: 0">GraphQL Variables:</label>
          <button @click="addGraphQLVariable" class="btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem">
            + Add Variable
          </button>
        </div>
        <div style="background: #0d1117; border: 1px solid #30363d; border-radius: 4px; padding: 1rem">
          <div v-for="(value, key) in executeForm.graphqlVariables" :key="key" style="margin-bottom: 0.75rem">
            <div style="display: flex; gap: 0.5rem; align-items: center; margin-bottom: 0.25rem">
              <label style="min-width: 120px; color: #79c0ff; font-family: monospace; font-size: 0.875rem"
                >{{ key }}:</label
              >
              <button
                @click="removeGraphQLVariable(key)"
                style="
                  background: #da3633;
                  color: white;
                  border: none;
                  padding: 0.25rem 0.5rem;
                  border-radius: 4px;
                  cursor: pointer;
                  font-size: 0.75rem;
                  margin-left: auto;
                "
              >
                Remove
              </button>
            </div>
            <textarea
              :value="typeof value === 'string' ? value : JSON.stringify(value, null, 2)"
              @input="updateGraphQLVariable(key, $event.target.value)"
              style="
                width: 100%;
                background: #161b22;
                border: 1px solid #30363d;
                color: #c9d1d9;
                padding: 0.5rem;
                border-radius: 4px;
                font-family: monospace;
                font-size: 0.875rem;
                min-height: 60px;
                resize: vertical;
              "
              placeholder="Enter value (JSON if object/array)"
            ></textarea>
          </div>
        </div>
      </div>
      <div v-else class="form-group" style="margin-top: 1.5rem">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem">
          <label style="margin: 0">GraphQL Variables:</label>
          <button @click="addGraphQLVariable" class="btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem">
            + Add Variable
          </button>
        </div>
        <p style="color: #8b949e; font-size: 0.875rem; margin: 0">
          No variables found in request body. Click "Add Variable" to add one.
        </p>
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
            <div>
              <strong>Executed:</strong> {{ new Date(comparisonData.detail1.execution.executedAt).toLocaleString() }}
            </div>
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
              {{ getComparisonTimeDiff() > 0 ? '+' : '' }}{{ getComparisonTimeDiff() }}ms ({{ getComparisonTimeDiff() >
              0 ? '+' : '' }}{{ getComparisonTimeDiffPercent() }}%)
            </div>
          </div>
        </div>

        <div class="comparison-column">
          <h3>Request 2</h3>
          <div class="comparison-stats">
            <div><strong>Status:</strong> {{ comparisonData.detail2.execution.statusCode }}</div>
            <div><strong>Duration:</strong> {{ comparisonData.detail2.execution.durationMs }}ms</div>
            <div><strong>Request ID:</strong> {{ comparisonData.detail2.execution.requestIdHeader }}</div>
            <div>
              <strong>Executed:</strong> {{ new Date(comparisonData.detail2.execution.executedAt).toLocaleString() }}
            </div>
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

</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { API } from '@/utils/api'
import { formatSQL as formatSQLUtil } from '@/utils/ui-utils'
import type { 
  Server,
  SampleQuery,
  ExecutedRequest,
  AllExecutionsResponse,
  ExecuteResponse,
  ExecutionDetail
} from '@/types'


export default defineComponent(// Export component definition (template will be provided by SPA loader)
{
  data() {
    return {
      sampleQueries: [] as SampleQuery[],
      servers: [] as Server[],
      selectedSampleQuery: null as SampleQuery | null,
      requests: [] as ExecutedRequest[],
      allRequests: [] as ExecutedRequest[],
      // Filtering and pagination
      searchQuery: "",
      currentPage: 1,
      pageSize: 20,
      totalRequests: 0,
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
        tokenOverride: "",
        devIdOverride: "",
        graphqlVariables: {},
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
      return this.selectedSampleQuery ? new Date(this.selectedSampleQuery.createdAt).toLocaleString() : "";
    },

    selectedSampleQueryData() {
      if (!this.selectedSampleQuery) return "";
      try {
        const data = JSON.parse(this.selectedSampleQuery.requestData);
        return JSON.stringify(data, null, 2);
      } catch (e) {
        console.error("Error parsing selected sample query data:", e);
        return this.selectedSampleQuery.requestData;
      }
    },

    selectedSampleQueryVariables() {
      if (!this.selectedSampleQuery) return null;
      try {
        const data = JSON.parse(this.selectedSampleQuery.requestData);
        return data.variables ? JSON.stringify(data.variables, null, 2) : null;
      } catch (e) {
        console.error("Error parsing selected sample query variables:", e);
        return null;
      }
    },

    totalPages() {
      return Math.ceil(this.totalRequests / this.pageSize);
    },

    hasPrevPage() {
      return this.currentPage > 1;
    },

    hasNextPage() {
      return this.currentPage < this.totalPages;
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
        this.servers = await API.get<Server[]>("/api/servers");
      } catch (error) {
        console.error("Failed to load servers:", error);
        this.servers = [];
      }
    },

    async loadSampleQueries() {
      try {
        this.sampleQueries = await API.get<SampleQuery[]>("/api/requests");
      } catch (error) {
        console.error("Failed to load sample queries:", error);
      }
    },

    async loadAllRequests() {
      try {
        const offset = (this.currentPage - 1) * this.pageSize;
        const params = new URLSearchParams({
          limit: String(this.pageSize),
          offset: String(offset),
          search: this.searchQuery,
        });

        const response = await API.get<AllExecutionsResponse>(`/api/all-executions?${params}`);
        this.allRequests = response.executions || [];
        this.totalRequests = response.total || 0;
      } catch (error) {
        console.error("Failed to load requests:", error);
        this.allRequests = [];
        this.totalRequests = 0;
      }
    },

    async changePage(page) {
      if (page < 1 || page > this.totalPages) return;
      this.currentPage = page;
      await this.loadAllRequests();
    },

    async handleSearchChange() {
      this.currentPage = 1;
      await this.loadAllRequests();
    },

    async handleFilterChange() {
      this.currentPage = 1;
      await this.loadAllRequests();
    },

    getSampleQueryDisplayName(sq) {
      // Backend now provides displayName
      return sq.displayName || sq.name || "Unknown";
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

    async loadRequests(sampleQueryId: number) {
      try {
        this.requests = await API.get<ExecutedRequest[]>(`/api/executions?request_id=${sampleQueryId}`);
      } catch (error) {
        console.error("Failed to load requests for sample query:", error);
        this.requests = [];
      }
    },

    getRequestStatusClass(req) {
      // Show error class if there's an error field, even with 200 status
      if (req.error) return "error";
      return req.statusCode >= 200 && req.statusCode < 300 ? "success" : "error";
    },

    getExecutionStatusClass(req) {
      // Show error class if there's an error field, even with 200 status
      if (req.error) return "error";
      return req.statusCode >= 200 && req.statusCode < 300 ? "success" : "error";
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
        this.selectedRequestIds = this.selectedRequestIds.filter((i) => i !== id);
      }
    },

    showRequestDetail(requestId) {
      // Use router push for SPA navigation
      window.history.pushState({}, '', `/requests/${requestId}`);
      window.dispatchEvent(new PopStateEvent('popstate'));
    },

    async compareSelectedRequests() {
      if (this.selectedRequestIds.length !== 2) return;

      const ids = this.selectedRequestIds;

      // Fetch details for both requests
      const [detail1, detail2] = await Promise.all(ids.map((id) => API.get<ExecutionDetail>(`/api/executions/${id}`)));

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

    // SQL formatter
    formatSQL(sql) {
      return formatSQLUtil(sql);
    },

    openNewSampleQueryModal() {
      // Reset form
      this.newQueryForm = {
        name: "",
        requestData: "",
        serverId: "",
        createNewServer: false,
        url: "",
        bearerToken: "",
        devId: "",
      };
      this.showNewSampleQueryModal = true;
    },

    openExecuteNewModal() {
      // Reset form
      this.selectedSampleQuery = null;
      this.executeForm = {
        serverId: "",
        requestDataOverride: "",
        tokenOverride: "",
        devIdOverride: "",
        graphqlVariables: {},
      };
      this.showExecuteNewModal = true;
    },

    openExecuteModalWithSampleQuery(sq) {
      this.selectedSampleQuery = sq;
      // Find the server to get token and dev ID
      const server = this.servers.find((s) => s.id === sq.serverId);
      // Pre-populate form with sample query data
      this.executeForm = {
        serverId: sq.serverId ? String(sq.serverId) : "",
        requestDataOverride: sq.requestData || "",
        tokenOverride: server?.bearerToken || "",
        devIdOverride: server?.devId || "",
        graphqlVariables: {},
      };
      // Parse GraphQL variables
      this.parseGraphQLVariables();
      this.showExecuteNewModal = true;
    },

    async saveNewSampleQuery() {
      try {
        const payload: { 
          name: string; 
          requestData: string; 
          serverId?: number; 
          url?: string; 
          bearerToken?: string; 
          devId?: string;
        } = {
          name: this.newQueryForm.name,
          requestData: this.newQueryForm.requestData,
        };

        // If using existing server
        if (this.newQueryForm.serverId && !this.newQueryForm.createNewServer) {
          payload.serverId = parseInt(this.newQueryForm.serverId);
        } else if (this.newQueryForm.createNewServer && this.newQueryForm.url) {
          // Creating new server
          payload.url = this.newQueryForm.url;
          payload.bearerToken = this.newQueryForm.bearerToken;
          payload.devId = this.newQueryForm.devId;
        }

        await API.post<{ id: number }>("/api/requests", payload);

        // Reload sample queries
        await this.loadSampleQueries();

        // Close modal
        this.showNewSampleQueryModal = false;

        // Reset form
        this.newQueryForm = {
          name: "",
          requestData: "",
          serverId: "",
          createNewServer: false,
          url: "",
          bearerToken: "",
          devId: "",
        };
      } catch (error) {
        console.error("Failed to save sample query:", error);
        alert(`Failed to save sample query: ${error.message}`);
      }
    },

    selectSampleQueryForExecution() {
      if (this.selectedSampleQuery) {
        // Pre-populate request data from selected sample query
        this.executeForm.requestDataOverride = this.selectedSampleQuery.requestData || "";

        // Pre-populate server if available
        if (this.selectedSampleQuery.serverId) {
          this.executeForm.serverId = String(this.selectedSampleQuery.serverId);
          // Get token and dev ID from server
          this.updateServerDefaults();
        }
        // Parse GraphQL variables
        this.parseGraphQLVariables();
      } else if (this.executeForm.serverId) {
        // Server was selected but no sample query - update defaults
        this.updateServerDefaults();
      }
    },

    updateServerDefaults() {
      if (this.executeForm.serverId) {
        const server = this.servers.find((s) => s.id === parseInt(this.executeForm.serverId));
        if (server) {
          // Always set defaults from server (user can still override)
          this.executeForm.tokenOverride = server.bearerToken || "";
          this.executeForm.devIdOverride = server.devId || "";
        }
      }
    },

    async executeSelectedQuery() {
      try {
        // Determine request data to use
        let requestData = this.executeForm.requestDataOverride;
        if (!requestData && this.selectedSampleQuery) {
          requestData = this.selectedSampleQuery.requestData;
        }

        if (!requestData) {
          alert("Please provide request data");
          return;
        }

        // Determine server
        let serverId = null;
        if (this.executeForm.serverId) {
          serverId = parseInt(this.executeForm.serverId);
        } else if (this.selectedSampleQuery && this.selectedSampleQuery.serverId) {
          serverId = parseInt(this.selectedSampleQuery.serverId);
        }

        // If we have a sample query ID, use the execute endpoint
        if (this.selectedSampleQuery && this.selectedSampleQuery.id) {
          const payload = {
            serverId: serverId || undefined,
            bearerTokenOverride: this.executeForm.tokenOverride || undefined,
            devIdOverride: this.executeForm.devIdOverride || undefined,
            requestDataOverride: requestData,
          };

          const result = await API.post<ExecuteResponse>(`/api/requests/${this.selectedSampleQuery.id}/execute`, payload);

          // Reload requests to show new execution
          await this.loadAllRequests();

          // Close modal
          this.showExecuteNewModal = false;

          // Navigate to execution detail
          if (result.executionId) {
            window.history.pushState({}, '', `/requests/${result.executionId}`); window.dispatchEvent(new PopStateEvent('popstate'));
          }
        } else {
          // No sample query - execute directly using /api/execute endpoint
          if (!serverId) {
            alert("Please select a server");
            return;
          }

          const payload = {
            serverId: serverId,
            requestData: requestData,
            bearerTokenOverride: this.executeForm.tokenOverride || undefined,
            devIdOverride: this.executeForm.devIdOverride || undefined,
          };

          const result = await API.post<ExecuteResponse>("/api/execute", payload);

          // Reload requests to show new execution
          await this.loadAllRequests();

          // Close modal
          this.showExecuteNewModal = false;

          // Navigate to execution detail
          if (result.executionId) {
            window.history.pushState({}, '', `/requests/${result.executionId}`); window.dispatchEvent(new PopStateEvent('popstate'));
          }
        }
      } catch (error) {
        console.error("Failed to execute request:", error);
        alert(`Failed to execute request: ${error.message}`);
      }
    },

    parseGraphQLVariables() {
      try {
        const requestData = this.executeForm.requestDataOverride || this.selectedSampleQuery?.requestData;
        if (!requestData) return;

        let parsed;
        if (typeof requestData === "string") {
          parsed = JSON.parse(requestData);
        } else {
          parsed = requestData;
        }

        // Look for variables in GraphQL format
        if (parsed.variables && typeof parsed.variables === "object") {
          // Deep clone to avoid reference issues
          this.executeForm.graphqlVariables = JSON.parse(JSON.stringify(parsed.variables));
        } else if (Array.isArray(parsed)) {
          // Handle array of requests - look for variables in each
          const allVariables = {};
          parsed.forEach((item) => {
            if (item.variables && typeof item.variables === "object") {
              Object.assign(allVariables, item.variables);
            }
          });
          if (Object.keys(allVariables).length > 0) {
            this.executeForm.graphqlVariables = allVariables;
          }
        }
      } catch (error) {
        console.warn("Failed to parse GraphQL variables:", error);
        this.executeForm.graphqlVariables = {};
      }
    },

    updateGraphQLVariable(key, value) {
      // Try to parse as JSON if it looks like JSON, otherwise use as string
      let parsedValue = value;
      try {
        if (value.trim().startsWith("{") || value.trim().startsWith("[")) {
          parsedValue = JSON.parse(value);
        } else if (value.trim() === "true" || value.trim() === "false") {
          parsedValue = value.trim() === "true";
        } else if (!isNaN(value) && value.trim() !== "") {
          parsedValue = Number(value);
        }
      } catch (e) {
        console.error("Error parsing GraphQL variable:", e);
        parsedValue = value;
      }

      this.executeForm.graphqlVariables[key] = parsedValue;
      this.updateRequestDataWithVariables();
    },

    removeGraphQLVariable(key) {
      delete this.executeForm.graphqlVariables[key];
      // Create new object to trigger reactivity
      this.executeForm.graphqlVariables = {
        ...this.executeForm.graphqlVariables,
      };
      this.updateRequestDataWithVariables();
    },

    addGraphQLVariable() {
      const key = prompt("Enter variable name:");
      if (key && key.trim()) {
        this.executeForm.graphqlVariables[key.trim()] = "";
        // Create new object to trigger reactivity
        this.executeForm.graphqlVariables = {
          ...this.executeForm.graphqlVariables,
        };
      }
    },

    updateRequestDataWithVariables() {
      try {
        // Use the current requestDataOverride as the base, or fall back to selected sample query
        let baseData = this.executeForm.requestDataOverride;
        if (!baseData && this.selectedSampleQuery) {
          baseData = this.selectedSampleQuery.requestData;
        }
        if (!baseData) return;

        let parsed;
        if (typeof baseData === "string") {
          parsed = JSON.parse(baseData);
        } else {
          parsed = baseData;
        }

        // Update variables
        if (Array.isArray(parsed)) {
          // If it's an array, update variables in each item
          parsed.forEach((item) => {
            if (item.variables !== undefined) {
              item.variables = this.executeForm.graphqlVariables;
            }
          });
        } else {
          // Single object
          parsed.variables = this.executeForm.graphqlVariables;
        }

        // Update the request data override
        this.executeForm.requestDataOverride = JSON.stringify(parsed, null, 2);
      } catch (error) {
        console.warn("Failed to update request data with variables:", error);
      }
    },
  },
})
</script>
