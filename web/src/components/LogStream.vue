<template>
  <div class="log-stream-component" :class="{ 'compact-mode': compact }">
    <div
      v-if="!compact"
      class="log-stream-header"
      style="
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0.5rem;
        background: #161b22;
        border-bottom: 1px solid #30363d;
      "
    >
      <div style="display: flex; align-items: center; gap: 0.5rem">
        <span style="font-weight: 500">Logs</span>
        <span :style="{ color: statusColor, fontSize: '0.75rem' }">{{ statusText }}</span>
      </div>
      <div style="display: flex; align-items: center; gap: 0.5rem">
        <span style="color: #8b949e; font-size: 0.75rem">{{ logCountText }}</span>
        <button class="btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem" @click="clearLogs">
          Clear
        </button>
      </div>
    </div>
    <div ref="logsContainer" class="logs" :style="compact ? 'max-height: 300px;' : 'max-height: 500px;'">
      <div v-if="filteredLogs.length === 0" style="padding: 2rem; text-align: center; color: #8b949e">
        <div style="font-size: 1.5rem; margin-bottom: 0.5rem">📝</div>
        <div>{{ wsConnected ? "No logs yet" : "Connecting to log stream..." }}</div>
      </div>
      <div v-for="(log, index) in filteredLogs" :key="index" class="log-line" @click="onLogClick(log)">
        <span v-if="showContainer && !compact" class="log-container">{{ getShortContainerName(log.containerId) }}</span>
        <span v-if="log.entry?.timestamp" class="log-timestamp">{{ formatTimestamp(log.entry.timestamp) }}</span>
        <span v-if="log.entry?.level" class="log-level" :class="log.entry.level">{{ log.entry.level }}</span>
        <span v-if="log.entry?.message" class="log-message">{{ log.entry.message }}</span>
        <span v-for="([key, value], idx) in Object.entries(log.entry?.fields || {})" :key="idx" class="log-field">
          <template v-if="!compact || shouldShowField(key, value)">
            <span class="log-field-key">{{ key }}</span
            >=<span class="log-field-value">{{ formatFieldValue(value) }}</span>
          </template>
        </span>
        <span v-if="log.entry?.file" class="log-file">{{ log.entry.file }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from "vue";
import type { Container, LogMessage, ContainerData } from "@/types";

interface Props {
  requestIdFilter?: string | null;
  containerFilter?: string[];
  levelFilter?: string[];
  maxLogs?: number;
  autoScroll?: boolean;
  compact?: boolean;
  showContainer?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  requestIdFilter: null,
  containerFilter: () => [],
  levelFilter: () => ["DBG", "DEBUG", "TRC", "TRACE", "INF", "INFO", "WRN", "WARN", "ERR", "ERROR", "FATAL", "NONE"],
  maxLogs: 1000,
  autoScroll: true,
  compact: false,
  showContainer: true,
});

const emit = defineEmits<{
  "log-clicked": [log: LogMessage];
}>();

const logs = ref<LogMessage[]>([]);
const ws = ref<WebSocket | null>(null);
const wsConnected = ref(false);
const containers = ref<Container[]>([]);
const containerIDNames = ref<Record<string, string>>({});
const logsContainer = ref<HTMLElement>();

const filteredLogs = computed(() => {
  return logs.value.filter((log) => {
    if (props.requestIdFilter) {
      const logRequestId = log.entry?.fields?.request_id;
      if (logRequestId !== props.requestIdFilter) {
        return false;
      }
    }

    if (props.containerFilter.length > 0) {
      const containerName = getContainerName(log.containerId);
      if (!props.containerFilter.includes(containerName)) {
        return false;
      }
    }

    if (props.levelFilter.length > 0) {
      const level = log.entry?.level || "NONE";
      if (!props.levelFilter.includes(level)) {
        return false;
      }
    }

    return true;
  });
});

const statusColor = computed(() => (wsConnected.value ? "#7ee787" : "#f85149"));
const statusText = computed(() => (wsConnected.value ? "Connected" : "Connecting..."));
const logCountText = computed(() => `${filteredLogs.value.length} logs`);

watch(
  () => props.requestIdFilter,
  (newVal, oldVal) => {
    if (newVal !== oldVal) {
      sendFilterUpdate();
    }
  }
);

watch(
  () => props.containerFilter,
  () => {
    sendFilterUpdate();
  },
  { deep: true }
);

watch(
  () => props.levelFilter,
  () => {
    sendFilterUpdate();
  },
  { deep: true }
);

async function loadContainers() {
  try {
    const response: Response = await fetch("/api/containers");
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    const data: ContainerData | Container[] = await response.json();

    if (Array.isArray(data)) {
      containers.value = data;
    } else {
      containers.value = data.containers || [];
    }

    containerIDNames.value = {};
    containers.value.forEach((container) => {
      containerIDNames.value[container.ID] = container.Name;
    });
  } catch (error) {
    console.error("Failed to load containers:", error);
  }
}

function connectWebSocket() {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  const wsUrl = `${protocol}//${window.location.host}/api/ws`;

  ws.value = new WebSocket(wsUrl);

  ws.value.onopen = () => {
    wsConnected.value = true;
    sendFilterUpdate();
  };

  ws.value.onmessage = (event) => {
    const message = JSON.parse(event.data);
    if (message.type === "log") {
      handleNewLog(message.data);
    } else if (message.type === "logs") {
      handleNewLogs(message.data);
    } else if (message.type === "logs_initial") {
      handleInitialLogs(message.data);
    } else if (message.type === "containers") {
      handleContainerUpdate(message.data);
    }
  };

  ws.value.onclose = () => {
    wsConnected.value = false;
    setTimeout(() => connectWebSocket(), 5000);
  };

  ws.value.onerror = (error) => {
    console.error("WebSocket error:", error);
  };
}

function sendFilterUpdate() {
  if (!ws.value || ws.value.readyState !== WebSocket.OPEN) {
    console.log("Cannot send filter update - WebSocket not connected");
    return;
  }

  const traceFilters = [];
  if (props.requestIdFilter) {
    traceFilters.push({
      type: "request_id",
      value: props.requestIdFilter,
    });
  }

  const filter = {
    selectedContainers: props.containerFilter.length > 0 ? props.containerFilter : containers.value.map((c) => c.Name),
    selectedLevels: props.levelFilter,
    searchQuery: "",
    traceFilters: traceFilters,
  };

  console.log("Sending filter update:", filter);

  ws.value.send(
    JSON.stringify({
      type: "filter",
      data: filter,
    })
  );
}

function handleNewLog(log: LogMessage) {
  logs.value.push(log);
  if (logs.value.length > props.maxLogs) {
    logs.value = logs.value.slice(-Math.floor(props.maxLogs / 2));
  }
  if (props.autoScroll) {
    nextTick(() => scrollToBottom());
  }
}

function handleNewLogs(newLogs: LogMessage[]) {
  logs.value.push(...newLogs);
  if (logs.value.length > props.maxLogs) {
    logs.value = logs.value.slice(-Math.floor(props.maxLogs / 2));
  }
  if (props.autoScroll) {
    nextTick(() => scrollToBottom());
  }
}

function handleInitialLogs(initialLogs: LogMessage[]) {
  console.log(`Received ${initialLogs.length} initial filtered logs`);
  logs.value = initialLogs;
  if (props.autoScroll) {
    nextTick(() => scrollToBottom());
  }
}

function handleContainerUpdate(data: { containers: Container[] }) {
  containers.value = data.containers;

  containerIDNames.value = {};
  data.containers.forEach((container) => {
    containerIDNames.value[container.ID] = container.Name;
  });
}

function getContainerName(containerId: string): string {
  return containerIDNames.value[containerId] || containerId;
}

function getShortContainerName(containerId: string): string {
  const fullName = getContainerName(containerId);
  const parts = fullName.split("-");
  if (parts.length >= 3 && parts[parts.length - 1].match(/^\d+$/)) {
    return parts[parts.length - 2];
  }
  return fullName;
}

function formatTimestamp(timestamp: string): string {
  if (!timestamp) return "";

  if (timestamp.match(/^\d{2}:\d{2}:\d{2}/)) {
    return timestamp.substring(0, 8);
  }

  try {
    const timeMatch = timestamp.match(/(\d{2}):(\d{2}):(\d{2})/);
    if (timeMatch) {
      return `${timeMatch[1]}:${timeMatch[2]}:${timeMatch[3]}`;
    }
  } catch {
    // If parsing fails, return original
  }

  return timestamp;
}

function scrollToBottom() {
  if (logsContainer.value) {
    logsContainer.value.scrollTop = logsContainer.value.scrollHeight;
  }
}

function onLogClick(log: LogMessage) {
  emit("log-clicked", log);
}

function shouldShowField(key: string, value: any): boolean {
  if (key === "error") return true;
  const s = String(value);
  return s.length < 50;
}

function formatFieldValue(value: any): string {
  if (typeof value !== "string") {
    return String(value);
  }
  const shortValue = value.length > 100 ? value.substring(0, 100) + "..." : value;
  return shortValue;
}

function clearLogs() {
  logs.value = [];
}

onMounted(() => {
  connectWebSocket();
  loadContainers();
});

onBeforeUnmount(() => {
  if (ws.value) {
    ws.value.close();
  }
});
</script>
