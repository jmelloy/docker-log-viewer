const { createApp } = Vue;

const app = createApp({
  data() {
    return {
      containers: [],
      selectedContainers: new Set(),
      logs: [],
      searchQuery: "",
      traceFilter: null,
      selectedLevels: new Set(["DBG", "DEBUG", "TRC", "TRACE", "INF", "INFO", "WRN", "WARN", "ERR", "ERROR", "FATAL", "NONE"]),
      ws: null,
      wsConnected: false,
      showLogModal: false,
      showExplainModal: false,
      showAnalyzer: false,
      selectedLog: null,
      explainPlan: { planSource: '', planQuery: '', error: null },
      sqlAnalysis: {
        totalQueries: 0,
        uniqueQueries: 0,
        avgDuration: 0,
        totalDuration: 0,
        slowestQueries: [],
        frequentQueries: [],
        nPlusOne: [],
        tables: []
      }
    };
  },

  computed: {
    filteredLogs() {
      const startIdx = Math.max(0, this.logs.length - 1000);
      return this.logs.slice(startIdx).filter(log => this.shouldShowLog(log));
    },

    containersByProject() {
      const groups = {};
      this.containers.forEach(container => {
        const project = this.getProjectName(container.Name);
        if (!groups[project]) {
          groups[project] = [];
        }
        groups[project].push(container);
      });
      return groups;
    },

    projectNames() {
      return Object.keys(this.containersByProject).sort();
    },

    filterDisplayType() {
      return this.traceFilter ? this.traceFilter.type : "No filter";
    },

    filterDisplayValue() {
      return this.traceFilter ? this.traceFilter.value : "";
    },

    statusText() {
      return this.wsConnected ? "Connected" : "Connecting...";
    },

    statusColor() {
      return this.wsConnected ? "#7ee787" : "#f85149";
    },

    logCountText() {
      return `${this.logs.length} logs`;
    }
  },

  methods: {
    async init() {
      await this.loadContainers();
      this.connectWebSocket();
      await this.loadInitialLogs();
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
      const hasAll = levelVariants.every(v => this.selectedLevels.has(v));
      
      if (hasAll) {
        levelVariants.forEach(v => this.selectedLevels.delete(v));
      } else {
        levelVariants.forEach(v => this.selectedLevels.add(v));
      }
    },

    isLevelSelected(level) {
      const levelVariants = this.getLevelVariants(level);
      return levelVariants.every(v => this.selectedLevels.has(v));
    },

    async loadContainers() {
      try {
        const response = await fetch("/api/containers");
        this.containers = await response.json();
        this.containers.forEach(c => this.selectedContainers.add(c.ID));
      } catch (error) {
        console.error("Failed to load containers:", error);
      }
    },

    async loadInitialLogs() {
      try {
        const response = await fetch("/api/logs");
        const logs = await response.json();
        this.logs = logs;
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
      };

      this.ws.onmessage = (event) => {
        const message = JSON.parse(event.data);
        if (message.type === "log") {
          this.handleNewLog(message.data);
        } else if (message.type === "containers") {
          this.handleContainerUpdate(message.data);
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

    handleNewLog(log) {
      this.logs.push(log);
      if (this.logs.length > 10000) {
        this.logs = this.logs.slice(-1000);
      }
      this.$nextTick(() => {
        this.scrollToBottom();
      });
    },

    handleContainerUpdate(data) {
      const newContainers = data.containers;
      const oldIDs = new Set(this.containers.map(c => c.ID));
      const newIDs = new Set(newContainers.map(c => c.ID));

      const added = newContainers.filter(c => !oldIDs.has(c.ID));
      const removed = this.containers.filter(c => !newIDs.has(c.ID));

      if (added.length > 0) {
        added.forEach(c => {
          this.selectedContainers.add(c.ID);
          console.log(`Container started: ${c.Name} (${c.ID})`);
        });
      }

      if (removed.length > 0) {
        removed.forEach(c => {
          this.selectedContainers.delete(c.ID);
          console.log(`Container stopped: ${c.Name} (${c.ID})`);
        });
      }

      this.containers = newContainers;

      if (this.traceFilter) {
        this.analyzeTrace();
      }
    },

    shouldShowLog(log) {
      if (!this.selectedContainers.has(log.containerId)) {
        return false;
      }

      const logLevel = log.entry?.level ? log.entry.level.toUpperCase() : "NONE";
      if (!this.selectedLevels.has(logLevel)) {
        return false;
      }

      if (this.searchQuery && !this.matchesSearch(log)) {
        return false;
      }

      if (this.traceFilter) {
        const val = log.entry?.fields?.[this.traceFilter.type];
        if (val !== this.traceFilter.value) {
          return false;
        }
      }

      return true;
    },

    matchesSearch(log) {
      const searchable = [
        log.entry?.raw,
        log.entry?.message,
        log.entry?.level,
        ...Object.values(log.entry?.fields || {}),
      ]
        .join(" ")
        .toLowerCase();

      return searchable.includes(this.searchQuery.toLowerCase());
    },

    getProjectName(containerName) {
      const parts = containerName.split(/-/);
      if (parts.length <= 1) {
        return containerName;
      }

      if (parts[parts.length - 1].match(/^\d+$/)) {
        parts.pop();
        if (parts.length <= 2) {
          return parts[0];
        }
        return parts.slice(0, 2).join("-");
      }

      return containerName;
    },

    toggleContainer(containerId) {
      if (this.selectedContainers.has(containerId)) {
        this.selectedContainers.delete(containerId);
      } else {
        this.selectedContainers.add(containerId);
      }
    },

    isContainerSelected(containerId) {
      return this.selectedContainers.has(containerId);
    },

    toggleProject(project) {
      const projectContainers = this.containersByProject[project];
      const allSelected = projectContainers.every(c => this.selectedContainers.has(c.ID));
      
      projectContainers.forEach(c => {
        if (allSelected) {
          this.selectedContainers.delete(c.ID);
        } else {
          this.selectedContainers.add(c.ID);
        }
      });
    },

    isProjectSelected(project) {
      const projectContainers = this.containersByProject[project];
      return projectContainers.every(c => this.selectedContainers.has(c.ID));
    },

    isProjectIndeterminate(project) {
      const projectContainers = this.containersByProject[project];
      const someSelected = projectContainers.some(c => this.selectedContainers.has(c.ID));
      const allSelected = projectContainers.every(c => this.selectedContainers.has(c.ID));
      return someSelected && !allSelected;
    },

    getContainerName(containerId) {
      return this.containers.find(c => c.ID === containerId)?.Name || containerId;
    },

    setTraceFilter(type, value) {
      this.traceFilter = { type, value };
      this.analyzeTrace();
    },

    clearTraceFilter() {
      this.traceFilter = null;
      this.showAnalyzer = false;
    },

    clearLogs() {
      this.logs = [];
    },

    clearSearch() {
      this.searchQuery = "";
    },

    analyzeTrace() {
      if (!this.traceFilter) {
        this.showAnalyzer = false;
        return;
      }

      const traceLogs = this.logs.filter((log) => {
        if (!this.selectedContainers.has(log.containerId)) {
          return false;
        }
        const val = log.entry?.fields?.[this.traceFilter.type];
        return val === this.traceFilter.value;
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

          // Extract variables from db.vars field if present
          let variables = {};
          const dbVars = log.entry?.fields?.["db.vars"];
          if (dbVars) {
            try {
              // db.vars can be a JSON array or string
              const varsArray =
                typeof dbVars === "string" ? JSON.parse(dbVars) : dbVars;
              if (Array.isArray(varsArray)) {
                // Convert array to indexed map: {"1": value1, "2": value2, ...}
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
  }

  normalizeQuery(query) {
    return query
      .replace(/\$\d+/g, "$N")
      .replace(/'[^']*'/g, "'?'")
      .replace(/\d+/g, "N")
      .replace(/\s+/g, " ")
      .trim();
  }

  renderAnalysis(queries) {
    if (queries.length === 0) {
      document.getElementById("totalQueries").textContent = "0";
      document.getElementById("uniqueQueries").textContent = "0";
      document.getElementById("avgDuration").textContent = "0ms";
      document.getElementById("totalDuration").textContent = "0ms";
      document.getElementById("slowestQueries").innerHTML =
        '<div class="query-item">No SQL queries found</div>';
      document.getElementById("frequentQueries").innerHTML = "";
      document.getElementById("nPlusOne").innerHTML = "";
      document.getElementById("tablesAccessed").innerHTML = "";
      return;
    }

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

    document.getElementById("totalQueries").textContent = totalQueries;
    document.getElementById("uniqueQueries").textContent = uniqueQueries;
    document.getElementById("avgDuration").textContent =
      avgDuration.toFixed(2) + "ms";
    document.getElementById("totalDuration").textContent =
      totalDuration.toFixed(2) + "ms";

    const sortedBySlowest = [...queries]
      .sort((a, b) => b.duration - a.duration)
      .slice(0, 5);
    document.getElementById("slowestQueries").innerHTML = sortedBySlowest
      .map(
        (q, index) => `
            <div class="query-item" data-query-index="${index}">
                <div class="query-header">
                    <span class="query-duration ${
                      q.duration > 10 ? "query-slow" : ""
                    }">${q.duration.toFixed(2)}ms</span>
                </div>
                <div class="query-text">${this.escapeHtml(q.query)}</div>
                <div class="query-meta">
                    <span>Table: ${q.table}</span>
                    <span>Op: ${q.operation}</span>
                    <span>Rows: ${q.rows}</span>
                </div>
                <div class="query-actions">
                    <button class="btn-explain" data-query="${encodeURIComponent(
                      q.query
                    )}" data-variables="${encodeURIComponent(
          JSON.stringify(q.variables || {})
        )}">Run EXPLAIN</button>
                </div>
            </div>
        `
      )
      .join("");

    // Add event listeners to EXPLAIN buttons
    document.querySelectorAll(".btn-explain").forEach((btn) => {
      btn.addEventListener("click", (e) => {
        e.stopPropagation();
        const query = decodeURIComponent(btn.getAttribute("data-query"));
        const variablesStr = decodeURIComponent(
          btn.getAttribute("data-variables")
        );
        let variables = {};
        try {
          variables = JSON.parse(variablesStr || "{}");
        } catch (e) {
          console.warn("Failed to parse variables:", variablesStr);
        }
        this.runExplain(query, variables);
      });
    });

    const sortedByFrequency = Object.entries(queryGroups)
      .map(([normalized, data]) => ({
        normalized,
        count: data.count,
        example: data.queries[0],
        avgDuration:
          data.queries.reduce((sum, q) => sum + q.duration, 0) / data.count,
      }))
      .sort((a, b) => b.count - a.count)
      .slice(0, 5);

    document.getElementById("frequentQueries").innerHTML = sortedByFrequency
      .map(
        (item) => `
            <div class="query-item">
                <div class="query-header">
                    <span class="query-count">${item.count}x</span>
                    <span class="query-duration">${item.avgDuration.toFixed(
                      2
                    )}ms avg</span>
                </div>
                <div class="query-text">${this.escapeHtml(
                  item.example.query
                )}</div>
                <div class="query-meta">
                    <span>Table: ${item.example.table}</span>
                    <span>Op: ${item.example.operation}</span>
                </div>
                <div class="query-actions">
                    <button class="btn-explain" data-query="${encodeURIComponent(
                      item.example.query
                    )}" data-variables="${encodeURIComponent(
          JSON.stringify(item.example.variables || {})
        )}">Run EXPLAIN</button>
                </div>
            </div>
        `
      )
      .join("");

    // Add event listeners to frequent queries EXPLAIN buttons
    document
      .querySelectorAll("#frequentQueries .btn-explain")
      .forEach((btn) => {
        btn.addEventListener("click", (e) => {
          e.stopPropagation();
          const query = decodeURIComponent(btn.getAttribute("data-query"));
          const variablesStr = decodeURIComponent(
            btn.getAttribute("data-variables")
          );
          let variables = {};
          try {
            variables = JSON.parse(variablesStr || "{}");
          } catch (e) {
            console.warn("Failed to parse variables:", variablesStr);
          }
          this.runExplain(query, variables);
        });
      });

    const nPlusOne = sortedByFrequency.filter((item) => item.count > 5);
    if (nPlusOne.length > 0) {
      document.getElementById("nPlusOne").innerHTML = nPlusOne
        .map(
          (item) => `
                <div class="query-item">
                    <div class="query-header">
                        <span class="query-count">${
                          item.count
                        }x executions</span>
                    </div>
                    <div class="query-text">${this.escapeHtml(
                      item.example.query
                    )}</div>
                    <div class="query-meta">
                        <span>Table: ${item.example.table}</span>
                        <span>Consider batching or eager loading</span>
                    </div>
                </div>
            `
        )
        .join("");
    } else {
      document.getElementById("nPlusOne").innerHTML =
        '<div class="query-item">No potential N+1 issues detected</div>';
    }

    const tables = {};
    queries.forEach((q) => {
      if (!tables[q.table]) {
        tables[q.table] = 0;
      }
      tables[q.table]++;
    });

    document.getElementById("tablesAccessed").innerHTML = Object.entries(tables)
      .sort((a, b) => b[1] - a[1])
      .map(
        ([table, count]) => `
                <span class="table-badge">${table}<span class="table-count">(${count})</span></span>
            `
      )
      .join("");
  }

  showAnalyzer() {
    document.getElementById("analyzerPanel").classList.remove("hidden");
    document.querySelector(".log-viewer").classList.add("with-analyzer");
  }

  closeAnalyzer() {
    document.getElementById("analyzerPanel").classList.add("hidden");
    document.querySelector(".log-viewer").classList.remove("with-analyzer");
  }

  convertAnsiToHtml(text) {
    const ansiMap = {
      0: "",
      1: "ansi-bold",
      30: "ansi-gray",
      31: "ansi-red",
      32: "ansi-green",
      33: "ansi-yellow",
      34: "ansi-blue",
      35: "ansi-magenta",
      36: "ansi-cyan",
      37: "ansi-white",
      90: "ansi-gray",
      91: "ansi-bright-red",
      92: "ansi-bright-green",
      93: "ansi-bright-yellow",
      94: "ansi-bright-blue",
      95: "ansi-bright-magenta",
      96: "ansi-bright-cyan",
      97: "ansi-bright-white",
    };

    const parts = [];
    const regex = /\x1b\[([0-9;]+)m/g;
    let lastIndex = 0;
    let currentClasses = [];
    let match;

    while ((match = regex.exec(text)) !== null) {
      if (match.index > lastIndex) {
        const content = text.substring(lastIndex, match.index);
        if (currentClasses.length > 0) {
          parts.push(
            `<span class="${currentClasses.join(" ")}">${this.escapeHtml(
              content
            )}</span>`
          );
        } else {
          parts.push(this.escapeHtml(content));
        }
      }

      const codes = match[1].split(";");
      currentClasses = [];
      codes.forEach((code) => {
        if (ansiMap[code]) {
          currentClasses.push(ansiMap[code]);
        }
      });

      lastIndex = regex.lastIndex;
    }

    if (lastIndex < text.length) {
      const content = text.substring(lastIndex);
      if (currentClasses.length > 0) {
        parts.push(
          `<span class="${currentClasses.join(" ")}">${this.escapeHtml(
            content
          )}</span>`
        );
      } else {
        parts.push(this.escapeHtml(content));
      }
    }

    return parts.join("");
  }

  showLogDetails(log) {
    const rawContent = log.entry?.raw || "No raw log available";
    document.getElementById("rawLogContent").innerHTML =
      this.convertAnsiToHtml(rawContent);

    const parsedFieldsEl = document.getElementById("parsedFields");
    parsedFieldsEl.innerHTML = "";

    const fields = [
      { key: "Timestamp", value: log.entry?.timestamp },
      { key: "Level", value: log.entry?.level },
      { key: "File", value: log.entry?.file },
      { key: "Message", value: log.entry?.message },
    ];

    fields.forEach(({ key, value }) => {
      if (value) {
        const fieldEl = document.createElement("div");
        fieldEl.className = "parsed-field";
        fieldEl.innerHTML = `
                    <div class="parsed-field-key">${key}</div>
                    <div class="parsed-field-value">${this.escapeHtml(
                      value
                    )}</div>
                `;
        parsedFieldsEl.appendChild(fieldEl);
      }
    });

    if (log.entry?.fields) {
      Object.entries(log.entry.fields).forEach(([key, value]) => {
        const fieldEl = document.createElement("div");
        fieldEl.className = "parsed-field";

        let displayValue = value;
        const isJson =
          value.trim().startsWith("{") || value.trim().startsWith("[");

        if (isJson) {
          try {
            const parsed = JSON.parse(value);
            displayValue = `<pre>${JSON.stringify(parsed, null, 2)}</pre>`;
          } catch (e) {
            displayValue = this.escapeHtml(value);
          }
        } else {
          displayValue = this.escapeHtml(value);
        }

        fieldEl.innerHTML = `
                    <div class="parsed-field-key">${this.escapeHtml(key)}</div>
                    <div class="parsed-field-value">${displayValue}</div>
                `;
        parsedFieldsEl.appendChild(fieldEl);
      });
    }

    document.getElementById("logModal").classList.remove("hidden");
  }

  closeModal() {
    document.getElementById("logModal").classList.add("hidden");
  }

  async runExplain(query, variables = {}) {
    // Fix unquoted values before sending
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
      this.showExplainResult(result);
    } catch (error) {
      this.showExplainResult({
        error: `Failed to run EXPLAIN: ${error.message}`,
        query: query,
      });
    }
  }

  closeExplainModal() {
    if (result.error) {
      // Show error message
      const pev2El = document.getElementById('pev2App');
      pev2El.innerHTML = `<div class="alert alert-danger m-3">${this.escapeHtml(result.error)}</div>`;
    } else {
      // Convert the PostgreSQL EXPLAIN JSON format to the format PEV2 expects
      // PEV2 expects the plan as a JSON string
      let planText = '';
      if (result.queryPlan && result.queryPlan.length > 0) {
        // PostgreSQL EXPLAIN (FORMAT JSON) returns an array with one element containing the plan
        planText = JSON.stringify(result.queryPlan, null, 2);
      }
      
      // Update the PEV2 component
      if (this.pev2App && this.pev2App._instance) {
        this.pev2App._instance.proxy.updatePlan(planText, result.query || '');
      }
    }

    document.getElementById("explainModal").classList.remove("hidden");
  }

  closeExplainModal() {
    document.getElementById("explainModal").classList.add("hidden");
  }

  scrollToBottom() {
    const logsEl = document.getElementById("logs");
    logsEl.scrollTop = logsEl.scrollHeight;
  }

  updateStatus(text, connected) {
    const statusEl = document.getElementById("status");
    statusEl.textContent = text;
    statusEl.style.color = connected ? "#7ee787" : "#f85149";
  }

  updateLogCount() {
    document.getElementById(
      "logCount"
    ).textContent = `${this.logs.length} logs`;
  }

  escapeHtml(text) {
    const div = document.createElement("div");
    div.textContent = text;
    return div.innerHTML;
  }
}

const app = new DockerLogParser();
