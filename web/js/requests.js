class RequestManager {
  constructor() {
    this.sampleQueries = [];
    this.servers = [];
    this.selectedSampleQuery = null;
    this.requests = [];
    this.allRequests = [];
    this.init();
  }

  async init() {
    this.setupEventListeners();
    await this.loadServers();
    await this.loadSampleQueries();
    await this.loadAllRequests();
    this.populateServerDropdown();
    this.populateSampleQueryDropdown();
  }

  setupEventListeners() {
    // Manage sample queries modal
    document
      .getElementById("manageSampleQueriesBtn")
      .addEventListener("click", () => {
        this.showManageSampleQueriesModal();
      });

    document
      .getElementById("closeManageSampleQueriesModal")
      .addEventListener("click", () => {
        this.hideManageSampleQueriesModal();
      });

    // New sample query modal
    document
      .getElementById("newSampleQueryBtn")
      .addEventListener("click", () => {
        this.showNewSampleQueryModal();
      });

    document
      .getElementById("closeNewSampleQueryModal")
      .addEventListener("click", () => {
        this.hideNewSampleQueryModal();
      });

    document
      .getElementById("cancelNewSampleQueryBtn")
      .addEventListener("click", () => {
        this.hideNewSampleQueryModal();
      });

    document
      .getElementById("saveSampleQueryBtn")
      .addEventListener("click", () => {
        this.saveNewSampleQuery();
      });

    // Server selection change
    document
      .getElementById("newSampleQueryServer")
      .addEventListener("change", (e) => {
        this.handleServerChange(e.target.value);
      });

    // Sample query selection
    document
      .getElementById("sampleQuerySelect")
      .addEventListener("change", (e) => {
        this.handleSampleQuerySelection(e.target.value);
      });

    // Execute from sidebar
    document
      .getElementById("executeSidebarBtn")
      .addEventListener("click", () => {
        this.executeFromSidebar();
      });

    // Request detail modal
    document
      .getElementById("closeRequestDetailModal")
      .addEventListener("click", () => {
        this.hideRequestDetailModal();
      });

    // EXPLAIN plan modal
    document
      .getElementById("closeExplainPlanModal")
      .addEventListener("click", () => {
        this.hideExplainPlanModal();
      });

    // Comparison modal
    document
      .getElementById("closeComparisonModal")
      .addEventListener("click", () => {
        this.hideComparisonModal();
      });

    // Compare button
    document
      .getElementById("compareRequestsBtn")
      .addEventListener("click", () => {
        this.compareSelectedRequests();
      });
  }

  async loadServers() {
    try {
      const response = await fetch("/api/servers");
      this.servers = await response.json();
    } catch (error) {
      console.error("Failed to load servers:", error);
      this.servers = [];
    }
  }

  async loadSampleQueries() {
    try {
      const response = await fetch("/api/requests");
      this.sampleQueries = await response.json();
      this.renderSampleQueriesList();
    } catch (error) {
      console.error("Failed to load sample queries:", error);
    }
  }

  async loadAllRequests() {
    try {
      const response = await fetch("/api/all-executions");
      this.allRequests = await response.json();
      this.renderAllRequestsList();
    } catch (error) {
      console.error("Failed to load requests:", error);
    }
  }

  renderSampleQueriesList() {
    const container = document.getElementById("sampleQueriesList");

    if (this.sampleQueries.length === 0) {
      container.innerHTML =
        '<p style="padding: 1rem; color: #6c757d; text-align: center;">No sample queries</p>';
      return;
    }

    container.innerHTML = this.sampleQueries
      .map((sq) => {
        // Extract operation name from requestData
        let displayName = sq.name;
        try {
          const data = JSON.parse(sq.requestData);
          if (data.operationName) {
            displayName = data.operationName;
          }
        } catch (e) {
          // Use the original name if parsing fails
        }

        return `
        <div class="request-item" data-id="${sq.id}">
          <div class="request-item-content">
            <div class="request-item-name">${this.escapeHtml(displayName)}</div>
            <div class="request-item-meta">
              <span>${new Date(sq.createdAt).toLocaleDateString()}</span>
            </div>
          </div>
          <button class="btn-delete-query" data-id="${
            sq.id
          }" data-name="${this.escapeHtml(displayName)}">Delete</button>
        </div>
      `;
      })
      .join("");

    // Add delete handlers
    container.querySelectorAll(".btn-delete-query").forEach((btn) => {
      btn.addEventListener("click", async (e) => {
        e.stopPropagation();
        const id = parseInt(btn.dataset.id);
        const name = btn.dataset.name;
        await this.deleteSampleQuery(id, name);
      });
    });
  }

  async renderAllRequestsList() {
    const container = document.getElementById("requestsList");

    if (this.allRequests.length === 0) {
      container.innerHTML =
        '<p style="color: #6c757d;">No requests executed yet.</p>';
      return;
    }

    // Fetch additional details for each request to get logs
    const requestsWithDetails = await Promise.all(
      this.allRequests.slice(0, 20).map(async (req) => {
        try {
          const response = await fetch(`/api/executions/${req.id}`);
          const detail = await response.json();
          const lastLog =
            detail.logs.length > 0 ? detail.logs[detail.logs.length - 1] : null;
          const operationName = detail.execution.name || "Unknown";

          return { ...req, lastLog, operationName };
        } catch (e) {
          return { ...req, lastLog: null, operationName: "Unknown" };
        }
      })
    );

    container.innerHTML = requestsWithDetails
      .map((req) => {
        const statusClass =
          req.statusCode >= 200 && req.statusCode < 300 ? "success" : "error";
        const time = new Date(req.executedAt);
        const timeStr = time.toLocaleTimeString();
        const serverUrl = req.serverUrl || "N/A";

        return `
        <div class="execution-item-detailed" data-id="${req.id}">
          <input type="checkbox" class="exec-compare-checkbox" data-id="${
            req.id
          }">
          <span class="exec-status ${statusClass}">${
          req.statusCode || "ERR"
        }</span>
          <span class="exec-name" title="${this.escapeHtml(
            req.operationName
          )}">${this.escapeHtml(req.operationName)}</span>
          <span class="exec-duration">${req.durationMs}ms</span>
          <span class="exec-time">${timeStr}</span>
          <span class="exec-server" title="${this.escapeHtml(
            serverUrl
          )}">${this.escapeHtml(serverUrl)}</span>
          <span class="exec-id" title="req_id=${req.requestIdHeader}">req_id=${
          req.requestIdHeader
        }</span>
        </div>
      `;
      })
      .join("");

    // Add click handlers
    container.querySelectorAll(".execution-item-detailed").forEach((item) => {
      item.addEventListener("click", (e) => {
        if (e.target.type !== "checkbox") {
          const id = parseInt(item.dataset.id);
          this.showRequestDetail(id);
        }
      });
    });

    // Handle checkbox changes
    container.querySelectorAll(".exec-compare-checkbox").forEach((checkbox) => {
      checkbox.addEventListener("change", () => {
        this.updateCompareButton();
      });
    });
  }

  updateCompareButton() {
    const checkedBoxes = document.querySelectorAll(
      ".exec-compare-checkbox:checked"
    );
    const compareBtn = document.getElementById("compareRequestsBtn");

    if (checkedBoxes.length === 2) {
      compareBtn.style.display = "block";
    } else {
      compareBtn.style.display = "none";
    }
  }

  async showRequestDetail(requestId) {
    try {
      const response = await fetch(`/api/executions/${requestId}`);
      const detail = await response.json();
      this.renderRequestDetail(detail);
    } catch (error) {
      console.error("Failed to load request detail:", error);
    }
  }

  renderRequestDetail(detail) {
    const modal = document.getElementById("requestDetailModal");

    // Overview stats
    const statusClass =
      detail.execution.statusCode >= 200 && detail.execution.statusCode < 300
        ? "success"
        : "error";
    document.getElementById("reqStatusCode").textContent =
      detail.execution.statusCode || "Error";
    document.getElementById(
      "reqStatusCode"
    ).className = `stat-value ${statusClass}`;
    document.getElementById(
      "reqDuration"
    ).textContent = `${detail.execution.durationMs}ms`;
    document.getElementById("reqRequestID").textContent =
      detail.execution.requestIdHeader;
    document.getElementById("reqTime").textContent = new Date(
      detail.execution.executedAt
    ).toLocaleString();

    // Request Body
    if (detail.execution.requestData) {
      try {
        const data = JSON.parse(detail.execution.requestData);
        document.getElementById("reqRequestBody").textContent = JSON.stringify(
          data,
          null,
          2
        );
      } catch (e) {
        document.getElementById("reqRequestBody").textContent =
          detail.execution.requestData;
      }
    } else {
      document.getElementById("reqRequestBody").textContent =
        "(no request data)";
    }

    // Response
    if (detail.execution.responseBody) {
      try {
        const data = JSON.parse(detail.execution.responseBody);
        document.getElementById("reqResponse").textContent = JSON.stringify(
          data,
          null,
          2
        );
      } catch (e) {
        document.getElementById("reqResponse").textContent =
          detail.execution.responseBody;
      }
    } else {
      document.getElementById("reqResponse").textContent = "(no response)";
    }

    // Error
    if (detail.execution.error) {
      document.getElementById("reqErrorSection").style.display = "block";
      document.getElementById("reqError").textContent = detail.execution.error;
    } else {
      document.getElementById("reqErrorSection").style.display = "none";
    }

    // SQL Analysis
    if (detail.sqlAnalysis && detail.sqlAnalysis.totalQueries > 0) {
      document.getElementById("reqSQLSection").style.display = "block";
      document.getElementById("sqlTotalQueries").textContent =
        detail.sqlAnalysis.totalQueries;
      document.getElementById("sqlUniqueQueries").textContent =
        detail.sqlAnalysis.uniqueQueries;
      document.getElementById(
        "sqlAvgDuration"
      ).textContent = `${detail.sqlAnalysis.avgDuration.toFixed(2)}ms`;
      document.getElementById(
        "sqlTotalDuration"
      ).textContent = `${detail.sqlAnalysis.totalDuration.toFixed(2)}ms`;

      // Render SQL queries
      const sqlContainer = document.getElementById("sqlQueriesList");
      sqlContainer.innerHTML = detail.sqlQueries
        .map((q, idx) => {
          const hasPlan = q.explainPlan && q.explainPlan.length > 0;
          return `
          <div class="sql-query-item">
            <div class="sql-query-header">
              <span>${q.tableName || "unknown"} - ${
            q.operation || "SELECT"
          }</span>
              <span class="sql-query-duration">${q.durationMs.toFixed(
                2
              )}ms</span>
            </div>
            <div class="sql-query-text">${this.escapeHtml(q.query)}</div>
            <div class="query-actions">
              ${
                !hasPlan
                  ? `<button class="btn-explain" data-query-idx="${idx}">Run EXPLAIN</button>`
                  : ""
              }
              ${
                hasPlan
                  ? `<button class="btn-explain btn-secondary" data-query-idx="${idx}" data-show-plan="true">View Saved Plan</button>`
                  : ""
              }
            </div>
          </div>
        `;
        })
        .join("");

      // Add EXPLAIN click handlers
      sqlContainer.querySelectorAll(".btn-explain").forEach((btn) => {
        btn.addEventListener("click", (e) => {
          const idx = parseInt(e.target.dataset.queryIdx);
          const query = detail.sqlQueries[idx];

          if (e.target.dataset.showPlan === "true") {
            // Show saved plan
            try {
              const plan = JSON.parse(query.explainPlan);
              this.showExplainPlan(plan, query.query);
            } catch (err) {
              alert("Error parsing saved plan: " + err.message);
            }
          } else {
            // Run new EXPLAIN
            const variables = query.variables
              ? JSON.parse(query.variables)
              : {};
            this.runExplain(query.query, variables);
          }
        });
      });
    } else {
      document.getElementById("reqSQLSection").style.display = "none";
    }

    // Logs
    document.getElementById("logsCount").textContent = detail.logs.length;
    const logsContainer = document.getElementById("reqLogs");

    if (detail.logs.length === 0) {
      logsContainer.innerHTML =
        '<p style="color: #6c757d;">No logs captured</p>';
    } else {
      logsContainer.innerHTML = detail.logs
        .map(
          (log) => `
        <div class="log-entry">
          <div class="log-entry-header">
            <span class="log-level ${log.level || "INFO"}">${
            log.level || "INFO"
          }</span>
            <span class="log-timestamp">${new Date(
              log.timestamp
            ).toLocaleTimeString()}</span>
          </div>
          <div class="log-message">${this.escapeHtml(
            log.message || log.rawLog
          )}</div>
        </div>
      `
        )
        .join("");
    }

    modal.classList.remove("hidden");
  }

  hideRequestDetailModal() {
    document.getElementById("requestDetailModal").classList.add("hidden");
  }

  showExplainPlan(planData, query) {
    const modal = document.getElementById("explainPlanModal");
    const errorDiv = document.getElementById("explainPlanError");
    const pev2Container = document.getElementById("pev2ExplainApp");

    errorDiv.style.display = "none";

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

      modal.classList.remove("hidden");
    } catch (err) {
      errorDiv.textContent = `Failed to display plan: ${err.message}`;
      errorDiv.style.display = "block";
      modal.classList.remove("hidden");
    }
  }

  hideExplainPlanModal() {
    document.getElementById("explainPlanModal").classList.add("hidden");
    if (this.pev2App) {
      this.pev2App.unmount();
      this.pev2App = null;
    }
  }

  showNewSampleQueryModal() {
    // Reset form
    document.getElementById("newSampleQueryName").value = "";
    document.getElementById("newSampleQueryURL").value = "";
    document.getElementById("newSampleQueryData").value = "";
    document.getElementById("newSampleQueryToken").value = "";
    document.getElementById("newSampleQueryDevID").value = "";

    // Populate server dropdown
    const serverSelect = document.getElementById("newSampleQueryServer");
    serverSelect.innerHTML = '<option value="">-- New Server --</option>';
    this.servers.forEach((server) => {
      const option = document.createElement("option");
      option.value = server.id;
      option.textContent = `${server.name} (${server.url})`;
      serverSelect.appendChild(option);
    });

    // Reset to new server mode
    serverSelect.value = "";
    this.handleServerChange("");

    document.getElementById("newSampleQueryModal").classList.remove("hidden");
  }

  handleServerChange(serverId) {
    const newServerFields = document.getElementById("newServerFields");
    if (serverId === "") {
      // New server mode - show URL/token/devID fields
      newServerFields.style.display = "block";
      document.getElementById("newSampleQueryURL").required = true;
    } else {
      // Existing server mode - hide URL/token/devID fields
      newServerFields.style.display = "none";
      document.getElementById("newSampleQueryURL").required = false;
    }
  }

  hideNewSampleQueryModal() {
    document.getElementById("newSampleQueryModal").classList.add("hidden");
  }

  async saveNewSampleQuery() {
    const name = document.getElementById("newSampleQueryName").value.trim();
    const requestData = document
      .getElementById("newSampleQueryData")
      .value.trim();
    const serverSelect = document.getElementById("newSampleQueryServer");
    const serverId = serverSelect.value;

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
    const payload = {
      name,
      requestData,
    };

    if (serverId) {
      // Use existing server
      payload.serverId = parseInt(serverId);
    } else {
      // Create new server with provided details
      const url = document.getElementById("newSampleQueryURL").value.trim();
      const bearerToken = document
        .getElementById("newSampleQueryToken")
        .value.trim();
      const devId = document.getElementById("newSampleQueryDevID").value.trim();

      if (!url) {
        alert("Please provide a URL for the new server");
        return;
      }

      payload.url = url;
      if (bearerToken) payload.bearerToken = bearerToken;
      if (devId) payload.devId = devId;
    }

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

      this.hideNewSampleQueryModal();
      await this.loadServers(); // Reload servers in case a new one was created
      await this.loadSampleQueries();
    } catch (error) {
      console.error("Failed to save sample query:", error);
      alert("Failed to save sample query: " + error.message);
    }
  }

  showExecuteQueryModal() {
    if (!this.selectedSampleQuery) return;

    // Populate server dropdown
    const serverSelect = document.getElementById("executeServer");
    serverSelect.innerHTML = '<option value="">-- Select Server --</option>';
    this.servers.forEach((server) => {
      const option = document.createElement("option");
      option.value = server.id;
      option.textContent = `${server.name} (${server.url})`;
      serverSelect.appendChild(option);
    });

    // Pre-select the sample query's server if available
    if (this.selectedSampleQuery.server) {
      serverSelect.value = this.selectedSampleQuery.server.id;
    }

    // Clear override fields
    document.getElementById("executeToken").value = "";
    document.getElementById("executeDevID").value = "";

    document.getElementById("executeQueryModal").classList.remove("hidden");
  }

  hideExecuteQueryModal() {
    document.getElementById("executeQueryModal").classList.add("hidden");
  }

  async confirmExecuteQuery() {
    if (!this.selectedSampleQuery) return;

    const serverSelect = document.getElementById("executeServer");
    const serverId = serverSelect.value;
    const tokenOverride = document.getElementById("executeToken").value.trim();
    const devIdOverride = document.getElementById("executeDevID").value.trim();

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

      this.hideExecuteQueryModal();
      alert(
        "Query execution started. Results will appear in the requests list."
      );

      // Reload requests after a delay
      setTimeout(() => {
        this.loadRequests(this.selectedSampleQuery.id);
        this.loadAllRequests();
      }, 12000); // Wait 12 seconds for logs to be collected
    } catch (error) {
      console.error("Failed to execute query:", error);
      alert("Failed to execute query: " + error.message);
    }
  }

  populateServerDropdown() {
    const select = document.getElementById("executeServerSelect");
    select.innerHTML = '<option value="">-- Select Server --</option>';
    this.servers.forEach((server) => {
      const option = document.createElement("option");
      option.value = server.id;
      option.textContent = `${server.name} (${server.url})`;
      select.appendChild(option);
    });
  }

  populateSampleQueryDropdown() {
    const select = document.getElementById("sampleQuerySelect");
    select.innerHTML = '<option value="">-- Select to populate --</option>';
    this.sampleQueries.forEach((sq) => {
      const option = document.createElement("option");
      option.value = sq.id;

      // Extract operation name for display
      let displayName = sq.name;
      try {
        const data = JSON.parse(sq.requestData);
        if (data.operationName) {
          displayName = data.operationName;
        }
      } catch (e) {
        // Use the original name if parsing fails
      }

      option.textContent = displayName;
      select.appendChild(option);
    });
  }

  handleSampleQuerySelection(queryId) {
    if (!queryId) return;

    const query = this.sampleQueries.find((sq) => sq.id === parseInt(queryId));
    if (!query) return;

    // Populate the form fields
    document.getElementById("executeQueryData").value = query.requestData;

    if (query.serverId) {
      document.getElementById("executeServerSelect").value = query.serverId;
    }
  }

  async executeFromSidebar() {
    const serverId = document.getElementById("executeServerSelect").value;
    const queryData = document.getElementById("executeQueryData").value.trim();
    const tokenOverride = document
      .getElementById("executeTokenOverride")
      .value.trim();
    const devIdOverride = document
      .getElementById("executeDevIDOverride")
      .value.trim();

    if (!serverId) {
      alert("Please select a server");
      return;
    }

    if (!queryData) {
      alert("Please enter query data");
      return;
    }

    // Validate JSON
    try {
      JSON.parse(queryData);
    } catch (e) {
      alert("Invalid JSON in query data");
      return;
    }

    const payload = {
      serverId: parseInt(serverId),
      requestData: queryData,
    };

    if (tokenOverride) {
      payload.bearerTokenOverride = tokenOverride;
    }
    if (devIdOverride) {
      payload.devIdOverride = devIdOverride;
    }

    try {
      const response = await fetch(`/api/execute`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        throw new Error("Failed to execute query");
      }

      alert(
        "Query execution started. Results will appear in the requests list."
      );

      // Clear form
      document.getElementById("executeQueryData").value = "";
      document.getElementById("executeTokenOverride").value = "";
      document.getElementById("executeDevIDOverride").value = "";
      document.getElementById("sampleQuerySelect").value = "";

      // Reload requests after a delay
      setTimeout(() => {
        this.loadAllRequests();
      }, 12000); // Wait 12 seconds for logs to be collected
    } catch (error) {
      console.error("Failed to execute query:", error);
      alert("Failed to execute query: " + error.message);
    }
  }

  showManageSampleQueriesModal() {
    document
      .getElementById("manageSampleQueriesModal")
      .classList.remove("hidden");
  }

  hideManageSampleQueriesModal() {
    document.getElementById("manageSampleQueriesModal").classList.add("hidden");
  }

  async deleteSampleQuery(id, name) {
    if (
      !confirm(
        `Delete sample query "${name}"? This will also delete all associated executions.`
      )
    ) {
      return;
    }

    try {
      const response = await fetch(`/api/requests/${id}`, {
        method: "DELETE",
      });

      if (!response.ok) {
        throw new Error("Failed to delete sample query");
      }

      // Reload sample queries and update dropdowns
      await this.loadSampleQueries();
      this.renderSampleQueriesList();
      this.populateSampleQueryDropdown();
      await this.loadAllRequests();
    } catch (error) {
      console.error("Failed to delete sample query:", error);
      alert("Failed to delete sample query: " + error.message);
    }
  }

  async runExplain(query, variables = {}) {
    try {
      const payload = {
        query: query,
        variables: variables,
      };

      const response = await fetch("/api/explain", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(payload),
      });

      const result = await response.json();

      // Show EXPLAIN result in a simple alert for now
      // TODO: Integrate PEV2 visualization
      if (result.error) {
        alert(`EXPLAIN Error: ${result.error}`);
      } else {
        const planText = result.queryPlan
          ? JSON.stringify(result.queryPlan, null, 2)
          : "No plan available";
        console.log("EXPLAIN Plan:", planText);
        alert(
          "EXPLAIN plan logged to console. Check browser console for details."
        );
      }
    } catch (error) {
      alert(`Failed to run EXPLAIN: ${error.message}`);
    }
  }

  async compareSelectedRequests() {
    const checkedBoxes = Array.from(
      document.querySelectorAll(".exec-compare-checkbox:checked")
    );
    if (checkedBoxes.length !== 2) return;

    const ids = checkedBoxes.map((cb) => parseInt(cb.dataset.id));

    // Fetch details for both requests
    const [detail1, detail2] = await Promise.all(
      ids.map(async (id) => {
        const response = await fetch(`/api/executions/${id}`);
        return await response.json();
      })
    );

    this.showComparison(detail1, detail2);
  }

  showComparison(detail1, detail2) {
    const modal = document.getElementById("comparisonModal");
    const content = document.getElementById("comparisonContent");

    const exec1 = detail1.execution;
    const exec2 = detail2.execution;

    const timeDiff = exec2.durationMs - exec1.durationMs;
    const timeDiffPercent = ((timeDiff / exec1.durationMs) * 100).toFixed(1);

    // Reorder queries in detail2 to align with detail1 based on query hash
    const reorderedQueries2 = this.alignQueriesByHash(
      detail1.sqlQueries,
      detail2.sqlQueries
    );

    // Get server display names
    const server1Name = this.getServerName(exec1.serverUrl);
    const server2Name = this.getServerName(exec2.serverUrl);

    content.innerHTML = `
      <div class="comparison-grid">
        <div class="comparison-column">
          <h3>Request 1 - ${this.escapeHtml(server1Name)}</h3>
          <div class="comparison-stats">
            <div><strong>Status:</strong> ${
              exec1.statusCode
            } | <strong>Duration:</strong> ${exec1.durationMs}ms</div>
            <div title="${
              exec1.requestIdHeader
            }"><strong>Request ID:</strong> ${exec1.requestIdHeader.substring(
      0,
      20
    )}...</div>
            <div><strong>Executed:</strong> ${new Date(
              exec1.executedAt
            ).toLocaleString()}</div>
          </div>
          <div class="comparison-section">
            <h4>Response</h4>
            <pre class="json-display">${this.escapeHtml(
              exec1.responseBody || "No response"
            )}</pre>
          </div>
          <div class="comparison-section">
            <h4>SQL Queries (${detail1.sqlQueries.length})</h4>
            <div class="comparison-queries">
              ${detail1.sqlQueries
                .map((q, idx) => {
                  const matchingQ = reorderedQueries2[idx];
                  const isSame =
                    matchingQ && matchingQ.queryHash === q.queryHash;
                  const isSlower =
                    isSame && q.durationMs > matchingQ.durationMs;
                  return `
                  <div class="comparison-query ${
                    isSlower ? "slower-query" : ""
                  }">
                    <div><strong>${
                      q.tableName
                    }</strong> - ${q.durationMs.toFixed(2)}ms</div>
                    <div class="sql-query-text">${this.escapeHtml(
                      q.query
                    )}</div>
                  </div>
                `;
                })
                .join("")}
            </div>
          </div>
        </div>

        <div class="comparison-divider">
          <div class="comparison-diff">
            <div>Δ Time</div>
            <div class="${timeDiff > 0 ? "diff-slower" : "diff-faster"}">${
      timeDiff > 0 ? "+" : ""
    }${timeDiff}ms<br>${timeDiff > 0 ? "+" : ""}${timeDiffPercent}%</div>
          </div>
        </div>

        <div class="comparison-column">
          <h3>Request 2 - ${this.escapeHtml(server2Name)}</h3>
          <div class="comparison-stats">
            <div><strong>Status:</strong> ${
              exec2.statusCode
            } | <strong>Duration:</strong> ${exec2.durationMs}ms</div>
            <div title="${
              exec2.requestIdHeader
            }"><strong>Request ID:</strong> ${exec2.requestIdHeader.substring(
      0,
      20
    )}...</div>
            <div><strong>Executed:</strong> ${new Date(
              exec2.executedAt
            ).toLocaleString()}</div>
          </div>
          <div class="comparison-section">
            <h4>Response</h4>
            <pre class="json-display">${this.escapeHtml(
              exec2.responseBody || "No response"
            )}</pre>
          </div>
          <div class="comparison-section">
            <h4>SQL Queries (${reorderedQueries2.length})</h4>
            <div class="comparison-queries">
              ${reorderedQueries2
                .map((q, idx) => {
                  if (!q) {
                    return `<div class="comparison-query" style="opacity: 0.3; font-style: italic;">
                    <div>No matching query</div>
                  </div>`;
                  }
                  const matchingQ = detail1.sqlQueries[idx];
                  const isSame =
                    matchingQ && matchingQ.queryHash === q.queryHash;
                  const isSlower =
                    isSame && q.durationMs > matchingQ.durationMs;
                  return `
                  <div class="comparison-query ${
                    isSlower ? "slower-query" : ""
                  }">
                    <div><strong>${
                      q.tableName
                    }</strong> - ${q.durationMs.toFixed(2)}ms</div>
                    <div class="sql-query-text">${this.escapeHtml(
                      q.query
                    )}</div>
                  </div>
                `;
                })
                .join("")}
            </div>
          </div>
        </div>
      </div>
    `;

    modal.classList.remove("hidden");
  }

  alignQueriesByHash(queries1, queries2) {
    const result = [];
    const usedIndices = new Set();

    // For each query in queries1, try to find a matching query in queries2
    for (const q1 of queries1) {
      let matchIndex = -1;

      // Find a matching query by hash
      for (let i = 0; i < queries2.length; i++) {
        if (!usedIndices.has(i) && queries2[i].queryHash === q1.queryHash) {
          matchIndex = i;
          break;
        }
      }

      if (matchIndex >= 0) {
        result.push(queries2[matchIndex]);
        usedIndices.add(matchIndex);
      } else {
        // No match found, add placeholder
        result.push(null);
      }
    }

    // Add remaining unmatched queries from queries2
    for (let i = 0; i < queries2.length; i++) {
      if (!usedIndices.has(i)) {
        result.push(queries2[i]);
      }
    }

    return result;
  }

  hideComparisonModal() {
    document.getElementById("comparisonModal").classList.add("hidden");
  }

  getServerName(serverUrl) {
    if (!serverUrl) return "Unknown Server";

    const server = this.servers.find((s) => s.url === serverUrl);
    if (server && server.name) {
      return server.name;
    }

    // Fallback to URL hostname
    try {
      const url = new URL(serverUrl);
      return url.hostname;
    } catch (e) {
      return serverUrl;
    }
  }

  escapeHtml(text) {
    const div = document.createElement("div");
    div.textContent = text;
    return div.innerHTML;
  }
}

// Initialize app
const app = new RequestManager();
