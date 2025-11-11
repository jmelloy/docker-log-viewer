import { createAppHeader } from "/static/js/shared/navigation.js";
import { API } from "/static/js/shared/api.js";
import { formatSQL, escapeHtml } from "/static/js/utils.js";
import { loadTemplate } from "/static/js/shared/template-loader.js";

const template = await loadTemplate("template.html");

const { createApp } = Vue;

const app = createApp({
  data() {
    return {
      sampleQueries: [],
      servers: [],
      selectedSampleQuery: null,
      requests: [],
      allRequests: [],
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
        this.servers = await API.get("/api/servers");
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
      }
    },

    async loadAllRequests() {
      try {
        const offset = (this.currentPage - 1) * this.pageSize;
        const params = new URLSearchParams({
          limit: this.pageSize,
          offset: offset,
          search: this.searchQuery,
        });

        const response = await API.get(`/api/all-executions?${params}`);
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

    async loadRequests(sampleQueryId) {
      try {
        this.requests = await API.get(`/api/executions?request_id=${sampleQueryId}`);
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
      window.location.href = `/requests/detail/?id=${requestId}`;
    },

    async compareSelectedRequests() {
      if (this.selectedRequestIds.length !== 2) return;

      const ids = this.selectedRequestIds;

      // Fetch details for both requests
      const [detail1, detail2] = await Promise.all(ids.map((id) => API.get(`/api/executions/${id}`)));

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

    // Wrapper for global formatSQL function
    formatSQL(sql) {
      return formatSQL(sql);
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
        const payload = {
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

        await API.post("/api/requests", payload);

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

          const result = await API.post(`/api/requests/${this.selectedSampleQuery.id}/execute`, payload);

          // Reload requests to show new execution
          await this.loadAllRequests();

          // Close modal
          this.showExecuteNewModal = false;

          // Navigate to execution detail
          if (result.executionId) {
            window.location.href = `/requests/detail/?id=${result.executionId}`;
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

          const result = await API.post("/api/execute", payload);

          // Reload requests to show new execution
          await this.loadAllRequests();

          // Close modal
          this.showExecuteNewModal = false;

          // Navigate to execution detail
          if (result.executionId) {
            window.location.href = `/requests/detail/?id=${result.executionId}`;
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

  template,
});

app.component("app-header", createAppHeader("requests"));

app.mount("#app");
