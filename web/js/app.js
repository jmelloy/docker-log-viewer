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
    };
  },

  watch: {
    searchQuery() {
      this.sendFilterUpdate();
    },
  },

  computed: {
    filteredLogs() {
      // Backend now does filtering, just return all logs
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
    this.init();
  },

  methods: {
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
        const response = await fetch("/api/containers");
        this.containers = await response.json();
        
        // Get valid container names
        const validNames = new Set(this.containers.map(c => c.Name));
        
        if (this.selectedContainers.size === 0) {
          // No saved state, select all
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
        
        this.sendFilterUpdate();
      } catch (error) {
        console.error("Failed to load containers:", error);
      }
    },

    async loadInitialLogs() {
      try {
        const response = await fetch("/api/logs");
        const logs = await response.json();
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
      this.$nextTick(() => this.scrollToBottom());
    },

    handleNewLogs(logs) {
      // Handle batched logs from backend
      this.logs.push(...logs);
      if (this.logs.length > 100000) {
        this.logs = this.logs.slice(-50000);
      }
      this.$nextTick(() => this.scrollToBottom());
    },

    handleInitialLogs(logs) {
      // Replace all logs with initial filtered set
      console.log(`Received ${logs.length} initial filtered logs`);
      this.logs = logs;
      this.$nextTick(() => this.scrollToBottom());
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
        traceFilters: Array.from(this.traceFilters.entries()).map(([type, value]) => ({ type, value })),
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
      return projectContainers.every((c) => this.selectedContainers.has(c.Name));
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
      this.analyzeTrace();
    },

    removeTraceFilter(type) {
      this.traceFilters.delete(type);
      this.sendFilterUpdate();
      if (this.traceFilters.size === 0) {
        this.showAnalyzer = false;
      } else {
        this.analyzeTrace();
      }
    },

    clearTraceFilters() {
      this.traceFilters.clear();
      this.sendFilterUpdate();
      this.showAnalyzer = false;
    },

    async saveTrace() {
      if (this.traceFilters.size === 0) return;

      const filterDesc = Array.from(this.traceFilters.entries()).map(([k, v]) => `${k}: ${v}`).join(', ');
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
          traceId: this.traceFilters.get('trace_id') || null,
          requestId: this.traceFilters.get('request_id') || null,
          name: name,
          logs: traceLogs,
          sqlQueries: sqlQueries,
        };

        const response = await fetch('/api/save-trace', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(payload),
        });

        if (response.ok) {
          const result = await response.json();
          alert(`Trace saved successfully! ID: ${result.id}`);
        } else {
          alert('Failed to save trace');
        }
      } catch (error) {
        console.error('Error saving trace:', error);
        alert(`Failed to save trace: ${error.message}`);
      }
    },

    clearLogs() {
      this.logs = [];
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

    async runExplain(query, variables = {}, metadata = null) {
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

    async shareExplainPlan() {
      try {
        const form = document.createElement('form');
        form.method = 'POST';
        form.action = 'https://explain.dalibo.com/new';
        form.target = '_blank';
        
        // Build descriptive title from metadata
        let title = 'Query Plan from Logseidon';
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
            title = parts.join(' ') + ' - Logseidon';
          }
        }
        
        const titleInput = document.createElement('input');
        titleInput.type = 'hidden';
        titleInput.name = 'title';
        titleInput.value = title;
        form.appendChild(titleInput);
        
        const planInput = document.createElement('input');
        planInput.type = 'hidden';
        planInput.name = 'plan';
        planInput.value = this.explainData.planSource;
        form.appendChild(planInput);
        
        const queryInput = document.createElement('input');
        queryInput.type = 'hidden';
        queryInput.name = 'query';
        queryInput.value = this.explainData.planQuery;
        form.appendChild(queryInput);
        
        document.body.appendChild(form);
        form.submit();
        document.body.removeChild(form);
      } catch (error) {
        console.error('Error sharing plan:', error);
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

  template: `
    <div class="app-container">
      <header class="app-header">
        <div style="display: flex; align-items: center; gap: 1rem; width: 100%;">
          <h1 style="margin: 0">🔱 Logseidon</h1>
          <nav style="display: flex; gap: 1rem; align-items: center;">
            <a href="/" class="active">Log Viewer</a>
            <a href="/requests.html">Request Manager</a>
            <a href="/settings.html">Settings</a>
          </nav>
          <div class="header-controls">
            <div class="search-box">
              <input type="text" v-model="searchQuery" placeholder="Search logs...">
              <button @click="searchQuery = ''" class="clear-btn" title="Clear search">✕</button>
            </div>
            <div class="trace-filter-display" v-if="hasTraceFilters">
              <span v-for="([key, value], index) in Array.from(traceFilters.entries())" :key="key" class="trace-filter-badge">
                <span class="filter-key">{{ key }}</span>=<span class="filter-value">{{ value }}</span>
                <button @click="removeTraceFilter(key)" class="filter-remove" title="Remove filter">×</button>
              </span>
              <button @click="saveTrace" class="btn-star" title="Save trace to request manager">⭐</button>
              <button @click="clearTraceFilters" class="clear-btn" title="Clear all filters">✕</button>
            </div>
          </div>
        </div>
      </header>
      
      <div class="main-layout">
        <aside class="sidebar">
          <!-- SQL Query Analyzer Section -->
          <div v-if="showAnalyzer" class="section analyzer-section-container">
            <div class="section-header">
              <h3>SQL Query Analyzer</h3>
              <button @click="showAnalyzer = false" class="close-analyzer-btn">✕</button>
            </div>
            <div v-if="sqlAnalysis" class="analyzer-content-compact">
              <div class="analyzer-subsection">
                <h4>Overview</h4>
                <div class="stats-grid-compact">
                  <div class="stat-item">
                    <span class="stat-label">Total Queries</span>
                    <span class="stat-value">{{ sqlAnalysis.totalQueries }}</span>
                  </div>
                  <div class="stat-item">
                    <span class="stat-label">Unique</span>
                    <span class="stat-value">{{ sqlAnalysis.uniqueQueries }}</span>
                  </div>
                  <div class="stat-item">
                    <span class="stat-label">Avg Duration</span>
                    <span class="stat-value">{{ sqlAnalysis.avgDuration.toFixed(2) }}ms</span>
                  </div>
                  <div class="stat-item">
                    <span class="stat-label">Total Duration</span>
                    <span class="stat-value">{{ sqlAnalysis.totalDuration.toFixed(2) }}ms</span>
                  </div>
                </div>
              </div>

              <div class="analyzer-subsection">
                <h4>Slowest Queries</h4>
                <div class="query-list-compact">
                  <div v-if="sqlAnalysis.slowestQueries.length === 0" class="query-item-compact">No SQL queries</div>
                  <div v-for="(q, index) in sqlAnalysis.slowestQueries.slice(0, 3)" :key="index" class="query-item-compact">
                    <div class="query-header-compact">
                      <span class="query-duration" :class="{ 'query-slow': q.duration > 10 }">{{ q.duration.toFixed(2) }}ms</span>
                      <span class="query-meta-inline">{{ q.table }} · {{ q.operation }}</span>
                    </div>
                    <div class="query-text-compact">{{ q.query.substring(0, 60) }}{{ q.query.length > 60 ? '...' : '' }}</div>
                    <button class="btn-explain-compact" @click="runExplain(q.query, q.variables, { table: q.table, operation: q.operation, type: 'slowest' })">EXPLAIN</button>
                  </div>
                </div>
              </div>

              <div class="analyzer-subsection">
                <h4>Most Frequent</h4>
                <div class="query-list-compact">
                  <div v-for="(item, index) in sqlAnalysis.frequentQueries.slice(0, 3)" :key="index" class="query-item-compact">
                    <div class="query-header-compact">
                      <span class="query-count">{{ item.count }}x</span>
                      <span class="query-meta-inline">{{ item.example.table }} · {{ item.avgDuration.toFixed(2) }}ms</span>
                    </div>
                    <div class="query-text-compact">{{ item.example.query.substring(0, 60) }}{{ item.example.query.length > 60 ? '...' : '' }}</div>
                    <button class="btn-explain-compact" @click="runExplain(item.example.query, item.example.variables, { table: item.example.table, operation: item.example.operation, type: 'frequent' })">EXPLAIN</button>
                  </div>
                </div>
              </div>

              <div v-if="sqlAnalysis.nPlusOne.length > 0" class="analyzer-subsection">
                <h4>N+1 Issues ({{ sqlAnalysis.nPlusOne.length }})</h4>
                <div class="query-list-compact">
                  <div v-for="(item, index) in sqlAnalysis.nPlusOne.slice(0, 2)" :key="index" class="query-item-compact nplusone">
                    <div class="query-header-compact">
                      <span class="query-count">{{ item.count }}x</span>
                      <span class="query-meta-inline">{{ item.example.table }}</span>
                    </div>
                    <div class="query-text-compact">{{ item.example.query.substring(0, 60) }}...</div>
                  </div>
                </div>
              </div>

              <div class="analyzer-subsection">
                <h4>Tables</h4>
                <div class="table-list-compact">
                  <span v-for="(item, index) in sqlAnalysis.tables" :key="index" class="table-badge-compact">
                    {{ item.table }}<span class="table-count">({{ item.count }})</span>
                  </span>
                </div>
              </div>
            </div>
          </div>

          <div class="section">
            <h3>Containers</h3>
            <div class="container-list">
              <div v-for="project in projectNames" :key="project" class="project-section">
                <div class="project-header" @click="toggleProjectCollapse(project)">
                  <span class="disclosure-arrow" :class="{ collapsed: isProjectCollapsed(project) }">▼</span>
                  <div class="checkbox" 
                       :class="{ checked: isProjectSelected(project), indeterminate: isProjectIndeterminate(project) }"
                       @click.stop="toggleProject(project)"></div>
                  <span class="project-name">{{ project }}</span>
                  <span class="project-count">({{ containersByProject[project].length }})</span>
                </div>
                <div class="project-containers" :class="{ collapsed: isProjectCollapsed(project) }">
                  <div v-for="container in containersByProject[project]" 
                       :key="container.Name"
                       class="container-item"
                       :class="{ selected: isContainerSelected(container.Name) }"
                       @click="toggleContainer(container.Name)">
                    <div class="checkbox" :class="{ checked: isContainerSelected(container.Name) }"></div>
                    <div class="container-info">
                      <div class="container-name">{{ container.Name }}</div>
                      <div class="container-id">{{ container.ID }}</div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="section">
            <h3>Log Levels</h3>
            <div class="level-filters">
              <label class="level-filter">
                <input type="checkbox" value="TRC" :checked="isLevelSelected('TRC')" @change="toggleLevel('TRC')">
                <span class="level-badge level-trc">TRC</span>
              </label>
              <label class="level-filter">
                <input type="checkbox" value="DBG" :checked="isLevelSelected('DBG')" @change="toggleLevel('DBG')">
                <span class="level-badge level-dbg">DBG</span>
              </label>
              <label class="level-filter">
                <input type="checkbox" value="INF" :checked="isLevelSelected('INF')" @change="toggleLevel('INF')">
                <span class="level-badge level-inf">INF</span>
              </label>
              <label class="level-filter">
                <input type="checkbox" value="WRN" :checked="isLevelSelected('WRN')" @change="toggleLevel('WRN')">
                <span class="level-badge level-wrn">WRN</span>
              </label>
              <label class="level-filter">
                <input type="checkbox" value="ERR" :checked="isLevelSelected('ERR')" @change="toggleLevel('ERR')">
                <span class="level-badge level-err">ERR</span>
              </label>
              <label class="level-filter">
                <input type="checkbox" value="NONE" :checked="isLevelSelected('NONE')" @change="toggleLevel('NONE')">
                <span class="level-badge level-none">NONE</span>
              </label>
            </div>
          </div>

          <div class="section">
            <h3>Actions</h3>
            <button @click="clearLogs">Clear Logs</button>
          </div>

          <div class="sidebar-footer">
            <div class="status">
              <span :style="{ color: statusColor }">{{ statusText }}</span>
              <span>{{ logCountText }}</span>
            </div>
          </div>
        </aside>

        <main class="log-viewer">
          <div ref="logsContainer" class="logs">
            <div v-for="(log, index) in filteredLogs" 
                 :key="index"
                 class="log-line"
                 @click="openLogDetails(log)">
              <span class="log-container">{{ getContainerName(log.containerId) }}</span>
              <span v-if="log.entry?.timestamp" class="log-timestamp">{{ log.entry.timestamp }}</span>
              <span v-if="log.entry?.level" class="log-level" :class="log.entry.level">{{ log.entry.level }}</span>
              <span v-if="log.entry?.file" class="log-file">{{ log.entry.file }}</span>
              <span v-if="log.entry?.message" class="log-message">{{ log.entry.message }}</span>
              <span v-for="([key, value], idx) in Object.entries(log.entry?.fields || {})" 
                    :key="idx"
                    class="log-field">
                <span class="log-field-key">{{ key }}</span>=<span 
                  :class="{ 'log-field-value': !isJsonField(value) }"
                  @click.stop="!isJsonField(value) && setTraceFilter(key, value, $event)">{{ formatFieldValue(value) }}</span>
              </span>
            </div>
          </div>
        </main>


      </div>
    </div>

    <!-- Log Details Modal -->
    <div v-if="showLogModal" class="modal" @click="showLogModal = false">
      <div class="modal-content" @click.stop>
        <div class="modal-header">
          <h3>Log Details</h3>
          <button @click="showLogModal = false">✕</button>
        </div>
        <div v-if="selectedLog" class="modal-body">
          <div class="modal-section">
            <h4>Raw Log</h4>
            <pre v-html="convertAnsiToHtml(selectedLog.entry?.raw || 'No raw log available')"></pre>
          </div>
          <div class="modal-section">
            <h4>Parsed Fields</h4>
            <div>
              <div v-if="selectedLog.entry?.timestamp" class="parsed-field">
                <div class="parsed-field-key">Timestamp</div>
                <div class="parsed-field-value">{{ selectedLog.entry.timestamp }}</div>
              </div>
              <div v-if="selectedLog.entry?.level" class="parsed-field">
                <div class="parsed-field-key">Level</div>
                <div class="parsed-field-value">{{ selectedLog.entry.level }}</div>
              </div>
              <div v-if="selectedLog.entry?.file" class="parsed-field">
                <div class="parsed-field-key">File</div>
                <div class="parsed-field-value">{{ selectedLog.entry.file }}</div>
              </div>
              <div v-if="selectedLog.entry?.message" class="parsed-field">
                <div class="parsed-field-key">Message</div>
                <div class="parsed-field-value">{{ selectedLog.entry.message }}</div>
              </div>
              <div v-for="([key, value], idx) in Object.entries(selectedLog.entry?.fields || {})" :key="idx" class="parsed-field">
                <div class="parsed-field-key">{{ key }}</div>
                <div v-if="isJsonField(value)" class="parsed-field-value">
                  <pre>{{ formatJsonField(value) }}</pre>
                </div>
                <div v-else class="parsed-field-value">{{ value }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- EXPLAIN Modal -->
    <div v-if="showExplainModal" class="modal" @click="showExplainModal = false">
      <div class="modal-content explain-modal-content" @click.stop>
        <div class="modal-header">
          <h3>SQL Query Explain Plan (PEV2)</h3>
          <div style="display: flex; gap: 0.5rem;">
            <button v-if="!explainData.error" @click="shareExplainPlan" class="btn-share" title="Share on explain.dalibo.com">🔗 Share</button>
            <button @click="showExplainModal = false">✕</button>
          </div>
        </div>
        <div class="modal-body">
          <div v-if="explainData.error" class="alert alert-danger" style="margin: 1rem;">{{ explainData.error }}</div>
          <div v-else id="pev2App" class="d-flex flex-column">
            <pev2 :plan-source="explainData.planSource" :plan-query="explainData.planQuery"></pev2>
          </div>
        </div>
      </div>
    </div>
  `,
});

// Register PEV2 component
app.component("pev2", pev2.Plan);

// Mount the app
app.mount("#app");
