# Log Stream Component Integration Guide

## Overview

The Log Stream Component is a reusable Vue 3 component that provides real-time Docker log streaming with filtering capabilities. It was created by extracting and refactoring the log streaming logic from the main log viewer (`web/js/app.js`) into a standalone, reusable component.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     GraphQL Explorer                        │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  executeQuery()                                       │  │
│  │  • Shows log panel                                    │  │
│  │  • Clears previous logs                               │  │
│  │  • Sends request to backend                           │  │
│  └───────────────────────────────────────────────────────┘  │
│                           │                                 │
│                           ▼                                 │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  pollForResult()                                      │  │
│  │  • Polls execution endpoint                           │  │
│  │  • Extracts requestIdHeader                           │  │
│  │  • Sets filter for log component                      │  │
│  └───────────────────────────────────────────────────────┘  │
│                           │                                 │
│                           ▼                                 │
│  ┌───────────────────────────────────────────────────────┐  │
│  │         Log Stream Component                          │  │
│  │  Props:                                               │  │
│  │    requestIdFilter = "abc-123-xyz"                    │  │
│  │    compact = true                                     │  │
│  │    maxLogs = 500                                      │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
              ┌────────────────────────┐
              │   WebSocket /api/ws    │
              │  • Sends filter        │
              │  • Receives logs       │
              └────────────────────────┘
                           │
                           ▼
              ┌────────────────────────┐
              │   Backend (Go)         │
              │  • Filters by          │
              │    request_id          │
              │  • Streams logs        │
              └────────────────────────┘
```

## Component API

### Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `requestIdFilter` | String | `null` | Filter logs by specific request_id |
| `containerFilter` | Array | `[]` | Array of container names to filter |
| `levelFilter` | Array | All levels | Array of log levels to display |
| `maxLogs` | Number | `1000` | Maximum logs to keep in memory |
| `autoScroll` | Boolean | `true` | Auto-scroll to bottom on new logs |
| `compact` | Boolean | `false` | Use compact display mode |
| `showContainer` | Boolean | `true` | Show container name in logs |

### Events

| Event | Payload | Description |
|-------|---------|-------------|
| `log-clicked` | `log` | Emitted when a log line is clicked |

### Methods (via ref)

| Method | Description |
|--------|-------------|
| `clearLogs()` | Clear all logs from the display |

## Usage Examples

### 1. Basic Usage (GraphQL Explorer)

```vue
<template>
  <log-stream 
    ref="logStream"
    :request-id-filter="requestIdHeader"
    :compact="true"
    @log-clicked="handleLogClick"
  />
</template>

<script>
import { createLogStreamComponent } from './shared/log-stream-component.js';

export default {
  components: {
    'log-stream': createLogStreamComponent()
  },
  data() {
    return {
      requestIdHeader: null
    };
  },
  methods: {
    async executeQuery() {
      // Clear previous logs
      this.$refs.logStream.clearLogs();
      
      // Execute request
      const response = await API.post('/api/execute', payload);
      
      // Set filter when we get request ID
      this.requestIdHeader = response.requestIdHeader;
    },
    handleLogClick(log) {
      console.log('Log clicked:', log);
    }
  }
};
</script>
```

### 2. Full-Featured Usage

```vue
<template>
  <log-stream 
    :request-id-filter="currentRequestId"
    :container-filter="selectedContainers"
    :level-filter="['ERR', 'WARN', 'INFO']"
    :max-logs="2000"
    :auto-scroll="true"
    :compact="false"
    :show-container="true"
    @log-clicked="openLogDetails"
  />
</template>

<script>
export default {
  data() {
    return {
      currentRequestId: 'abc-123-xyz',
      selectedContainers: ['api-server', 'database']
    };
  },
  methods: {
    openLogDetails(log) {
      // Show modal with full log details
      this.selectedLog = log;
      this.showModal = true;
    }
  }
};
</script>
```

### 3. Minimal Usage (Container-Only Filtering)

```vue
<template>
  <log-stream 
    :container-filter="['nginx']"
    :compact="true"
  />
</template>
```

## WebSocket Protocol

### Client → Server (Filter Update)

The component sends filter updates to the backend:

```javascript
{
  type: "filter",
  data: {
    selectedContainers: ["api-server", "worker"],
    selectedLevels: ["ERR", "WARN", "INFO"],
    searchQuery: "",
    traceFilters: [
      { type: "request_id", value: "abc-123-xyz" }
    ]
  }
}
```

### Server → Client (Log Messages)

The backend sends log data in three message types:

#### Single Log
```javascript
{
  type: "log",
  data: {
    containerId: "abc123",
    timestamp: "2024-01-01T12:00:00Z",
    entry: {
      timestamp: "12:00:00.123",
      level: "INFO",
      message: "Request completed",
      fields: {
        request_id: "abc-123-xyz",
        duration: "123ms"
      }
    }
  }
}
```

#### Batch Logs
```javascript
{
  type: "logs",
  data: [ /* array of log objects */ ]
}
```

#### Initial Filtered Logs
```javascript
{
  type: "logs_initial",
  data: [ /* array of initial filtered logs */ ]
}
```

## Styling

The component uses these CSS classes:

- `.log-stream-component` - Main container
- `.log-stream-component.compact-mode` - Compact mode variant
- `.log-stream-header` - Header with status
- `.logs` - Log display area
- `.log-line` - Individual log line
- `.log-container` - Container name
- `.log-timestamp` - Timestamp
- `.log-level` - Log level badge
- `.log-file` - Source file
- `.log-message` - Log message
- `.log-field` - Key-value field
- `.log-field-key` - Field name
- `.log-field-value` - Field value

## State Management

The component manages its own state:

```javascript
data() {
  return {
    logs: [],              // Array of log entries
    ws: null,              // WebSocket connection
    wsConnected: false,    // Connection status
    containers: [],        // Available containers
    containerIDNames: {}   // Container ID → Name mapping
  }
}
```

## Performance Considerations

1. **Memory Management**: Keeps only `maxLogs` in memory (default: 1000)
2. **Efficient Updates**: Uses Vue's reactivity for optimal rendering
3. **Auto-cleanup**: Automatically closes WebSocket on unmount
4. **Smart Filtering**: Server-side filtering reduces client load

## Integration Checklist

When integrating the component into a new page:

- [ ] Import the component: `import { createLogStreamComponent } from './shared/log-stream-component.js'`
- [ ] Register the component: `app.component('log-stream', createLogStreamComponent())`
- [ ] Add to template with appropriate props
- [ ] Set up state for filter props (e.g., `requestIdFilter`)
- [ ] Handle `@log-clicked` event if needed
- [ ] Use `ref` to access `clearLogs()` method if needed
- [ ] Test WebSocket connection and filtering
- [ ] Verify auto-scroll behavior
- [ ] Check responsive layout

## Troubleshooting

### Component not receiving logs

1. Check WebSocket connection: Look for "Connected" status
2. Verify filter props are set correctly
3. Check browser console for WebSocket errors
4. Ensure backend is running and accessible

### Logs not auto-scrolling

1. Verify `autoScroll` prop is `true`
2. Check if container has fixed height
3. Ensure `ref` is correctly set on component

### Memory issues

1. Reduce `maxLogs` prop value
2. Clear logs periodically with `clearLogs()`
3. Use more specific filters to reduce log volume

## Future Enhancements

Planned improvements:
- [ ] Log export/download functionality
- [ ] Advanced search within logs
- [ ] Timestamp range filtering
- [ ] Syntax highlighting for JSON/SQL
- [ ] Log detail modal
- [ ] Performance metrics (logs/sec)
