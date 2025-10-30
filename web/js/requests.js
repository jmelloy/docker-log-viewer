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
      showRequestDetailModal: false,
      showExplainPlanModal: false,
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
      selectedRequestDetail: null,
      explainPlanData: null,
      comparisonData: null,
      pev2App: null,
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

    async showRequestDetail(requestId) {
      try {
        const response = await fetch(`/api/executions/${requestId}`);
        const detail = await response.json();
        this.selectedRequestDetail = detail;
        this.showRequestDetailModal = true;
      } catch (error) {
        console.error("Failed to load request detail:", error);
      }
    },

    getDetailStatusClass() {
      if (!this.selectedRequestDetail) return "";
      const statusCode = this.selectedRequestDetail.execution.statusCode;
      if (!statusCode || statusCode === 0) return "pending";
      return statusCode >= 200 && statusCode < 300 ? "success" : "error";
    },

    getDetailRequestData() {
      if (!this.selectedRequestDetail?.execution.requestBody)
        return "(no request data)";
      try {
        const data = JSON.parse(
          this.selectedRequestDetail.execution.requestBody
        );
        return JSON.stringify(data, null, 2);
      } catch (e) {
        return this.selectedRequestDetail.execution.requestBody;
      }
    },

    getDetailResponseBody() {
      if (!this.selectedRequestDetail?.execution.responseBody)
        return "(no response)";
      try {
        const data = JSON.parse(
          this.selectedRequestDetail.execution.responseBody
        );
        return JSON.stringify(data, null, 2);
      } catch (e) {
        return this.selectedRequestDetail.execution.responseBody;
      }
    },

    handleExplainClick(queryIdx) {
      const query = this.selectedRequestDetail.sqlQueries[queryIdx];

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
          this.selectedRequestDetail?.server?.defaultDatabase?.connectionString
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
        if (this.selectedRequestDetail) {
          const parts = [];

          // Add request name if available
          if (this.selectedRequestDetail.request?.name) {
            parts.push(this.selectedRequestDetail.request.name);
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

    async saveNewSampleQuery() {
      const {
        name,
        requestData,
        serverId,
        createNewServer,
        url,
        bearerToken,
        devId,
      } = this.newQueryForm;

      if (!name || !requestData) {
        alert("Please fill in all required fields");
        return;
      }

      // Validate JSON
      try {
        JSON.parse(requestData);
      } catch (e) {
        alert("Invalid JSON in query data");
        return;
      }

      // Build request payload
      const payload = { name, requestData };

      if (createNewServer) {
        // Create new server with provided details
        if (url) {
          payload.url = url;
          if (bearerToken) payload.bearerToken = bearerToken;
          if (devId) payload.devId = devId;
        }
      } else if (serverId) {
        // Use existing server
        payload.serverId = parseInt(serverId);
      }
      // If neither option is selected, create without server

      try {
        const response = await fetch("/api/requests", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify(payload),
        });

        if (!response.ok) {
          throw new Error("Failed to save sample query");
        }

        this.showNewSampleQueryModal = false;
        await this.loadServers(); // Reload servers in case a new one was created
        await this.loadSampleQueries();
      } catch (error) {
        console.error("Failed to save sample query:", error);
        alert("Failed to save sample query: " + error.message);
      }
    },

    openExecuteQueryModal() {
      if (!this.selectedSampleQuery) return;

      // Pre-select the sample query's server if available
      this.executeForm = {
        serverId: this.selectedSampleQuery.server
          ? this.selectedSampleQuery.server.id.toString()
          : "",
        requestDataOverride: "",
        urlOverride: "",
        tokenOverride: "",
        devIdOverride: "",
      };

      this.showExecuteQueryModal = true;
    },

    openExecuteNewModal() {
      this.executeForm = {
        serverId: "",
        requestDataOverride: "",
        urlOverride: "",
        tokenOverride: "",
        devIdOverride: "",
      };
      this.selectedSampleQuery = null;
      this.showExecuteNewModal = true;
    },

    openExecuteModalWithSampleQuery(sampleQuery) {
      this.selectedSampleQuery = sampleQuery;
      this.executeForm = {
        serverId: sampleQuery.server ? sampleQuery.server.id.toString() : "",
        requestDataOverride: "",
        urlOverride: "",
        tokenOverride: "",
        devIdOverride: "",
      };

      try {
        const data = JSON.parse(sampleQuery.requestData);
        this.executeForm.requestDataOverride = JSON.stringify(data, null, 2);
      } catch (e) {
        this.executeForm.requestDataOverride = sampleQuery.requestData;
      }

      this.showExecuteNewModal = true;
    },

    selectSampleQueryForExecution() {
      if (!this.selectedSampleQuery) {
        this.executeForm.requestDataOverride = "";
        return;
      }

      if (this.selectedSampleQuery.server) {
        this.executeForm.serverId =
          this.selectedSampleQuery.server.id.toString();
      }
      // Populate the request data override field with the selected sample query's data
      try {
        const data = JSON.parse(this.selectedSampleQuery.requestData);
        this.executeForm.requestDataOverride = JSON.stringify(data, null, 2);
      } catch (e) {
        this.executeForm.requestDataOverride =
          this.selectedSampleQuery.requestData;
      }
    },

    async executeSelectedQuery() {
      if (!this.selectedSampleQuery) {
        alert("Please select a sample query");
        return;
      }

      const {
        serverId,
        requestDataOverride,
        urlOverride,
        tokenOverride,
        devIdOverride,
      } = this.executeForm;

      if (!serverId) {
        alert("Please select a server");
        return;
      }

      // Validate request data if provided
      if (requestDataOverride) {
        try {
          JSON.parse(requestDataOverride);
        } catch (e) {
          alert("Invalid JSON in request data");
          return;
        }
      }

      const payload = {
        serverId: parseInt(serverId),
      };

      if (requestDataOverride) {
        payload.requestDataOverride = requestDataOverride;
      }
      if (urlOverride) {
        payload.urlOverride = urlOverride;
      }
      if (tokenOverride) {
        payload.bearerTokenOverride = tokenOverride;
      }
      if (devIdOverride) {
        payload.devIdOverride = devIdOverride;
      }

      try {
        const selectedServer = this.servers.find(
          (s) => s.id === parseInt(serverId)
        );
        let operationName = "Executing...";

        try {
          const requestData = JSON.parse(
            requestDataOverride || this.selectedSampleQuery.requestData
          );
          operationName = requestData.operationName || "Executing...";
        } catch (e) {
          // Ignore parse errors for operation name
        }

        const optimisticRequest = {
          id: Date.now(),
          statusCode: null,
          durationMs: null,
          executedAt: new Date().toISOString(),
          server: selectedServer,
          requestIdHeader: "pending",
          operationName: operationName,
          lastLog: null,
        };

        this.allRequests.unshift(optimisticRequest);

        const response = await fetch(
          `/api/requests/${this.selectedSampleQuery.id}/execute`,
          {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
            },
            body: JSON.stringify(payload),
          }
        );

        if (!response.ok) {
          this.allRequests = this.allRequests.filter(
            (r) => r.id !== optimisticRequest.id
          );
          throw new Error("Failed to execute query");
        }

        const result = await response.json();

        this.allRequests = this.allRequests.filter(
          (r) => r.id !== optimisticRequest.id
        );

        this.showExecuteNewModal = false;

        setTimeout(async () => {
          await this.loadAllRequests();
        }, 2000);
      } catch (error) {
        console.error("Failed to execute query:", error);
        alert("Failed to execute query: " + error.message);
      }
    },

    async confirmExecuteQuery() {
      if (!this.selectedSampleQuery) return;

      const { serverId, tokenOverride, devIdOverride } = this.executeForm;

      if (!serverId) {
        alert("Please select a server");
        return;
      }

      const payload = {
        serverId: parseInt(serverId),
      };

      if (tokenOverride) {
        payload.bearerTokenOverride = tokenOverride;
      }
      if (devIdOverride) {
        payload.devIdOverride = devIdOverride;
      }

      try {
        const response = await fetch(
          `/api/requests/${this.selectedSampleQuery.id}/execute`,
          {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
            },
            body: JSON.stringify(payload),
          }
        );

        if (!response.ok) {
          throw new Error("Failed to execute query");
        }

        const result = await response.json();
        this.showExecuteQueryModal = false;

        // Immediately reload to show the pending execution
        await this.loadAllRequests();
        await this.loadRequests(this.selectedSampleQuery.id);

        // Start polling for execution completion
        if (result.executionId) {
          this.pollExecutionStatus(result.executionId);
        }
      } catch (error) {
        console.error("Failed to execute query:", error);
        alert("Failed to execute query: " + error.message);
      }
    },

    async pollExecutionStatus(executionId, attempts = 0) {
      const maxAttempts = 60; // Poll for up to 60 seconds
      const interval = 1000; // 1 second

      if (attempts >= maxAttempts) {
        console.log("Polling timeout for execution", executionId);
        return;
      }

      try {
        const response = await fetch(`/api/executions/${executionId}`);
        if (!response.ok) {
          throw new Error("Failed to fetch execution detail");
        }

        const detail = await response.json();

        // Check if execution is complete (status code is set)
        if (detail.execution && detail.execution.statusCode > 0) {
          // Execution is complete, reload the requests
          await this.loadAllRequests();
          if (this.selectedSampleQuery) {
            await this.loadRequests(this.selectedSampleQuery.id);
          }
          console.log("Execution complete", executionId);
          return;
        }

        // Continue polling
        setTimeout(() => {
          this.pollExecutionStatus(executionId, attempts + 1);
        }, interval);
      } catch (error) {
        console.error("Error polling execution status:", error);
        // Continue polling despite errors
        setTimeout(() => {
          this.pollExecutionStatus(executionId, attempts + 1);
        }, interval);
      }
    },

    async deleteSampleQuery() {
      if (!this.selectedSampleQuery) return;

      if (
        !confirm(
          `Delete sample query "${this.selectedSampleQuery.name}"? This will also delete all requests.`
        )
      ) {
        return;
      }

      try {
        const response = await fetch(
          `/api/requests/${this.selectedSampleQuery.id}`,
          {
            method: "DELETE",
          }
        );

        if (!response.ok) {
          throw new Error("Failed to delete sample query");
        }

        this.selectedSampleQuery = null;

        await this.loadSampleQueries();
        await this.loadAllRequests();
      } catch (error) {
        console.error("Failed to delete sample query:", error);
        alert("Failed to delete sample query: " + error.message);
      }
    },

    async runExplain(query, variables = {}, connectionString) {
      try {
        console.log("selectedRequestDetail:", this.selectedRequestDetail);

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

    formatSQL(sql) {
      if (!sql?.trim()) return sql;

      let formatted = sql.replace(/\s+/g, " ").trim();

      const keywords = [
        "SELECT",
        "FROM",
        "WHERE",
        "JOIN",
        "LEFT JOIN",
        "RIGHT JOIN",
        "INNER JOIN",
        "FULL JOIN",
        "GROUP BY",
        "ORDER BY",
        "HAVING",
        "UNION",
        "INSERT INTO",
        "UPDATE",
        "DELETE FROM",
        "SET",
        "VALUES",
      ];

      keywords.forEach((kw) => {
        formatted = formatted.replace(
          new RegExp(`\\b${kw}\\b`, "gi"),
          `\n${kw.toUpperCase()}`
        );
      });

      // Split into lines and process SELECT lists specially
      let lines = formatted.split("\n");
      let result = [];

      for (let i = 0; i < lines.length; i++) {
        const line = lines[i].trim();
        if (!line) continue;

        // Check if this is a SELECT line with comma-separated items
        if (line.startsWith("SELECT ")) {
          result.push("SELECT");

          // Get everything after SELECT until next keyword
          let selectContent = line.substring(7); // Remove 'SELECT '
          let j = i + 1;
          while (
            j < lines.length &&
            !lines[j]
              .trim()
              .match(/^(FROM|WHERE|JOIN|GROUP BY|ORDER BY|HAVING|UNION)/i)
          ) {
            selectContent += " " + lines[j].trim();
            j++;
          }
          i = j - 1; // Skip processed lines

          // Split by commas (outside parens) and group to ~120 chars
          const items = [];
          let depth = 0;
          let current = "";
          for (let k = 0; k < selectContent.length; k++) {
            const char = selectContent[k];
            if (char === "(") depth++;
            else if (char === ")") depth--;

            if (char === "," && depth === 0) {
              items.push(current.trim());
              current = "";
            } else {
              current += char;
            }
          }
          if (current.trim()) items.push(current.trim());

          // Group items to fit ~120 chars per line
          let currentLine = "";
          items.forEach((item, idx) => {
            const separator = idx === 0 ? "" : ", ";
            if (currentLine && (currentLine + separator + item).length > 120) {
              result.push("  " + currentLine + ",");
              currentLine = item;
            } else {
              currentLine += separator + item;
            }
          });
          if (currentLine) result.push("  " + currentLine);

          result.push(""); // Blank line after SELECT
        } else {
          result.push(line);
        }
      }

      // Indent based on parens
      let indent = 0;
      return result
        .map((line) => {
          const trimmed = line.trim();
          if (!trimmed) return "";
          if (
            trimmed === "SELECT" ||
            trimmed.match(/^(FROM|WHERE|GROUP BY|ORDER BY)/i)
          ) {
            indent = 0;
            return trimmed;
          }

          if (trimmed.startsWith(")")) indent = Math.max(0, indent - 1);
          const formatted = "  ".repeat(indent) + trimmed;

          const opens = (trimmed.match(/\(/g) || []).length;
          const closes = (trimmed.match(/\)/g) || []).length;
          indent += opens - closes;

          return formatted;
        })
        .join("\n");
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

    <!-- Request Detail Modal -->
    <div v-if="showRequestDetailModal && selectedRequestDetail" class="modal">
      <div class="modal-content execution-modal-content">
        <div class="modal-header">
          <h3>{{ selectedRequestDetail.request?.name || '(unnamed)' }} - {{ selectedRequestDetail.server?.name || selectedRequestDetail.execution.server?.name || 'N/A' }}</h3>
          <button @click="showRequestDetailModal = false">✕</button>
        </div>
        <div class="modal-body">
          <div class="execution-overview">
            
            <div class="stat-item">
              <span class="stat-label">Status Code</span>
              <span class="stat-value" :class="getDetailStatusClass()">{{ selectedRequestDetail.execution.statusCode || 'Error' }}</span>
            </div>
            <div class="stat-item">
              <span class="stat-label">Duration</span>
              <span class="stat-value">{{ selectedRequestDetail.execution.durationMs }}ms</span>
            </div>
            <div class="stat-item">
              <span class="stat-label">Request ID</span>
              <span class="stat-value">{{ selectedRequestDetail.execution.requestIdHeader }}</span>
            </div>
            <div class="stat-item">
              <span class="stat-label">Executed At</span>
              <span class="stat-value">{{ new Date(selectedRequestDetail.execution.executedAt).toLocaleString() }}</span>
            </div>
            <div class="stat-item" v-if="selectedRequestDetail.server?.devId || selectedRequestDetail.execution.server?.devId">
              <span class="stat-label">Dev ID</span>
              <span class="stat-value">{{ selectedRequestDetail.server?.devId || selectedRequestDetail.execution.server?.devId || 'N/A' }}</span>
            </div>
          </div>

          <div class="modal-section">
            <h4>Request Body</h4>
            <pre class="json-display">{{ getDetailRequestData() }}</pre>
          </div>

          <div class="modal-section">
            <h4>Response</h4>
            <pre class="json-display">{{ getDetailResponseBody() }}</pre>
          </div>

          <div v-if="selectedRequestDetail.execution.error" class="modal-section">
            <h4>Error</h4>
            <pre class="error-display">{{ selectedRequestDetail.execution.error }}</pre>
          </div>

          <div v-if="selectedRequestDetail.sqlAnalysis && selectedRequestDetail.sqlAnalysis.totalQueries > 0" class="modal-section">
            <h4>SQL Analysis</h4>
            <div class="stats-grid">
              <div class="stat-item">
                <span class="stat-label">Total Queries</span>
                <span class="stat-value">{{ selectedRequestDetail.sqlAnalysis.totalQueries }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">Unique Queries</span>
                <span class="stat-value">{{ selectedRequestDetail.sqlAnalysis.uniqueQueries }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">Avg Duration</span>
                <span class="stat-value">{{ selectedRequestDetail.sqlAnalysis.avgDuration.toFixed(2) }}ms</span>
              </div>
            </div>
            <br/>

            <div class="sql-queries-list">
              <div v-for="(q, idx) in selectedRequestDetail.sqlQueries" :key="idx" class="sql-query-item">
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
            <h4>Logs (<span>{{ selectedRequestDetail.logs.length }}</span>)</h4>
            <div class="logs-list">
              <p v-if="selectedRequestDetail.logs.length === 0" style="color: #6c757d;">No logs captured</p>
              <div v-for="(log, idx) in selectedRequestDetail.logs" :key="idx" class="log-entry">
                <div class="log-entry-header">
                  <span class="log-level" :class="log.level || 'INFO'">{{ log.level || 'INFO' }}</span>
                  <span class="log-timestamp">{{ new Date(log.timestamp).toLocaleTimeString() }}</span>
                </div>
                <div class="log-message">{{ log.message || log.rawLog }}</div>
              </div>
            </div>
          </div>
        </div>
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
