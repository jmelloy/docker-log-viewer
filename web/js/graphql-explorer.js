import { createNavigation } from "./shared/navigation.js";
import { API } from "./shared/api.js";

const { createApp } = Vue;

const app = createApp({
  data() {
    return {
      servers: [],
      selectedServerId: "",
      query: "",
      operationName: "",
      variables: "{}",
      executing: false,
      result: null,
      error: null,
      executionId: null,
      showSampleQueries: false,
      sampleQueries: [],
    };
  },

  computed: {
    selectedServer() {
      if (!this.selectedServerId) return null;
      return this.servers.find((s) => s.id === parseInt(this.selectedServerId));
    },

    formattedResult() {
      if (!this.result) return "";
      // Check if result is already valid JSON string
      try {
        const parsed = JSON.parse(this.result);
        return JSON.stringify(parsed, null, 2);
      } catch (e) {
        // If not valid JSON, return as-is
        return this.result;
      }
    },

    canExecute() {
      return this.selectedServerId && this.query.trim();
    },
  },

  async mounted() {
    await this.loadServers();
    await this.loadSampleQueries();
    
    // Load example query if nothing is set
    if (!this.query) {
      this.query = `query ExampleQuery {
  # Add your GraphQL query here
}`;
    }
  },

  methods: {
    async loadServers() {
      try {
        this.servers = await API.get("/api/servers");
        // Auto-select first server if available
        if (this.servers.length > 0 && !this.selectedServerId) {
          this.selectedServerId = String(this.servers[0].id);
        }
      } catch (error) {
        console.error("Failed to load servers:", error);
        this.servers = [];
      }
    },

    async loadSampleQueries() {
      try {
        this.sampleQueries = await API.get("/api/requests");
      } catch (error) {
        console.error("Failed to load sample queries:", error);
        this.sampleQueries = [];
      }
    },

    async executeQuery() {
      if (!this.canExecute) return;

      this.executing = true;
      this.error = null;
      this.result = null;
      this.executionId = null;

      try {
        // Build request body
        const requestData = {
          query: this.query,
        };

        if (this.operationName.trim()) {
          requestData.operationName = this.operationName.trim();
        }

        // Parse variables
        if (this.variables.trim() && this.variables.trim() !== "{}") {
          try {
            requestData.variables = JSON.parse(this.variables);
          } catch (e) {
            this.error = `Invalid JSON in variables: ${e.message}`;
            this.executing = false;
            return;
          }
        }

        // Execute via API
        const payload = {
          serverId: parseInt(this.selectedServerId),
          requestData: JSON.stringify(requestData),
        };

        const response = await API.post("/api/execute", payload);

        if (response.executionId) {
          this.executionId = response.executionId;
          
          // Fetch execution details
          const detail = await API.get(`/api/executions/${response.executionId}`);
          
          if (detail.execution.error) {
            this.error = detail.execution.error;
          } else {
            this.result = detail.execution.responseBody;
          }
        } else {
          this.error = "No execution ID returned";
        }
      } catch (error) {
        console.error("Failed to execute query:", error);
        this.error = error.message;
      } finally {
        this.executing = false;
      }
    },

    async loadSampleQuery(sampleQuery) {
      try {
        const data = JSON.parse(sampleQuery.requestData);
        this.query = data.query || "";
        this.operationName = data.operationName || "";
        this.variables = data.variables
          ? JSON.stringify(data.variables, null, 2)
          : "{}";

        if (sampleQuery.serverId) {
          this.selectedServerId = String(sampleQuery.serverId);
        }

        this.showSampleQueries = false;
      } catch (e) {
        console.error("Failed to load sample query:", e);
        alert("Failed to load sample query: " + e.message);
      }
    },

    clearQuery() {
      this.query = "";
      this.operationName = "";
      this.variables = "{}";
      this.result = null;
      this.error = null;
      this.executionId = null;
    },

    viewExecutionDetail() {
      if (this.executionId) {
        window.location.href = `/request-detail.html?id=${this.executionId}`;
      }
    },

    async copyToClipboard(text) {
      try {
        await navigator.clipboard.writeText(text);
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
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem;">
            <h2 style="margin: 0;">GraphQL Explorer</h2>
            <div style="display: flex; gap: 0.5rem;">
              <button @click="showSampleQueries = !showSampleQueries" class="btn-secondary">
                {{ showSampleQueries ? 'Hide' : 'Load' }} Sample Queries
              </button>
              <button @click="clearQuery" class="btn-secondary">Clear</button>
              <button 
                @click="executeQuery" 
                :disabled="!canExecute || executing" 
                class="btn-primary"
                :style="{ opacity: !canExecute || executing ? 0.5 : 1 }">
                {{ executing ? 'Executing...' : '▶ Execute' }}
              </button>
            </div>
          </div>

          <!-- Sample Queries Panel -->
          <div v-if="showSampleQueries" class="modal-section" style="margin-bottom: 1rem;">
            <h4>Sample Queries</h4>
            <div v-if="sampleQueries.length === 0" style="color: #8b949e;">
              No sample queries available. Create one from the Requests page.
            </div>
            <div v-else style="display: grid; grid-template-columns: repeat(auto-fill, minmax(250px, 1fr)); gap: 0.75rem;">
              <div 
                v-for="sq in sampleQueries" 
                :key="sq.id"
                @click="loadSampleQuery(sq)"
                style="background: #161b22; border: 1px solid #30363d; border-radius: 4px; padding: 0.75rem; cursor: pointer; transition: border-color 0.2s;"
                @mouseover="$event.currentTarget.style.borderColor = '#58a6ff'"
                @mouseout="$event.currentTarget.style.borderColor = '#30363d'">
                <div style="font-weight: 500; margin-bottom: 0.25rem;">{{ sq.name }}</div>
                <div style="font-size: 0.75rem; color: #8b949e;">{{ sq.server?.url || 'No server' }}</div>
              </div>
            </div>
          </div>

          <!-- Configuration Section -->
          <div class="modal-section" style="margin-bottom: 1rem;">
            <div class="form-group" style="margin-bottom: 0;">
              <label for="serverSelect">Server:</label>
              <select id="serverSelect" v-model="selectedServerId" style="width: 100%; padding: 0.5rem; background: #0d1117; border: 1px solid #30363d; border-radius: 4px; color: #c9d1d9;">
                <option value="">-- Select Server --</option>
                <option v-for="server in servers" :key="server.id" :value="server.id">
                  {{ server.name }} ({{ server.url }})
                </option>
              </select>
            </div>
          </div>

          <!-- Query Editor -->
          <div class="modal-section" style="margin-bottom: 1rem;">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem;">
              <h4 style="margin: 0;">GraphQL Query</h4>
              <button @click="copyToClipboard(query)" class="btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;">📋 Copy</button>
            </div>
            <div class="form-group" style="margin-bottom: 0.5rem;">
              <label for="operationName">Operation Name (optional):</label>
              <input 
                type="text" 
                id="operationName" 
                v-model="operationName" 
                placeholder="e.g., FetchUsers"
                style="width: 100%; padding: 0.5rem; background: #0d1117; border: 1px solid #30363d; border-radius: 4px; color: #c9d1d9; font-family: monospace;" />
            </div>
            <textarea
              v-model="query"
              placeholder="Enter your GraphQL query here..."
              rows="15"
              style="width: 100%; padding: 1rem; background: #0d1117; border: 1px solid #30363d; border-radius: 4px; color: #c9d1d9; font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace; font-size: 0.875rem; resize: vertical;"
            ></textarea>
          </div>

          <!-- Variables Editor -->
          <div class="modal-section" style="margin-bottom: 1rem;">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem;">
              <h4 style="margin: 0;">Variables (JSON)</h4>
              <button @click="copyToClipboard(variables)" class="btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;">📋 Copy</button>
            </div>
            <textarea
              v-model="variables"
              placeholder='{"key": "value"}'
              rows="8"
              style="width: 100%; padding: 1rem; background: #0d1117; border: 1px solid #30363d; border-radius: 4px; color: #c9d1d9; font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace; font-size: 0.875rem; resize: vertical;"
            ></textarea>
          </div>

          <!-- Results Section -->
          <div v-if="error || result" class="modal-section">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem;">
              <h4 style="margin: 0;">{{ error ? 'Error' : 'Result' }}</h4>
              <div style="display: flex; gap: 0.5rem;">
                <button v-if="executionId" @click="viewExecutionDetail" class="btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;">
                  View Details →
                </button>
                <button v-if="result" @click="copyToClipboard(formattedResult)" class="btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;">📋 Copy</button>
              </div>
            </div>
            
            <div v-if="error" class="alert alert-danger" style="display: block; margin-bottom: 0;">
              {{ error }}
            </div>
            
            <pre v-if="result" class="json-display" style="max-height: 500px; overflow: auto;">{{ formattedResult }}</pre>
          </div>
        </main>
      </div>
    </div>
  `,
});

// Register components
app.component("app-nav", createNavigation("graphql-explorer"));

app.mount("#app");
