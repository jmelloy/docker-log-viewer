import { createAppHeader } from "./shared/navigation.js";
import { API } from "./shared/api.js";
import { loadTemplate } from "./shared/template-loader.js";

const template = await loadTemplate("/templates/app-main.html");

const { createApp } = Vue;

const app = createApp({
  data() {
    // Load persisted container state from localStorage (by name, not ID)
    let selectedContainers = new Set();
    try {
      const saved = localStorage.getItem("selectedContainers");
      if (saved) {
        selectedContainers = new Set(JSON.parse(saved));
      }
    } catch (e) {
      console.warn("Failed to load container state:", e);
    }

    return {
      containers: [],
      selectedContainers,
      logs: [],
      searchQuery: "",
      traceFilters: new Map(), // Map<fieldName, fieldValue>
      selectedLevels: new Set([
        "DBG",
        "DEBUG",
        "TRC",
        "TRACE",
        "INF",
        "INFO",
        "WRN",
        "WARN",
        "ERR",
        "ERROR",
        "FATAL",
        "NONE",
      ]),
      ws: null,
      wsConnected: false,
      showLogModal: false,
      showExplainModal: false,
      showAnalyzer: false,
      selectedLog: null,
      explainData: {
        planSource: "",
        planQuery: "",
        error: null,
        metadata: null,
      },
      sqlAnalysis: null,
      collapsedProjects: new Set(),
      portToServerMap: {}, // Map of port -> connectionString
      recentRequests: [], // Last 5 unique request IDs with paths
      logCounts: {}, // Map of container name -> log count
      retentions: {}, // Map of container name -> retention settings
      showRetentionModal: false,
      retentionContainer: null,
      retentionForm: {
        type: "count",
        value: 1000,
      },
    };
  },

  watch: {
    searchQuery() {
      this.sendFilterUpdate();
      this.updateURL();
    },
  },

  computed: {
    filteredLogs() {
      return this.logs;
    },

    containersByProject() {
      const groups = {};
      this.containers.forEach((container) => {
        const project = this.getProjectName(container.Name);
        if (!groups[project]) {
          groups[project] = [];
        }
        groups[project].push(container);
      });
      // Sort containers within each project by name
      Object.keys(groups).forEach((project) => {
        groups[project].sort((a, b) => a.Name.localeCompare(b.Name));
      });
      return groups;
    },

    projectNames() {
      return Object.keys(this.containersByProject).sort();
    },

    statusText() {
      return this.wsConnected ? "Connected" : "Connecting...";
    },

    statusColor() {
      return this.wsConnected ? "#7ee787" : "#f85149";
    },

    hasTraceFilters() {
      return this.traceFilters.size > 0;
    },

    logCountText() {
      return `${this.filteredLogs.length} logs`;
    },
  },

  mounted() {
    this.parseURLParameters();
    this.init();
  },

  updated() {
    // Apply syntax highlighting after DOM updates
    this.$nextTick(() => {
      this.applySyntaxHighlighting();
    });
  },

  methods: {
    applySyntaxHighlighting() {
      // Only apply if hljs is available
      if (typeof hljs === "undefined") return;

      // Highlight SQL queries in analyzer
      document.querySelectorAll(".query-text-compact").forEach((block) => {
        if (!block.classList.contains("hljs")) {
          const text = block.textContent;
          const highlighted = hljs.highlight(text, { language: "sql" });
          block.innerHTML = highlighted.value;
          block.classList.add("hljs");
        }
      });
    },
    parseURLParameters() {
      const params = new URLSearchParams(window.location.search);

      // Parse search query parameter
      const queryParam = params.get("query");
      if (queryParam) {
        this.searchQuery = queryParam;
      }

      // Parse trace filter parameters (e.g., ?trace_request_id=abc123 or ?request_id=abc123)
      const traceParamNames = ["request_id", "trace_id", "span_id"];
      for (const paramName of traceParamNames) {
        const value = params.get(paramName) || params.get(`trace_${paramName}`);
        if (value) {
          this.traceFilters.set(paramName, value);
        }
      }
    },

    updateURL() {
      const params = new URLSearchParams();

      // Add search query
      if (this.searchQuery) {
        params.set("query", this.searchQuery);
      }

      // Add trace filters
      for (const [key, value] of this.traceFilters.entries()) {
        params.set(key, value);
      }

      const newURL = params.toString()
        ? `${window.location.pathname}?${params.toString()}`
        : window.location.pathname;

      // Update URL without reloading
      window.history.replaceState({}, "", newURL);
    },

    async init() {
      await this.loadContainers();
      this.connectWebSocket();
      // Initial logs will come via WebSocket after filter is sent
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
      const hasAll = levelVariants.every((v) => this.selectedLevels.has(v));

      if (hasAll) {
        levelVariants.forEach((v) => this.selectedLevels.delete(v));
      } else {
        levelVariants.forEach((v) => this.selectedLevels.add(v));
      }
      this.sendFilterUpdate();
    },

    isLevelSelected(level) {
      const levelVariants = this.getLevelVariants(level);
      return levelVariants.every((v) => this.selectedLevels.has(v));
    },

    async loadContainers() {
      try {
        const data = await API.get("/api/containers");

        // Handle both old format (array) and new format (object with containers and portToServerMap)
        if (Array.isArray(data)) {
          this.containers = data;
        } else {
          this.containers = data.containers || [];
          this.portToServerMap = data.portToServerMap || {};
          this.logCounts = data.logCounts || {};
          this.retentions = data.retentions || {};
          console.log("Loaded port to server map:", this.portToServerMap);
        }

        // Get valid container names
        const validNames = new Set(this.containers.map((c) => c.Name));

        // Check if this is first load (no localStorage key exists)
        const isFirstLoad = !localStorage.getItem("selectedContainers");

        if (isFirstLoad && this.selectedContainers.size === 0) {
          // First load with no saved state, select all
          this.containers.forEach((c) => this.selectedContainers.add(c.Name));
          this.saveContainerState();
        } else {
          // Validate saved state - remove invalid entries (old IDs)
          const validSelections = new Set();
          for (const name of this.selectedContainers) {
            if (validNames.has(name)) {
              validSelections.add(name);
            }
          }

          // If everything was invalid, select all
          if (validSelections.size === 0) {
            this.containers.forEach((c) => validSelections.add(c.Name));
          }

          this.selectedContainers = validSelections;
          this.saveContainerState();
        }

        // Don't send filter update here - will be sent when WebSocket connects
      } catch (error) {
        console.error("Failed to load containers:", error);
      }
    },

    async loadInitialLogs() {
      try {
        const logs = await API.get("/api/logs");
        this.logs = logs;
        this.$nextTick(() => this.scrollToBottom());
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
        this.sendFilterUpdate();
      };

      this.ws.onmessage = (event) => {
        const message = JSON.parse(event.data);
        if (message.type === "log") {
          this.handleNewLog(message.data);
        } else if (message.type === "logs") {
          this.handleNewLogs(message.data);
        } else if (message.type === "logs_initial") {
          this.handleInitialLogs(message.data);
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
      if (this.logs.length > 100000) {
        this.logs = this.logs.slice(-50000);
      }
      this.updateRecentRequests(log);
      this.$nextTick(() => this.scrollToBottom());
    },

    handleNewLogs(logs) {
      // Handle batched logs from backend
      this.logs.push(...logs);
      if (this.logs.length > 100000) {
        this.logs = this.logs.slice(-50000);
      }
      logs.forEach((log) => this.updateRecentRequests(log));
      this.$nextTick(() => this.scrollToBottom());
    },

    handleInitialLogs(logs) {
      // Replace all logs with initial filtered set
      console.log(`Received ${logs.length} initial filtered logs`);
      this.logs = logs;
      logs.forEach((log) => this.updateRecentRequests(log));
      this.$nextTick(() => this.scrollToBottom());
    },

    updateRecentRequests(log) {
      const requestId = log.entry?.fields?.request_id;
      const path = log.entry?.fields?.path;
      const operationName =
        log.entry?.fields?.operation_name ||
        log.entry?.fields?.operationName ||
        log.entry?.fields?.["gql.operationName"];
      const method = log.entry?.fields?.method;
      const statusCode =
        log.entry?.fields?.status_code || log.entry?.fields?.statusCode;
      const latency =
        log.entry?.fields?.latency ||
        log.entry?.fields?.duration ||
        log.entry?.fields?.["request.duration"];
      const timestamp = log.entry?.timestamp || log.timestamp;

      if (!requestId || !path) return;

      if (latency && latency < 5) return;

      // Check if request ID already exists
      const existingIndex = this.recentRequests.findIndex(
        (r) => r.requestId === requestId
      );

      if (existingIndex !== -1) {
        // Update existing request with new info
        const existing = this.recentRequests[existingIndex];

        // Add operation name if we have one and it's not already in the set
        if (operationName && !existing.operations.includes(operationName)) {
          existing.operations.push(operationName);
        }

        // Update status code if we have one
        if (statusCode) {
          existing.statusCode = statusCode;
        }

        // Update latency if we have one
        if (latency) {
          existing.latency = latency;
        }

        // Update timestamp to latest
        if (timestamp) {
          existing.timestamp = timestamp;
        }

        // Move to front
        this.recentRequests.splice(existingIndex, 1);
        this.recentRequests.unshift(existing);
      } else {
        // Add new request to front
        this.recentRequests.unshift({
          requestId,
          path: path,
          operations: operationName ? [operationName] : [],
          method: method || "POST",
          statusCode: statusCode || null,
          latency: latency || null,
          timestamp: timestamp || new Date().toLocaleTimeString(),
        });

        // Keep only last 5
        if (this.recentRequests.length > 5) {
          this.recentRequests.pop();
        }
      }

      // Also check recent logs for this request_id to find gql.operationName
      if (requestId && !operationName) {
        const recentLogs = this.logs.slice(-100); // Check last 100 logs
        for (const recentLog of recentLogs) {
          if (recentLog.entry?.fields?.request_id === requestId) {
            const gqlOp = recentLog.entry?.fields?.["gql.operationName"];
            if (gqlOp) {
              const idx = this.recentRequests.findIndex(
                (r) => r.requestId === requestId
              );
              if (
                idx !== -1 &&
                !this.recentRequests[idx].operations.includes(gqlOp)
              ) {
                this.recentRequests[idx].operations.push(gqlOp);
              }
            }
          }
        }
      }
    },

    sendFilterUpdate() {
      if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
        console.log("Cannot send filter update - WebSocket not connected");
        return;
      }

      const filter = {
        selectedContainers: Array.from(this.selectedContainers),
        selectedLevels: Array.from(this.selectedLevels),
        searchQuery: this.searchQuery,
        traceFilters: Array.from(this.traceFilters.entries()).map(
          ([type, value]) => ({ type, value })
        ),
      };

      console.log("Sending filter update:", filter);

      this.ws.send(
        JSON.stringify({
          type: "filter",
          data: filter,
        })
      );
    },

    handleContainerUpdate(data) {
      const newContainers = data.containers;
      const oldNames = new Set(this.containers.map((c) => c.Name));
      const newNames = new Set(newContainers.map((c) => c.Name));

      const added = newContainers.filter((c) => !oldNames.has(c.Name));
      const removed = this.containers.filter((c) => !newNames.has(c.Name));

      if (added.length > 0) {
        added.forEach((c) => {
          this.selectedContainers.add(c.Name);
          console.log(`Container started: ${c.Name} (${c.ID})`);
        });
        this.sendFilterUpdate();
      }

      if (removed.length > 0) {
        removed.forEach((c) => {
          this.selectedContainers.delete(c.Name);
          console.log(`Container stopped: ${c.Name} (${c.ID})`);
        });
        this.sendFilterUpdate();
      }

      this.containers = newContainers;

      // Update port to server mapping
      if (data.portToServerMap) {
        this.portToServerMap = data.portToServerMap;
        console.log("Updated port to server map:", this.portToServerMap);
      }

      // Update log counts
      if (data.logCounts) {
        this.logCounts = data.logCounts;
        console.log("Updated log counts:", this.logCounts);
      }

      // Update retentions
      if (data.retentions) {
        this.retentions = data.retentions;
        console.log("Updated retentions:", this.retentions);
      }

      if (this.hasTraceFilters) {
        this.analyzeTrace();
      }
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

    toggleContainer(containerName) {
      if (this.selectedContainers.has(containerName)) {
        this.selectedContainers.delete(containerName);
      } else {
        this.selectedContainers.add(containerName);
      }
      this.saveContainerState();
      this.sendFilterUpdate();
    },

    isContainerSelected(containerName) {
      return this.selectedContainers.has(containerName);
    },

    toggleProject(project) {
      const projectContainers = this.containersByProject[project];
      const allSelected = projectContainers.every((c) =>
        this.selectedContainers.has(c.Name)
      );

      projectContainers.forEach((c) => {
        if (allSelected) {
          this.selectedContainers.delete(c.Name);
        } else {
          this.selectedContainers.add(c.Name);
        }
      });
      this.saveContainerState();
      this.sendFilterUpdate();
    },

    isProjectSelected(project) {
      const projectContainers = this.containersByProject[project];
      return projectContainers.every((c) =>
        this.selectedContainers.has(c.Name)
      );
    },

    isProjectIndeterminate(project) {
      const projectContainers = this.containersByProject[project];
      const someSelected = projectContainers.some((c) =>
        this.selectedContainers.has(c.Name)
      );
      const allSelected = projectContainers.every((c) =>
        this.selectedContainers.has(c.Name)
      );
      return someSelected && !allSelected;
    },

    toggleProjectCollapse(project) {
      if (this.collapsedProjects.has(project)) {
        this.collapsedProjects.delete(project);
      } else {
        this.collapsedProjects.add(project);
      }
    },

    isProjectCollapsed(project) {
      return this.collapsedProjects.has(project);
    },

    getContainerName(containerId) {
      return (
        this.containers.find((c) => c.ID === containerId)?.Name || containerId
      );
    },

    setTraceFilter(type, value, event) {
      if (event) event.stopPropagation();
      this.traceFilters.set(type, value);
      this.sendFilterUpdate();
      this.updateURL();
      this.analyzeTrace();
    },

    removeTraceFilter(type) {
      this.traceFilters.delete(type);
      this.sendFilterUpdate();
      this.updateURL();
      if (this.traceFilters.size === 0) {
        this.showAnalyzer = false;
      } else {
        this.analyzeTrace();
      }
    },

    clearTraceFilters() {
      this.traceFilters.clear();
      this.sendFilterUpdate();
      this.updateURL();
      this.showAnalyzer = false;
    },

    async saveTrace() {
      if (this.traceFilters.size === 0) return;

      const filterDesc = Array.from(this.traceFilters.entries())
        .map(([k, v]) => `${k}: ${v}`)
        .join(", ");
      const name = prompt(`Save trace as:`, filterDesc);
      if (!name) return;

      try {
        // Get all logs for this trace
        const traceLogs = this.logs.filter((log) => {
          const containerName = this.getContainerName(log.containerId);
          if (!this.selectedContainers.has(containerName)) {
            return false;
          }
          // Match ALL filters
          for (const [type, value] of this.traceFilters.entries()) {
            const val = log.entry?.fields?.[type];
            if (val !== value) return false;
          }
          return true;
        });

        // Extract SQL queries
        const sqlQueries = this.extractSQLQueries(traceLogs);

        const payload = {
          traceId: this.traceFilters.get("trace_id") || null,
          requestId: this.traceFilters.get("request_id") || null,
          name: name,
          logs: traceLogs,
          sqlQueries: sqlQueries,
        };

        const result = await API.post("/api/save-trace", payload);
        alert(`Trace saved successfully! ID: ${result.id}`);
      } catch (error) {
        console.error("Error saving trace:", error);
        alert(`Failed to save trace: ${error.message}`);
      }
    },

    analyzeTrace() {
      if (this.traceFilters.size === 0) {
        this.showAnalyzer = false;
        return;
      }

      const traceLogs = this.logs.filter((log) => {
        const containerName = this.getContainerName(log.containerId);
        if (!this.selectedContainers.has(containerName)) {
          return false;
        }
        // Match ALL filters
        for (const [type, value] of this.traceFilters.entries()) {
          const val = log.entry?.fields?.[type];
          if (val !== value) return false;
        }
        return true;
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

            let variables = {};
            const dbVars = log.entry?.fields?.["db.vars"];
            if (dbVars) {
              try {
                const varsArray =
                  typeof dbVars === "string" ? JSON.parse(dbVars) : dbVars;
                if (Array.isArray(varsArray)) {
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
    },

    normalizeQuery(query) {
      return query
        .replace(/\$\d+/g, "$N")
        .replace(/'[^']*'/g, "'?'")
        .replace(/\d+/g, "N")
        .replace(/\s+/g, " ")
        .trim();
    },

    renderAnalysis(queries) {
      if (queries.length === 0) {
        this.sqlAnalysis = {
          totalQueries: 0,
          uniqueQueries: 0,
          avgDuration: 0,
          totalDuration: 0,
          slowestQueries: [],
          frequentQueries: [],
          nPlusOne: [],
          tables: [],
        };
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

      this.sqlAnalysis = {
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

    openLogDetails(log) {
      this.selectedLog = log;
      this.showLogModal = true;
    },

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
    },

    getDatabaseConnectionString() {
      // Find the first container port that maps to a database
      for (const container of this.containers) {
        if (!container.Ports) continue;

        for (const port of container.Ports) {
          if (port.publicPort && this.portToServerMap[port.publicPort]) {
            console.log(
              `Using database for port ${port.publicPort}:`,
              this.portToServerMap[port.publicPort]
            );
            return this.portToServerMap[port.publicPort];
          }
        }
      }

      // No matching port found, return empty to use default
      console.log("No port mapping found, using default database connection");
      return "";
    },

    async runExplain(query, variables = {}, metadata = null) {
      try {
        // Determine connection string based on container ports
        const connectionString = this.getDatabaseConnectionString();

        const payload = {
          query: query,
          variables: variables,
          connectionString: connectionString,
        };

        const result = await API.post("/api/explain", payload);

        if (result.error) {
          this.explainData.error = result.error;
          this.explainData.planSource = "";
          this.explainData.planQuery = result.query || "";
          this.explainData.metadata = metadata;
        } else {
          let planText = "";
          if (result.queryPlan && result.queryPlan.length > 0) {
            planText = JSON.stringify(result.queryPlan, null, 2);
          }

          this.explainData.error = null;
          this.explainData.planSource = planText;
          this.explainData.planQuery = result.query || "";
          this.explainData.metadata = metadata;
        }

        this.showExplainModal = true;
      } catch (error) {
        this.explainData.error = `Failed to run EXPLAIN: ${error.message}`;
        this.explainData.planSource = "";
        this.explainData.planQuery = query;
        this.explainData.metadata = metadata;
        this.showExplainModal = true;
      }
    },

    openRetentionModal(containerName) {
      this.retentionContainer = containerName;
      const existing = this.retentions[containerName];
      if (existing) {
        this.retentionForm.type = existing.type;
        this.retentionForm.value = existing.value;
      } else {
        this.retentionForm.type = "count";
        this.retentionForm.value = 1000;
      }
      this.showRetentionModal = true;
    },

    getRetentionTooltip(containerName) {
      const retention = this.retentions[containerName];
      if (!retention) {
        return "Click to set retention policy";
      }
      if (retention.type === "count") {
        return `Retention: ${retention.value} logs`;
      } else {
        return `Retention: ${retention.value} seconds (${Math.floor(
          retention.value / 3600
        )}h ${Math.floor((retention.value % 3600) / 60)}m)`;
      }
    },

    async saveRetention() {
      try {
        const data = await API.post("/api/retention", {
          containerName: this.retentionContainer,
          retentionType: this.retentionForm.type,
          retentionValue: this.retentionForm.value,
        });

        this.retentions[this.retentionContainer] = {
          type: data.retentionType,
          value: data.retentionValue,
        };
        this.showRetentionModal = false;
      } catch (error) {
        console.error("Error saving retention:", error);
      }
    },

    async deleteRetention() {
      try {
        await API.delete(
          `/api/retention/${encodeURIComponent(this.retentionContainer)}`
        );
        delete this.retentions[this.retentionContainer];
        this.showRetentionModal = false;
      } catch (error) {
        console.error("Error deleting retention:", error);
      }
    },

    async shareExplainPlan() {
      try {
        const form = document.createElement("form");
        form.method = "POST";
        form.action = "https://explain.dalibo.com/new";
        form.target = "_blank";

        // Build descriptive title from metadata
        let title = "Query Plan from Logseidon";
        if (this.explainData.metadata) {
          const parts = [];
          if (this.explainData.metadata.type) {
            parts.push(this.explainData.metadata.type);
          }
          if (this.explainData.metadata.operation) {
            parts.push(this.explainData.metadata.operation.toUpperCase());
          }
          if (this.explainData.metadata.table) {
            parts.push(`on ${this.explainData.metadata.table}`);
          }
          if (parts.length > 0) {
            title = parts.join(" ") + " - Logseidon";
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
        planInput.value = this.explainData.planSource;
        form.appendChild(planInput);

        const queryInput = document.createElement("input");
        queryInput.type = "hidden";
        queryInput.name = "query";
        queryInput.value = this.explainData.planQuery;
        form.appendChild(queryInput);

        document.body.appendChild(form);
        form.submit();
        document.body.removeChild(form);
      } catch (error) {
        console.error("Error sharing plan:", error);
        alert(`Failed to share plan: ${error.message}`);
      }
    },

    scrollToBottom() {
      const logsEl = this.$refs.logsContainer;
      if (logsEl) {
        logsEl.scrollTop = logsEl.scrollHeight;
      }
    },

    escapeHtml(text) {
      const div = document.createElement("div");
      div.textContent = text;
      return div.innerHTML;
    },

    formatFieldValue(value) {
      const shortValue =
        value.length > 100 ? value.substring(0, 100) + "..." : value;
      return shortValue;
    },

    isJsonField(value) {
      return value.trim().startsWith("{") || value.trim().startsWith("[");
    },

    formatJsonField(value) {
      try {
        const parsed = JSON.parse(value);
        return JSON.stringify(parsed, null, 2);
      } catch (e) {
        return value;
      }
    },

    saveContainerState() {
      try {
        localStorage.setItem(
          "selectedContainers",
          JSON.stringify([...this.selectedContainers])
        );
      } catch (e) {
        console.warn("Failed to save container state:", e);
      }
    },
  },

  template,
});

app.component("app-header", createAppHeader("viewer"));
app.component("pev2", pev2.Plan);

app.mount("#app");
