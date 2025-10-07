class DockerLogParser {
  constructor() {
    this.containers = [];
    this.selectedContainers = new Set();
    this.logs = [];
    this.searchQuery = "";
    this.traceFilter = null;
    this.selectedLevels = new Set([
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
    ]);
    this.ws = null;

    this.init();
  }

  async init() {
    this.setupEventListeners();
    await this.loadContainers();
    this.connectWebSocket();
    await this.loadInitialLogs();
  }

  setupEventListeners() {
    document.getElementById("searchInput").addEventListener("input", (e) => {
      this.searchQuery = e.target.value.toLowerCase();
      this.renderLogs();
    });

    document
      .querySelectorAll('.level-filter input[type="checkbox"]')
      .forEach((checkbox) => {
        checkbox.addEventListener("change", (e) => {
          const level = e.target.value;
          const levelVariants = this.getLevelVariants(level);

          if (e.target.checked) {
            levelVariants.forEach((v) => this.selectedLevels.add(v));
          } else {
            levelVariants.forEach((v) => this.selectedLevels.delete(v));
          }
          this.renderLogs();
        });
      });

    document.getElementById("clearSearch").addEventListener("click", () => {
      document.getElementById("searchInput").value = "";
      this.searchQuery = "";
      this.renderLogs();
    });

    document.getElementById("clearFilter").addEventListener("click", () => {
      this.clearTraceFilter();
    });

    document.getElementById("clearLogs").addEventListener("click", () => {
      this.logs = [];
      this.renderLogs();
    });

    document.getElementById("closeAnalyzer").addEventListener("click", () => {
      this.closeAnalyzer();
    });

    document.getElementById("closeModal").addEventListener("click", () => {
      this.closeModal();
    });

    document.getElementById("logModal").addEventListener("click", (e) => {
      if (e.target.id === "logModal") {
        this.closeModal();
      }
    });

    document.addEventListener("keydown", (e) => {
      if (e.key === "Escape") {
        this.closeModal();
      }
    });
  }

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
  }

  async loadContainers() {
    try {
      const response = await fetch("/api/containers");
      this.containers = await response.json();
      this.containers.forEach((c) => this.selectedContainers.add(c.ID));
      this.renderContainers();
    } catch (error) {
      console.error("Failed to load containers:", error);
    }
  }

  async loadInitialLogs() {
    try {
      const response = await fetch("/api/logs");
      const logs = await response.json();
      this.logs = logs;
      this.renderLogs();
    } catch (error) {
      console.error("Failed to load logs:", error);
    }
  }

  connectWebSocket() {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const wsUrl = `${protocol}//${window.location.host}/api/ws`;

    this.ws = new WebSocket(wsUrl);

    this.ws.onopen = () => {
      this.updateStatus("Connected", true);
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
      this.updateStatus("Disconnected", false);
      setTimeout(() => this.connectWebSocket(), 5000);
    };

    this.ws.onerror = (error) => {
      console.error("WebSocket error:", error);
    };
  }

  handleNewLog(log) {
    this.logs.push(log);
    if (this.logs.length > 10000) {
      this.logs = this.logs.slice(-1000);
    }

    if (this.shouldShowLog(log)) {
      this.appendLog(log);
    }

    this.updateLogCount();
  }

  handleContainerUpdate(data) {
    const newContainers = data.containers;
    const oldIDs = new Set(this.containers.map((c) => c.ID));
    const newIDs = new Set(newContainers.map((c) => c.ID));

    const added = newContainers.filter((c) => !oldIDs.has(c.ID));
    const removed = this.containers.filter((c) => !newIDs.has(c.ID));

    if (added.length > 0) {
      added.forEach((c) => {
        this.selectedContainers.add(c.ID);
        console.log(`Container started: ${c.Name} (${c.ID})`);
      });
    }

    if (removed.length > 0) {
      removed.forEach((c) => {
        this.selectedContainers.delete(c.ID);
        console.log(`Container stopped: ${c.Name} (${c.ID})`);
      });
    }

    this.containers = newContainers;
    this.renderContainers();

    if (this.traceFilter) {
      this.analyzeTrace();
    }
  }

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
  }

  matchesSearch(log) {
    const searchable = [
      log.entry?.raw,
      log.entry?.message,
      log.entry?.level,
      ...Object.values(log.entry?.fields || {}),
    ]
      .join(" ")
      .toLowerCase();

    return searchable.includes(this.searchQuery);
  }

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
  }

  groupContainersByProject() {
    const groups = {};
    this.containers.forEach((container) => {
      const project = this.getProjectName(container.Name);
      if (!groups[project]) {
        groups[project] = [];
      }
      groups[project].push(container);
    });
    return groups;
  }

  renderContainers() {
    const list = document.getElementById("containerList");
    list.innerHTML = "";

    const groups = this.groupContainersByProject();
    const projectNames = Object.keys(groups).sort();

    projectNames.forEach((projectName) => {
      const projectContainers = groups[projectName];

      const projectSection = document.createElement("div");
      projectSection.className = "project-section";

      const projectHeader = document.createElement("div");
      projectHeader.className = "project-header";

      const allSelected = projectContainers.every((c) =>
        this.selectedContainers.has(c.ID)
      );
      const someSelected = projectContainers.some((c) =>
        this.selectedContainers.has(c.ID)
      );

      projectHeader.innerHTML = `
                <span class="disclosure-arrow">▼</span>
                <div class="checkbox ${
                  someSelected && !allSelected ? "indeterminate" : ""
                }"></div>
                <span class="project-name">${projectName}</span>
                <span class="project-count">(${projectContainers.length})</span>
            `;

      if (allSelected) {
        projectHeader.querySelector(".checkbox").classList.add("checked");
      }

      const containersList = document.createElement("div");
      containersList.className = "project-containers";

      projectContainers.forEach((container) => {
        const item = document.createElement("div");
        item.className = "container-item";
        if (this.selectedContainers.has(container.ID)) {
          item.classList.add("selected");
        }

        item.innerHTML = `
                    <div class="checkbox"></div>
                    <div class="container-info">
                        <div class="container-name">${container.Name}</div>
                        <div class="container-id">${container.ID}</div>
                    </div>
                `;

        item.addEventListener("click", (e) => {
          e.stopPropagation();
          if (this.selectedContainers.has(container.ID)) {
            this.selectedContainers.delete(container.ID);
          } else {
            this.selectedContainers.add(container.ID);
          }
          this.renderContainers();
          this.renderLogs();
        });

        containersList.appendChild(item);
      });

      projectHeader.addEventListener("click", (e) => {
        if (e.target.closest(".checkbox")) {
          const allSelected = projectContainers.every((c) =>
            this.selectedContainers.has(c.ID)
          );
          projectContainers.forEach((c) => {
            if (allSelected) {
              this.selectedContainers.delete(c.ID);
            } else {
              this.selectedContainers.add(c.ID);
            }
          });
          this.renderContainers();
          this.renderLogs();
        } else {
          containersList.classList.toggle("collapsed");
          projectHeader
            .querySelector(".disclosure-arrow")
            .classList.toggle("collapsed");
        }
      });

      projectSection.appendChild(projectHeader);
      projectSection.appendChild(containersList);
      list.appendChild(projectSection);
    });
  }

  renderLogs() {
    const logsEl = document.getElementById("logs");
    logsEl.innerHTML = "";

    const startIdx = Math.max(0, this.logs.length - 1000);

    for (let i = startIdx; i < this.logs.length; i++) {
      const log = this.logs[i];
      if (this.shouldShowLog(log)) {
        this.appendLog(log, logsEl);
      }
    }

    this.scrollToBottom();
    this.updateLogCount();
  }

  appendLog(log, container = null) {
    const logsEl = container || document.getElementById("logs");
    const line = document.createElement("div");
    line.className = "log-line";
    line.style.cursor = "pointer";
    line.title = "Click to view details";

    const parts = [];

    const containerName = this.containers.find(c => c.ID === log.containerId)?.Name || log.containerId;
    parts.push(`<span class="log-container">${containerName}</span>`);

    if (log.entry?.timestamp) {
      parts.push(`<span class="log-timestamp">${log.entry.timestamp}</span>`);
    }

    if (log.entry?.level) {
      parts.push(
        `<span class="log-level ${log.entry.level}">${log.entry.level}</span>`
      );
    }

    if (log.entry?.file) {
      parts.push(`<span class="log-file">${log.entry.file}</span>`);
    }

    if (log.entry?.message) {
      parts.push(
        `<span class="log-message">${this.escapeHtml(log.entry.message)}</span>`
      );
    }

    if (log.entry?.fields) {
      for (const [key, value] of Object.entries(log.entry.fields)) {
        const shortValue =
          value.length > 100 ? value.substring(0, 100) + "..." : value;
        const isJson =
          value.trim().startsWith("{") || value.trim().startsWith("[");
        const clickableClass = isJson ? "" : "log-field-value";
        const dataAttrs = isJson
          ? ""
          : `data-field-type="${key}" data-field-value="${this.escapeHtml(
              value
            )}"`;
        parts.push(
          `<span class="log-field"><span class="log-field-key">${key}</span>=<span class="${clickableClass}" ${dataAttrs}>${this.escapeHtml(
            shortValue
          )}</span></span>`
        );
      }
    }

    line.innerHTML = parts.join(" ");

    line.querySelectorAll(".log-field-value").forEach((el) => {
      el.addEventListener("click", (e) => {
        e.stopPropagation();
        const fieldType = el.getAttribute("data-field-type");
        const fieldValue = el.getAttribute("data-field-value");
        this.setTraceFilter(fieldType, fieldValue);
      });
    });

    line.addEventListener("click", () => {
      this.showLogDetails(log);
    });

    logsEl.appendChild(line);

    if (!container) {
      this.scrollToBottom();
    }
  }

  setTraceFilter(type, value) {
    this.traceFilter = { type, value };
    document.getElementById("filterType").textContent = type;
    document.getElementById("filterValue").textContent = value;
    document.getElementById("clearFilter").disabled = false;
    this.renderLogs();
    this.analyzeTrace();
  }

  clearTraceFilter() {
    this.traceFilter = null;
    document.getElementById("filterType").textContent = "No filter";
    document.getElementById("filterValue").textContent = "";
    document.getElementById("clearFilter").disabled = true;
    this.renderLogs();
    this.closeAnalyzer();
  }

  analyzeTrace() {
    if (!this.traceFilter) {
      this.closeAnalyzer();
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
    this.showAnalyzer();
  }

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

          queries.push({
            query,
            duration,
            table,
            operation,
            rows,
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
        (q) => `
            <div class="query-item">
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
            </div>
        `
      )
      .join("");

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
            </div>
        `
      )
      .join("");

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
