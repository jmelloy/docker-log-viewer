import { createAppHeader } from "/static/js/shared/navigation.js";
import { API } from "/static/js/shared/api.js";
import { formatSQL } from "/static/js/utils.js";
import { createLogStreamComponent } from "/static/js/shared/log-stream-component.js";
import { loadTemplate } from "/static/js/shared/template-loader.js";

const pev2Template = await loadTemplate("pev2-template.html");
const mainTemplate = await loadTemplate("template.html");

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
      showExecuteModal: false,
      executeForm: {
        tokenOverride: "",
        devIdOverride: "",
        requestDataOverride: "",
        graphqlVariables: {},
      },
      servers: [],
      showLiveLogStream: false, // Toggle between saved logs and live stream
      refreshTimer: null, // Timer for auto-refreshing recent requests
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
      if (!this.requestDetail || !this.requestDetail.execution.requestIdHeader) {
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

    // Cache parsed request body to avoid multiple JSON.parse calls
    parsedRequestBody() {
      if (!this.requestDetail?.execution.requestBody) return null;
      try {
        return JSON.parse(this.requestDetail.execution.requestBody);
      } catch (e) {
        console.error("Error parsing request body:", e);
        return null;
      }
    },

    isGraphQLRequest() {
      const data = this.parsedRequestBody;
      return !!(data && (data.query || data.operationName));
    },

    graphqlQuery() {
      if (!this.isGraphQLRequest) return null;
      return this.parsedRequestBody.query || "";
    },

    graphqlOperationName() {
      if (!this.isGraphQLRequest) return null;
      return this.parsedRequestBody.operationName || null;
    },

    graphqlVariables() {
      if (!this.isGraphQLRequest) return null;
      const variables = this.parsedRequestBody.variables;
      return variables ? JSON.stringify(variables, null, 2) : null;
    },

    requestData() {
      if (!this.requestDetail?.execution.requestBody) return "(no request data)";
      const data = this.parsedRequestBody;
      if (data) {
        return JSON.stringify(data, null, 2);
      }
      return this.requestDetail.execution.requestBody;
    },

    responseBody() {
      if (!this.requestDetail?.execution.responseBody) return "(no response)";
      try {
        const data = JSON.parse(this.requestDetail.execution.responseBody);
        return JSON.stringify(data, null, 2);
      } catch (e) {
        console.error("Error parsing response body:", e);
        return this.requestDetail.execution.responseBody;
      }
    },

    // Calculate the age of the request in minutes
    requestAgeMinutes() {
      if (!this.requestDetail?.execution.executedAt) return Infinity;
      const executedAt = new Date(this.requestDetail.execution.executedAt);
      const now = new Date();
      return (now - executedAt) / 1000 / 60; // Convert milliseconds to minutes
    },

    sqlAnalysisData() {
      if (!this.requestDetail?.sqlQueries || this.requestDetail.sqlQueries.length === 0) {
        return null;
      }

      const queries = this.requestDetail.sqlQueries.map((q) => ({
        query: q.query,
        duration: q.durationMs,
        table: q.tableName || "unknown",
        operation: q.operation || "SELECT",
        rows: q.rows || 0,
        variables: q.variables ? (typeof q.variables === "string" ? JSON.parse(q.variables) : q.variables) : {},
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

      const slowestQueries = [...queries].sort((a, b) => b.duration - a.duration).slice(0, 5);

      const frequentQueries = Object.entries(queryGroups)
        .map(([normalized, data]) => ({
          normalized,
          count: data.count,
          example: data.queries[0],
          avgDuration: data.queries.reduce((sum, q) => sum + q.duration, 0) / data.count,
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
    await this.loadServers();
    this.applySyntaxHighlighting();
  },

  updated() {
    // Apply syntax highlighting after DOM updates
    this.$nextTick(() => {
      this.applySyntaxHighlighting();
    });
  },

  beforeUnmount() {
    // Clean up refresh timer when component is destroyed
    if (this.refreshTimer) {
      clearTimeout(this.refreshTimer);
      this.refreshTimer = null;
    }
  },

  methods: {
    applySyntaxHighlighting() {
      // Only apply if hljs is available
      if (typeof hljs === "undefined") return;

      // Highlight JSON in request and response bodies
      document.querySelectorAll(".json-display:not(.hljs)").forEach((block) => {
        try {
          const text = block.textContent.trim();
          if (text.startsWith("{") || text.startsWith("[")) {
            const highlighted = hljs.highlight(text, { language: "json" });
            block.innerHTML = highlighted.value;
            block.classList.add("hljs");
          }
        } catch (e) {
          console.error("Error highlighting JSON:", e);
          // If highlighting fails, leave as is
        }
      });

      // Highlight GraphQL queries
      document.querySelectorAll(".graphql-query:not(.hljs)").forEach((block) => {
        try {
          const text = block.textContent.trim();
          const highlighted = hljs.highlight(text, { language: "graphql" });
          block.innerHTML = highlighted.value;
          block.classList.add("hljs");
        } catch (e) {
          console.error("Error highlighting GraphQL query:", e);
          // If highlighting fails, leave as is
        }
      });

      // Highlight SQL queries
      document.querySelectorAll(".sql-query-text:not(.hljs)").forEach((block) => {
        try {
          const text = block.textContent;
          const highlighted = hljs.highlight(text, { language: "sql" });
          block.innerHTML = highlighted.value;
          block.classList.add("hljs");
        } catch (e) {
          console.error("Error highlighting SQL query:", e);
          // If highlighting fails, leave as is
        }
      });
      document.querySelectorAll(".sql-query-text:not(.hljs)").forEach((block) => {
        try {
          const text = block.textContent;
          const highlighted = hljs.highlight(text, { language: "sql" });
          block.innerHTML = highlighted.value;
          block.classList.add("hljs");
        } catch (e) {
          console.error("Error highlighting SQL query:", e);
          // If highlighting fails, leave as is
        }
      });
    },
    async loadRequestDetail(requestId) {
      try {
        this.requestDetail = await API.get(`/api/executions/${requestId}`);
        this.loading = false;

        // Default to live stream if request is less than 3 minutes old
        const ageMinutes = this.requestAgeMinutes;
        if (ageMinutes < 3) {
          this.showLiveLogStream = true;
          console.log(`Request is ${ageMinutes.toFixed(1)} minutes old - defaulting to live stream`);
        }

        // Set up auto-refresh timer if request is less than 1 minute old
        if (ageMinutes < 1) {
          console.log(`Request is ${ageMinutes.toFixed(1)} minutes old - setting up 30s refresh timer`);
          this.setupRefreshTimer(requestId);
        }
      } catch (error) {
        console.error("Failed to load request detail:", error);
        this.error = error.message;
        this.loading = false;
      }
    },

    setupRefreshTimer(requestId) {
      // Clear any existing timer
      if (this.refreshTimer) {
        clearTimeout(this.refreshTimer);
      }

      // Set up new timer to refresh in 30 seconds
      this.refreshTimer = setTimeout(async () => {
        console.log("Auto-refreshing request details...");
        try {
          this.requestDetail = await API.get(`/api/executions/${requestId}`);

          // Check if we should continue refreshing
          const ageMinutes = this.requestAgeMinutes;
          if (ageMinutes < 1) {
            // Still less than 1 minute old, refresh again
            this.setupRefreshTimer(requestId);
          } else {
            console.log("Request is now over 1 minute old - stopping auto-refresh");
          }
        } catch (error) {
          console.error("Failed to auto-refresh request details:", error);
        }
      }, 10000); // 10 seconds
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
        this.runExplain(query.query, variables, this.requestDetail?.server?.defaultDatabase?.connectionString);
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
          template: pev2Template,
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
        planInput.value = JSON.stringify(this.explainPlanData.planData, null, 2);
        form.appendChild(planInput);

        const queryInput = document.createElement("input");
        queryInput.type = "hidden";
        queryInput.name = "query";
        queryInput.value = this.formatSQL(this.explainPlanData.query) || this.explainPlanData.query;
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

    async loadServers() {
      try {
        this.servers = await API.get("/api/servers");
      } catch (error) {
        console.error("Failed to load servers:", error);
      }
    },

    openExecuteModal() {
      // Pre-populate with current token and dev ID
      const server = this.requestDetail?.server || this.requestDetail?.execution?.server;
      this.executeForm = {
        tokenOverride: server?.bearerToken || "",
        devIdOverride: server?.devId || "",
        requestDataOverride: this.requestDetail?.execution?.requestBody || "",
        graphqlVariables: {},
      };

      // Parse GraphQL variables from request body
      this.parseGraphQLVariables();

      this.showExecuteModal = true;
    },

    parseGraphQLVariables() {
      try {
        const requestBody = this.requestDetail?.execution?.requestBody;
        if (!requestBody) return;

        let parsed;
        if (typeof requestBody === "string") {
          parsed = JSON.parse(requestBody);
        } else {
          parsed = requestBody;
        }

        // Look for variables in GraphQL format
        // GraphQL requests typically have: { query: "...", variables: {...} }
        if (parsed.variables && typeof parsed.variables === "object") {
          // Deep clone to avoid reference issues
          this.executeForm.graphqlVariables = JSON.parse(JSON.stringify(parsed.variables));
        } else if (Array.isArray(parsed)) {
          // Handle array of requests - look for variables in each
          const allVariables = {};
          parsed.forEach((item, _) => {
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
        // Not valid JSON, use as string
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
        // Use the current requestDataOverride as the base, or fall back to original
        let baseData = this.executeForm.requestDataOverride;
        if (!baseData) {
          baseData = this.requestDetail?.execution?.requestBody;
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

    async executeAgain() {
      try {
        const requestBody = this.requestDetail?.execution?.requestBody;
        if (!requestBody) {
          alert("No request data available");
          return;
        }

        // Determine server ID
        const server = this.requestDetail?.server || this.requestDetail?.execution?.server;
        const serverId = server?.id || this.requestDetail?.execution?.serverId;

        if (!serverId) {
          alert("No server found for this request");
          return;
        }

        const payload = {
          serverId: serverId,
          requestData: requestBody,
        };

        const result = await API.post("/api/execute", payload);

        // Navigate to new execution detail
        if (result.executionId) {
          window.location.href = `/request-detail.html?id=${result.executionId}`;
        }
      } catch (error) {
        console.error("Failed to execute request:", error);
        alert(`Failed to execute request: ${error.message}`);
      }
    },

    async executeRequest() {
      try {
        const requestBody = this.executeForm.requestDataOverride;
        if (!requestBody) {
          alert("Please provide request data");
          return;
        }

        // Determine server ID
        const server = this.requestDetail?.server || this.requestDetail?.execution?.server;
        const serverId = server?.id || this.requestDetail?.execution?.serverId;

        if (!serverId) {
          alert("No server found for this request");
          return;
        }

        const payload = {
          serverId: serverId,
          requestData: requestBody,
          bearerTokenOverride: this.executeForm.tokenOverride || undefined,
          devIdOverride: this.executeForm.devIdOverride || undefined,
        };

        const result = await API.post("/api/execute", payload);

        // Close modal
        this.showExecuteModal = false;

        // Navigate to new execution detail
        if (result.executionId) {
          window.location.href = `/request-detail.html?id=${result.executionId}`;
        }
      } catch (error) {
        console.error("Failed to execute request:", error);
        alert(`Failed to execute request: ${error.message}`);
      }
    },

    toggleLogStream() {
      this.showLiveLogStream = !this.showLiveLogStream;
    },
  },

  template: mainTemplate,
});

app.component("app-header", createAppHeader("request-detail"));
app.component("pev2", pev2.Plan);
app.component("log-stream", createLogStreamComponent());

app.mount("#app");
