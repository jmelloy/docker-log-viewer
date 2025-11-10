/**
 * Shared Log Stream Component
 *
 * A reusable Vue component for displaying streaming logs from Docker containers.
 * Can be used in both the main log viewer and GraphQL explorer.
 *
 * Props:
 * - requestIdFilter: Filter logs by specific request_id (optional)
 * - containerFilter: Filter logs by container names (optional, array)
 * - levelFilter: Filter logs by log levels (optional, array)
 * - maxLogs: Maximum number of logs to keep in memory (default: 1000)
 * - autoScroll: Auto-scroll to bottom when new logs arrive (default: true)
 * - compact: Use compact display mode (default: false)
 *
 * Events:
 * - @log-clicked: Emitted when a log line is clicked with the log entry
 */

import { API } from "./api.js";
import { loadTemplate } from "./template-loader.js";

const template = await loadTemplate("/static/js/shared/log-stream-template.html");

export function createLogStreamComponent() {
  const { createApp } = Vue;

  return {
    name: "LogStreamComponent",

    props: {
      requestIdFilter: {
        type: String,
        default: null,
      },
      containerFilter: {
        type: Array,
        default: () => [],
      },
      levelFilter: {
        type: Array,
        default: () => ["DBG", "DEBUG", "TRC", "TRACE", "INF", "INFO", "WRN", "WARN", "ERR", "ERROR", "FATAL", "NONE"],
      },
      maxLogs: {
        type: Number,
        default: 1000,
      },
      autoScroll: {
        type: Boolean,
        default: true,
      },
      compact: {
        type: Boolean,
        default: false,
      },
      showContainer: {
        type: Boolean,
        default: true,
      },
    },

    data() {
      return {
        logs: [],
        ws: null,
        wsConnected: false,
        containers: [],
        containerIDNames: {},
      };
    },

    computed: {
      filteredLogs() {
        return this.logs.filter((log) => {
          // Filter by request_id if specified
          if (this.requestIdFilter) {
            const logRequestId = log.entry?.fields?.request_id;
            if (logRequestId !== this.requestIdFilter) {
              return false;
            }
          }

          // Filter by container if specified
          if (this.containerFilter.length > 0) {
            const containerName = this.getContainerName(log.containerId);
            if (!this.containerFilter.includes(containerName)) {
              return false;
            }
          }

          // Filter by log level if specified
          if (this.levelFilter.length > 0) {
            const level = log.entry?.level || "NONE";
            if (!this.levelFilter.includes(level)) {
              return false;
            }
          }

          return true;
        });
      },

      statusColor() {
        return this.wsConnected ? "#7ee787" : "#f85149";
      },

      statusText() {
        return this.wsConnected ? "Connected" : "Connecting...";
      },

      logCountText() {
        return `${this.filteredLogs.length} logs`;
      },
    },

    watch: {
      requestIdFilter(newVal, oldVal) {
        if (newVal !== oldVal) {
          this.sendFilterUpdate();
        }
      },
      containerFilter: {
        handler() {
          this.sendFilterUpdate();
        },
        deep: true,
      },
      levelFilter: {
        handler() {
          this.sendFilterUpdate();
        },
        deep: true,
      },
    },

    mounted() {
      this.connectWebSocket();
      this.loadContainers();
    },

    beforeUnmount() {
      if (this.ws) {
        this.ws.close();
      }
    },

    methods: {
      async loadContainers() {
        try {
          const data = await API.get("/api/containers");

          if (Array.isArray(data)) {
            this.containers = data;
          } else {
            this.containers = data.containers || [];
          }

          // Build container ID to name mapping
          this.containerIDNames = {};
          this.containers.forEach((container) => {
            this.containerIDNames[container.ID] = container.Name;
          });
        } catch (error) {
          console.error("Failed to load containers:", error);
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

      sendFilterUpdate() {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
          console.log("Cannot send filter update - WebSocket not connected");
          return;
        }

        const traceFilters = [];
        if (this.requestIdFilter) {
          traceFilters.push({
            type: "request_id",
            value: this.requestIdFilter,
          });
        }

        const filter = {
          selectedContainers:
            this.containerFilter.length > 0 ? this.containerFilter : this.containers.map((c) => c.Name),
          selectedLevels: this.levelFilter,
          searchQuery: "",
          traceFilters: traceFilters,
        };

        console.log("Sending filter update:", filter);

        this.ws.send(
          JSON.stringify({
            type: "filter",
            data: filter,
          })
        );
      },

      handleNewLog(log) {
        this.logs.push(log);
        if (this.logs.length > this.maxLogs) {
          this.logs = this.logs.slice(-Math.floor(this.maxLogs / 2));
        }
        if (this.autoScroll) {
          this.$nextTick(() => this.scrollToBottom());
        }
      },

      handleNewLogs(logs) {
        this.logs.push(...logs);
        if (this.logs.length > this.maxLogs) {
          this.logs = this.logs.slice(-Math.floor(this.maxLogs / 2));
        }
        if (this.autoScroll) {
          this.$nextTick(() => this.scrollToBottom());
        }
      },

      handleInitialLogs(logs) {
        console.log(`Received ${logs.length} initial filtered logs`);
        this.logs = logs;
        if (this.autoScroll) {
          this.$nextTick(() => this.scrollToBottom());
        }
      },

      handleContainerUpdate(data) {
        const newContainers = data.containers;
        this.containers = newContainers;

        // Update container ID to name mapping
        this.containerIDNames = {};
        newContainers.forEach((container) => {
          this.containerIDNames[container.ID] = container.Name;
        });
      },

      getContainerName(containerId) {
        return this.containerIDNames[containerId] || containerId;
      },

      scrollToBottom() {
        const logsEl = this.$refs.logsContainer;
        if (logsEl) {
          logsEl.scrollTop = logsEl.scrollHeight;
        }
      },

      onLogClick(log) {
        this.$emit("log-clicked", log);
      },

      shouldShowField(key, value) {
        // Always show error field
        if (key === 'error') return true;
        // Show fields less than 40 characters
        const s = String(value);
        return s.length < 40;
      },

      formatFieldValue(value) {
        if (typeof value !== "string") {
          return String(value);
        }
        const shortValue = value.length > 100 ? value.substring(0, 100) + "..." : value;
        return shortValue;
      },

      isJsonField(value) {
        if (typeof value !== "string") return false;
        const trimmed = value.trim();
        return trimmed.startsWith("{") || trimmed.startsWith("[");
      },

      clearLogs() {
        this.logs = [];
      },
    },

    template,
  };
}
