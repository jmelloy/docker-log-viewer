<template>
  <div class="app-container">
    <app-header activePage="requests"></app-header>

    <div class="main-layout">
      <main class="content content-padded">
        <div v-if="loading" class="text-center p-3">
          <p>Loading request details...</p>
        </div>

        <div v-if="error" class="text-center p-3">
          <div class="alert alert-danger">{{ error }}</div>
          <button @click="goBack" class="btn-secondary">Go Back</button>
        </div>

        <div v-if="!loading && !error && requestDetail">
          <div class="flex-between mb-1_5">
            <div>
              <button @click="goBack" class="btn-secondary mb-0_5">← Back to Requests</button>
              <h2 class="m-0">{{ requestDetail.displayName || "(unnamed)" }}</h2>
              <p class="text-muted mt-0_25">
                {{ requestDetail.server?.name || requestDetail.execution.server?.name || "N/A" }}
              </p>
            </div>
            <div style="display: flex; gap: 0.5rem">
              <button @click="executeAgain" class="btn-primary">▶ Execute Again</button>
              <button @click="openExecuteModal" class="btn-secondary">⚙️ Re-execute with Options</button>
            </div>
          </div>

          <div class="execution-overview">
            <div class="stat-item">
              <span class="stat-label">Status Code</span>
              <span class="stat-value" :class="statusClass">{{
                requestDetail.execution.statusCode || "Executing"
              }}</span>
            </div>
            <div class="stat-item">
              <span class="stat-label">Duration</span>
              <span class="stat-value">{{ requestDetail.execution.durationMs }}ms</span>
            </div>
            <div class="stat-item">
              <span class="stat-label">Request ID</span>
              <span class="stat-value">{{ requestDetail.execution.requestIdHeader }}</span>
            </div>
            <div class="stat-item">
              <span class="stat-label">Executed At</span>
              <span class="stat-value">{{ new Date(requestDetail.execution.executedAt).toLocaleString() }}</span>
            </div>
            <div class="stat-item" v-if="requestDetail.server?.devId || requestDetail.execution.server?.devId">
              <span class="stat-label">Dev ID</span>
              <span class="stat-value">{{
                requestDetail.server?.devId || requestDetail.execution.server?.devId || "N/A"
              }}</span>
            </div>
          </div>

          <div class="modal-section">
            <div style="display: flex; gap: 1rem; flex-wrap: wrap">
              <div style="flex: 1; min-width: 300px">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem">
                  <h4 style="margin: 0">
                    Request Body<span
                      v-if="isGraphQLRequest"
                      style="color: #8b949e; font-size: 0.75rem; margin-left: 0.5rem"
                      >GraphQL</span
                    >
                  </h4>
                  <div style="display: flex; gap: 0.5rem">
                    <button
                      @click="copyToClipboard(requestData)"
                      class="btn-secondary"
                      style="padding: 0.25rem 0.5rem; font-size: 0.75rem"
                      title="Copy request"
                    >
                      📋 Copy
                    </button>
                    <button
                      @click="viewBigger('request')"
                      class="btn-secondary"
                      style="padding: 0.25rem 0.5rem; font-size: 0.75rem"
                      title="View bigger"
                    >
                      🔍 View
                    </button>
                  </div>
                </div>

                <!-- GraphQL Request Display -->
                <div v-if="isGraphQLRequest">
                  <div v-if="isGraphQLBatchRequest" style="margin-bottom: 0.5rem">
                    <div style="color: #8b949e; font-size: 0.75rem; margin-bottom: 0.25rem">
                      Operations ({{ graphqlOperations.length }}):
                    </div>
                    <select
                      v-model="selectedOperationIndex"
                      style="
                        width: 100%;
                        background: #161b22;
                        border: 1px solid #30363d;
                        border-radius: 4px;
                        padding: 0.5rem;
                        font-family: monospace;
                        font-size: 0.875rem;
                        color: #79c0ff;
                        cursor: pointer;
                      "
                    >
                      <option v-for="(op, idx) in graphqlOperations" :key="idx" :value="idx">
                        {{ op.operationName }}
                      </option>
                    </select>
                  </div>
                  <div v-else-if="graphqlOperationName" style="margin-bottom: 0.5rem">
                    <div style="color: #8b949e; font-size: 0.75rem; margin-bottom: 0.25rem">Operation:</div>
                    <div
                      style="
                        background: #161b22;
                        border: 1px solid #30363d;
                        border-radius: 4px;
                        padding: 0.5rem;
                        font-family: monospace;
                        font-size: 0.875rem;
                        color: #79c0ff;
                      "
                    >
                      {{ graphqlOperationName }}
                    </div>
                  </div>
                  <div style="margin-bottom: 0.5rem">
                    <div style="color: #8b949e; font-size: 0.75rem; margin-bottom: 0.25rem">Query:</div>
                    <pre class="graphql-query" style="max-height: 12em; white-space: pre-wrap">{{ graphqlQuery }}</pre>
                  </div>
                  <div v-if="graphqlVariables" style="margin-bottom: 0.5rem">
                    <div style="color: #8b949e; font-size: 0.75rem; margin-bottom: 0.25rem">Variables:</div>
                    <pre class="json-display" style="max-height: 8em">{{ graphqlVariables }}</pre>
                  </div>
                </div>

                <!-- Standard Request Display -->
                <pre v-else class="json-display" style="max-height: 20em">{{ requestData }}</pre>
              </div>
              <div style="flex: 1; min-width: 300px">
                <div
                  style="
                    display: flex;
                    justify-content: space-between;
                    align-items: center;
                    margin-bottom: 0.5rem;
                    gap: 0.5rem;
                  "
                >
                  <h4 style="margin: 0">Response</h4>
                  <input
                    type="text"
                    v-model="responseFilter"
                    placeholder="Filter (.data.users[0] or text)"
                    style="
                      flex: 1;
                      max-width: 250px;
                      padding: 0.5rem;
                      background: #161b22;
                      border: 1px solid #30363d;
                      border-radius: 4px;
                      color: #c9d1d9;
                      font-family: monospace;
                      font-size: 0.75rem;
                    "
                    title="Filter response JSON. Examples: .data, .data.users[0], .errors[1].message, or simple text search"
                  />

                  <div style="display: flex; gap: 0.5rem">
                    <button
                      @click="copyToClipboard(filteredResponseBody)"
                      class="btn-secondary"
                      style="padding: 0.25rem 0.5rem; font-size: 0.75rem"
                      title="Copy response"
                    >
                      📋 Copy
                    </button>
                    <button
                      @click="viewBigger('response')"
                      class="btn-secondary"
                      style="padding: 0.25rem 0.5rem; font-size: 0.75rem"
                      title="View bigger"
                    >
                      🔍 View
                    </button>
                  </div>
                </div>
                <pre class="json-display" style="max-height: 28em">{{ filteredResponseBody }}</pre>
              </div>
            </div>
          </div>

          <div v-if="requestDetail.execution.error" class="modal-section">
            <h4>Error</h4>
            <pre class="error-display">{{ requestDetail.execution.error }}</pre>
          </div>

          <div v-if="sqlAnalysisData" class="modal-section">
            <h4>SQL Query Analyzer</h4>

            <div class="analyzer-subsection" style="margin-bottom: 1.5rem">
              <h5
                style="
                  color: #8b949e;
                  font-size: 0.9rem;
                  margin-bottom: 0.5rem;
                  text-transform: uppercase;
                  letter-spacing: 0.05em;
                "
              >
                Overview
              </h5>
              <div class="stats-grid-compact">
                <div class="stat-item">
                  <span class="stat-label">Total Queries</span>
                  <span class="stat-value">{{ sqlAnalysisData.totalQueries }}</span>
                </div>
                <div class="stat-item">
                  <span class="stat-label">Unique</span>
                  <span class="stat-value">{{ sqlAnalysisData.uniqueQueries }}</span>
                </div>
                <div class="stat-item">
                  <span class="stat-label">Avg Duration</span>
                  <span class="stat-value">{{ sqlAnalysisData.avgDuration.toFixed(2) }}ms</span>
                </div>
                <div class="stat-item">
                  <span class="stat-label">Total Duration</span>
                  <span class="stat-value">{{ sqlAnalysisData.totalDuration.toFixed(2) }}ms</span>
                </div>
              </div>
            </div>

            <div style="display: flex; gap: 1rem; flex-wrap: wrap; margin-bottom: 1.5rem">
              <div
                v-if="sqlAnalysisData.slowestQueries.length > 0"
                class="analyzer-subsection"
                style="flex: 1; min-width: 280px"
              >
                <h5
                  style="
                    color: #8b949e;
                    font-size: 0.9rem;
                    margin-bottom: 0.5rem;
                    text-transform: uppercase;
                    letter-spacing: 0.05em;
                  "
                >
                  Slowest Queries
                </h5>
                <div class="query-list-compact">
                  <div
                    v-for="(q, index) in sqlAnalysisData.slowestQueries.slice(0, 5)"
                    :key="index"
                    class="query-item-compact"
                  >
                    <div class="query-header-compact">
                      <span class="query-duration" :class="{ 'query-slow': q.duration > 10 }"
                        >{{ q.duration.toFixed(2) }}ms</span
                      >
                      <span class="query-meta-inline">{{ q.table }} · {{ q.operation }}</span>
                    </div>
                    <div class="query-text-compact">
                      {{ q.query.substring(0, 100) }}{{ q.query.length > 100 ? "..." : "" }}
                    </div>
                    <button
                      class="btn-explain-compact"
                      @click="
                        handleExplainClick(
                          Math.max(
                            0,
                            requestDetail.sqlQueries.findIndex((sq) => sq.query === q.query)
                          )
                        )
                      "
                    >
                      EXPLAIN
                    </button>
                  </div>
                </div>
              </div>

              <div
                v-if="sqlAnalysisData.frequentQueries.length > 0"
                class="analyzer-subsection"
                style="flex: 1; min-width: 280px"
              >
                <h5
                  style="
                    color: #8b949e;
                    font-size: 0.9rem;
                    margin-bottom: 0.5rem;
                    text-transform: uppercase;
                    letter-spacing: 0.05em;
                  "
                >
                  Most Frequent
                </h5>
                <div class="query-list-compact">
                  <div
                    v-for="(item, index) in sqlAnalysisData.frequentQueries.slice(0, 5)"
                    :key="index"
                    class="query-item-compact"
                  >
                    <div class="query-header-compact">
                      <span class="query-count">{{ item.count }}x</span>
                      <span class="query-meta-inline"
                        >{{ item.example.table }} · {{ item.avgDuration.toFixed(2) }}ms</span
                      >
                    </div>
                    <div class="query-text-compact">
                      {{ item.example.query.substring(0, 100) }}{{ item.example.query.length > 100 ? "..." : "" }}
                    </div>
                    <button
                      class="btn-explain-compact"
                      @click="
                        handleExplainClick(
                          Math.max(
                            0,
                            requestDetail.sqlQueries.findIndex((sq) => sq.query === item.example.query)
                          )
                        )
                      "
                    >
                      EXPLAIN
                    </button>
                  </div>
                </div>
              </div>

              <div
                v-if="
                  requestDetail.indexAnalysis &&
                  requestDetail.indexAnalysis.recommendations &&
                  requestDetail.indexAnalysis.recommendations.length > 0
                "
                class="analyzer-subsection"
                style="flex: 1; min-width: 280px"
              >
                <h5
                  style="
                    color: #8b949e;
                    font-size: 0.9rem;
                    margin-bottom: 0.5rem;
                    text-transform: uppercase;
                    letter-spacing: 0.05em;
                  "
                >
                  Index Recommendations
                </h5>
                <div class="query-list-compact">
                  <div
                    v-for="(rec, index) in requestDetail.indexAnalysis.recommendations.slice(0, 5)"
                    :key="index"
                    class="query-item-compact"
                  >
                    <div class="query-header-compact">
                      <span
                        class="index-priority-badge"
                        :class="'priority-' + rec.priority"
                        style="font-size: 0.65rem; padding: 0.15rem 0.3rem"
                        >{{ rec.priority.toUpperCase() }}</span
                      >
                      <span class="query-meta-inline">{{ rec.tableName }}</span>
                    </div>
                    <div style="font-size: 0.7rem; color: #c9d1d9; margin-bottom: 0.25rem">{{ rec.reason }}</div>
                    <div style="font-size: 0.65rem; color: #8b949e; margin-bottom: 0.25rem">
                      {{ rec.columns.join(", ") }}
                    </div>
                    <div style="font-size: 0.65rem; color: #7ee787">{{ rec.estimatedImpact }}</div>
                  </div>
                </div>
              </div>
            </div>

            <div v-if="sqlAnalysisData.tables.length > 0" class="analyzer-subsection" style="margin-bottom: 1.5rem">
              <h5
                style="
                  color: #8b949e;
                  font-size: 0.9rem;
                  margin-bottom: 0.5rem;
                  text-transform: uppercase;
                  letter-spacing: 0.05em;
                "
              >
                Tables
              </h5>
              <div class="table-list-compact">
                <span v-for="(item, index) in sqlAnalysisData.tables" :key="index" class="table-badge-compact">
                  {{ item.table }}<span class="table-count">({{ item.count }})</span>
                </span>
              </div>
            </div>

            <div class="analyzer-subsection">
              <h5
                style="
                  color: #8b949e;
                  font-size: 0.9rem;
                  margin-bottom: 0.5rem;
                  text-transform: uppercase;
                  letter-spacing: 0.05em;
                "
              >
                All Queries
              </h5>
              <div class="sql-queries-list">
                <div v-for="(q, idx) in requestDetail.sqlQueries" :key="idx" class="sql-query-item">
                  <div class="sql-query-header">
                    <span>{{ q.tableName || "unknown" }} - {{ q.operation || "SELECT" }}</span>
                    <span class="sql-query-duration">{{ q.durationMs.toFixed(2) }}ms</span>
                  </div>
                  <div class="sql-query-text">{{ formatSQL(q.query) }}</div>
                  <div class="query-actions">
                    <button
                      v-if="!q.explainPlan || q.explainPlan.length === 0"
                      class="btn-explain"
                      @click="handleExplainClick(idx)"
                    >
                      Run EXPLAIN
                    </button>
                    <button
                      v-if="q.explainPlan && q.explainPlan.length > 0"
                      class="btn-explain btn-secondary"
                      @click="handleExplainClick(idx)"
                    >
                      View Saved Plan
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div
            v-if="
              requestDetail.indexAnalysis &&
              (requestDetail.indexAnalysis.sequentialScans?.length > 0 ||
                requestDetail.indexAnalysis.recommendations?.length > 0)
            "
            class="modal-section"
          >
            <h4>Index Analysis</h4>
            <div class="stats-grid">
              <div class="stat-item">
                <span class="stat-label">Sequential Scans</span>
                <span
                  class="stat-value"
                  :class="{ 'text-warning': requestDetail.indexAnalysis.summary.sequentialScans > 0 }"
                  >{{ requestDetail.indexAnalysis.summary.sequentialScans }}</span
                >
              </div>
              <div class="stat-item">
                <span class="stat-label">Index Scans</span>
                <span class="stat-value">{{ requestDetail.indexAnalysis.summary.indexScans }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">Recommendations</span>
                <span
                  class="stat-value"
                  :class="{ 'text-warning': requestDetail.indexAnalysis.summary.highPriorityRecs > 0 }"
                  >{{ requestDetail.indexAnalysis.summary.totalRecommendations }}</span
                >
              </div>
              <div class="stat-item">
                <span class="stat-label">High Priority</span>
                <span
                  class="stat-value"
                  :class="{ 'text-danger': requestDetail.indexAnalysis.summary.highPriorityRecs > 0 }"
                  >{{ requestDetail.indexAnalysis.summary.highPriorityRecs }}</span
                >
              </div>
            </div>

            <div v-if="requestDetail.indexAnalysis.sequentialScans?.length > 0" style="margin-top: 1rem">
              <h5 style="color: #8b949e; font-size: 0.9rem; margin-bottom: 0.5rem">Sequential Scan Issues</h5>
              <div class="index-issues-list">
                <div
                  v-for="(issue, idx) in requestDetail.indexAnalysis.sequentialScans.slice(0, 5)"
                  :key="idx"
                  class="index-issue-item"
                >
                  <div class="index-issue-header">
                    <span class="index-issue-table">{{ issue.tableName }}</span>
                    <span class="index-issue-stats"
                      >{{ issue.occurrences }}x · {{ issue.durationMs.toFixed(2) }}ms · cost:
                      {{ issue.cost.toFixed(0) }}</span
                    >
                  </div>
                  <div v-if="issue.filterCondition" class="index-issue-filter">Filter: {{ issue.filterCondition }}</div>
                </div>
              </div>
            </div>

            <div v-if="requestDetail.indexAnalysis.recommendations.length > 0" style="margin-top: 1rem">
              <h5 style="color: #8b949e; font-size: 0.9rem; margin-bottom: 0.5rem">Index Recommendations</h5>
              <div class="index-recommendations-list">
                <div
                  v-for="(rec, idx) in requestDetail.indexAnalysis.recommendations.slice(0, 5)"
                  :key="idx"
                  class="index-recommendation-item"
                >
                  <div class="index-recommendation-header">
                    <span class="index-priority-badge" :class="'priority-' + rec.priority">{{
                      rec.priority.toUpperCase()
                    }}</span>
                    <span class="index-recommendation-table">{{ rec.tableName }}</span>
                  </div>
                  <div class="index-recommendation-reason">{{ rec.reason }}</div>
                  <div class="index-recommendation-columns">Columns: {{ rec.columns.join(", ") }}</div>
                  <div class="index-recommendation-impact">Impact: {{ rec.estimatedImpact }}</div>
                  <div class="index-recommendation-sql">
                    <code>{{ rec.sqlCommand }}</code>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="modal-section">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem">
              <h4 style="margin: 0">
                Logs
                <span v-if="!showLiveLogStream"
                  >(<span>{{ filteredRequestLogs.length }}</span
                  >)
                  <span
                    v-if="requestDetail.logs.length > filteredRequestLogs.length"
                    style="color: #8b949e; font-size: 0.85rem; font-weight: normal"
                    >({{ requestDetail.logs.length - filteredRequestLogs.length }} TRACE filtered)</span
                  ></span
                >
                <span v-else style="color: #8b949e; font-size: 0.85rem; font-weight: normal">(Live Stream)</span>
              </h4>
              <div style="display: flex; gap: 0.5rem">
                <button
                  @click="toggleLogStream"
                  class="btn-secondary"
                  style="padding: 0.35rem 0.75rem; font-size: 0.85rem"
                >
                  {{ showLiveLogStream ? "📋 Show Saved" : "📡 Show Live" }}
                </button>
                <a
                  v-if="requestViewerLink"
                  :href="requestViewerLink"
                  target="_blank"
                  class="btn-primary"
                  style="padding: 0.35rem 0.75rem; font-size: 0.85rem; text-decoration: none"
                  >View in Log Viewer →</a
                >
              </div>
            </div>

            <!-- Saved Logs View -->
            <div v-if="!showLiveLogStream" class="logs-list">
              <p v-if="filteredRequestLogs.length === 0" style="color: #6c757d">
                No logs captured (or all logs are TRACE level)
              </p>
              <div v-for="(log, idx) in filteredRequestLogs" :key="idx" class="log-line" @click="openLogDetails(log)">
                <span v-if="log.timestamp" class="log-timestamp">{{ new Date(log.timestamp).toLocaleString() }}</span>
                <span v-if="log.level" class="log-level" :class="log.level">{{ log.level }}</span>
                <span v-if="log.file" class="log-file">{{ log.file }}</span>
                <span v-if="log.message" class="log-message">{{ log.message }}</span>
                <template v-for="([key, value], fieldIdx) in Object.entries(log.fields || {})" :key="fieldIdx">
                  <span class="log-field-key">{{ key }}</span
                  >=<span class="log-field-value">{{ formatFieldValue(value) }}</span>
                </template>
              </div>
            </div>

            <!-- Live Log Stream -->
            <log-stream
              v-else
              :request-id-filter="requestDetail.execution.requestIdHeader"
              :max-logs="1000"
              :auto-scroll="true"
              :compact="false"
              :show-container="true"
            />
          </div>
        </div>
      </main>
    </div>
  </div>

  <!-- Request Body Modal -->
  <div v-if="showRequestModal" class="modal" @click="showRequestModal = false">
    <div class="modal-content" @click.stop style="max-width: 900px">
      <div class="modal-header">
        <h3>
          Request Body<span v-if="isGraphQLRequest" style="color: #8b949e; font-size: 0.875rem; margin-left: 0.5rem"
            >GraphQL</span
          >
        </h3>
        <div style="display: flex; gap: 0.5rem">
          <button @click="copyToClipboard(requestData)" class="btn-secondary" style="padding: 0.5rem 1rem">
            📋 Copy
          </button>
          <button @click="showRequestModal = false">✕</button>
        </div>
      </div>
      <div class="modal-body">
        <!-- GraphQL Request Display -->
        <div v-if="isGraphQLRequest">
          <div v-if="isGraphQLBatchRequest" style="margin-bottom: 1rem">
            <h4 style="color: #8b949e; font-size: 0.875rem; margin-bottom: 0.5rem">
              Operations ({{ graphqlOperations.length }}):
            </h4>
            <select
              v-model="selectedOperationIndex"
              style="
                width: 100%;
                background: #161b22;
                border: 1px solid #30363d;
                border-radius: 4px;
                padding: 0.75rem;
                font-family:
                  Monaco,
                  Menlo,
                  Ubuntu Mono,
                  monospace;
                font-size: 0.875rem;
                color: #79c0ff;
                cursor: pointer;
              "
            >
              <option v-for="(op, idx) in graphqlOperations" :key="idx" :value="idx">{{ op.operationName }}</option>
            </select>
          </div>
          <div v-else-if="graphqlOperationName" style="margin-bottom: 1rem">
            <h4 style="color: #8b949e; font-size: 0.875rem; margin-bottom: 0.5rem">Operation:</h4>
            <div
              style="
                background: #161b22;
                border: 1px solid #30363d;
                border-radius: 4px;
                padding: 0.75rem;
                font-family:
                  Monaco,
                  Menlo,
                  Ubuntu Mono,
                  monospace;
                font-size: 0.875rem;
                color: #79c0ff;
              "
            >
              {{ graphqlOperationName }}
            </div>
          </div>
          <div style="margin-bottom: 1rem">
            <h4 style="color: #8b949e; font-size: 0.875rem; margin-bottom: 0.5rem">Query:</h4>
            <pre
              class="graphql-query"
              style="
                background: #0d1117;
                border: 1px solid #30363d;
                border-radius: 4px;
                padding: 1rem;
                overflow: auto;
                font-family:
                  Monaco,
                  Menlo,
                  Ubuntu Mono,
                  monospace;
                font-size: 0.875rem;
                line-height: 1.5;
                color: #c9d1d9;
                white-space: pre-wrap;
                word-break: break-word;
                max-height: 400px;
              "
              >{{ graphqlQuery }}</pre
            >
          </div>
          <div v-if="graphqlVariables">
            <h4 style="color: #8b949e; font-size: 0.875rem; margin-bottom: 0.5rem">Variables:</h4>
            <pre
              style="
                background: #0d1117;
                border: 1px solid #30363d;
                border-radius: 4px;
                padding: 1rem;
                overflow: auto;
                font-family:
                  Monaco,
                  Menlo,
                  Ubuntu Mono,
                  monospace;
                font-size: 0.875rem;
                line-height: 1.5;
                color: #c9d1d9;
                white-space: pre-wrap;
                word-break: break-word;
                max-height: 300px;
              "
              >{{ graphqlVariables }}</pre
            >
          </div>
        </div>

        <!-- Standard Request Display -->
        <pre
          v-else
          style="
            background: #0d1117;
            border: 1px solid #30363d;
            border-radius: 4px;
            padding: 1rem;
            overflow: auto;
            font-family:
              Monaco,
              Menlo,
              Ubuntu Mono,
              monospace;
            font-size: 0.875rem;
            line-height: 1.5;
            color: #c9d1d9;
            white-space: pre-wrap;
            word-break: break-word;
          "
          >{{ requestData }}</pre
        >
      </div>
    </div>
  </div>

  <!-- Response Body Modal -->
  <div v-if="showResponseModal" class="modal" @click="showResponseModal = false">
    <div class="modal-content" @click.stop style="max-width: 900px">
      <div class="modal-header">
        <h3>Response Body</h3>
        <div style="display: flex; gap: 0.5rem; align-items: center">
          <input
            type="text"
            v-model="responseFilter"
            placeholder="Filter (.data.users[0] or text)"
            style="
              flex: 1;
              max-width: 300px;
              padding: 0.5rem;
              background: #161b22;
              border: 1px solid #30363d;
              border-radius: 4px;
              color: #c9d1d9;
              font-family: monospace;
              font-size: 0.875rem;
            "
            title="Filter response JSON. Examples: .data, .data.users[0], .errors[1].message, or simple text search"
          />
          <button @click="copyToClipboard(filteredResponseBody)" class="btn-secondary" style="padding: 0.5rem 1rem">
            📋 Copy
          </button>
          <button @click="showResponseModal = false">✕</button>
        </div>
      </div>
      <div class="modal-body">
        <pre
          class="json-display"
          style="
            background: #0d1117;
            border: 1px solid #30363d;
            border-radius: 4px;
            padding: 1rem;
            overflow: auto;
            font-family:
              Monaco,
              Menlo,
              Ubuntu Mono,
              monospace;
            font-size: 0.875rem;
            line-height: 1.5;
            color: #c9d1d9;
            white-space: pre-wrap;
            word-break: break-word;
          "
          >{{ filteredResponseBody }}</pre
        >
      </div>
    </div>
  </div>

  <!-- EXPLAIN Plan Side Panel -->
  <div v-if="showExplainPlanPanel" class="side-panel-overlay" @click="closeExplainPlanModal">
    <div class="side-panel" @click.stop>
      <div class="side-panel-header">
        <h3>SQL Query EXPLAIN Plan (PEV2)</h3>
        <div style="display: flex; gap: 0.5rem">
          <button @click="shareExplainPlan" class="btn-secondary" style="padding: 0.5rem 1rem">📋 Share</button>
          <button @click="closeExplainPlanModal">✕</button>
        </div>
      </div>
      <div class="side-panel-body">
        <div
          v-if="explainPlanData && explainPlanData.error"
          class="alert alert-danger"
          style="display: block; margin: 1rem"
        >
          {{ explainPlanData.error }}
        </div>
        <div
          v-if="explainPlanData && !explainPlanData.error"
          id="pev2ExplainApp"
          class="d-flex flex-column"
          style="height: 100%"
        >
          <pev2 :plan-source="explainPlanData.planSource" :plan-query="explainPlanData.planQuery"></pev2>
        </div>
      </div>
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
          <pre
            v-html="convertAnsiToHtml(selectedLog.entry?.rawLog || selectedLog.entry?.raw || 'No raw log available')"
          ></pre>
        </div>
        <div class="modal-section">
          <h4>Parsed Fields</h4>
          <div>
            <div v-if="selectedLog.entry?.timestamp" class="parsed-field">
              <div class="parsed-field-key">Timestamp</div>
              <div class="parsed-field-value">{{ new Date(selectedLog.entry.timestamp).toLocaleString() }}</div>
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
              <div v-if="isSQLMessage(selectedLog.entry.message)" class="parsed-field-value">
                <pre
                  class="sql-query-text"
                  style="white-space: pre-wrap; margin: 0"
                  v-html="formatAndHighlightSQL(selectedLog.entry.message)"
                ></pre>
              </div>
              <div v-else class="parsed-field-value">{{ selectedLog.entry.message }}</div>
            </div>
            <div
              v-for="([key, value], idx) in Object.entries(selectedLog.entry?.fields || {})"
              :key="idx"
              class="parsed-field"
            >
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

  <!-- Execute Request Modal -->
  <div v-if="showExecuteModal" class="modal" @click="showExecuteModal = false">
    <div class="modal-content" @click.stop style="max-width: 900px">
      <div class="modal-header">
        <h3>Re-execute Request</h3>
        <button @click="showExecuteModal = false">✕</button>
      </div>
      <div class="modal-body">
        <div class="form-group">
          <label for="executeToken">Bearer Token Override (optional):</label>
          <input type="text" id="executeToken" v-model="executeForm.tokenOverride" placeholder="Override token" />
        </div>
        <div class="form-group">
          <label for="executeDevID">Dev ID Override (optional):</label>
          <input type="text" id="executeDevID" v-model="executeForm.devIdOverride" placeholder="Override dev ID" />
        </div>

        <div v-if="Object.keys(executeForm.graphqlVariables).length > 0" class="form-group" style="margin-top: 1.5rem">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem">
            <label style="margin: 0">GraphQL Variables:</label>
            <button
              @click="addGraphQLVariable"
              class="btn-secondary"
              style="padding: 0.25rem 0.5rem; font-size: 0.75rem"
            >
              + Add Variable
            </button>
          </div>
          <div style="background: #0d1117; border: 1px solid #30363d; border-radius: 4px; padding: 1rem">
            <div v-for="(value, key) in executeForm.graphqlVariables" :key="key" style="margin-bottom: 0.75rem">
              <div style="display: flex; gap: 0.5rem; align-items: center; margin-bottom: 0.25rem">
                <label style="min-width: 120px; color: #79c0ff; font-family: monospace; font-size: 0.875rem"
                  >{{ key }}:</label
                >
                <button
                  @click="removeGraphQLVariable(key)"
                  style="
                    background: #da3633;
                    color: white;
                    border: none;
                    padding: 0.25rem 0.5rem;
                    border-radius: 4px;
                    cursor: pointer;
                    font-size: 0.75rem;
                    margin-left: auto;
                  "
                >
                  Remove
                </button>
              </div>
              <textarea
                :value="typeof value === 'string' ? value : JSON.stringify(value, null, 2)"
                @input="updateGraphQLVariable(key, ($event.target as HTMLTextAreaElement).value)"
                style="
                  width: 100%;
                  background: #161b22;
                  border: 1px solid #30363d;
                  color: #c9d1d9;
                  padding: 0.5rem;
                  border-radius: 4px;
                  font-family: monospace;
                  font-size: 0.875rem;
                  min-height: 60px;
                  resize: vertical;
                "
                placeholder="Enter value (JSON if object/array)"
              ></textarea>
            </div>
          </div>
        </div>
        <div v-else class="form-group" style="margin-top: 1.5rem">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem">
            <label style="margin: 0">GraphQL Variables:</label>
            <button
              @click="addGraphQLVariable"
              class="btn-secondary"
              style="padding: 0.25rem 0.5rem; font-size: 0.75rem"
            >
              + Add Variable
            </button>
          </div>
          <p style="color: #8b949e; font-size: 0.875rem; margin: 0">
            No variables found in request body. Click "Add Variable" to add one.
          </p>
        </div>

        <div class="form-group" style="margin-top: 1.5rem">
          <label for="executeRequestData">Request Data:</label>
          <textarea
            id="executeRequestData"
            v-model="executeForm.requestDataOverride"
            placeholder="Enter or edit request data (JSON)"
            rows="12"
            style="font-family: monospace; font-size: 0.875rem"
          ></textarea>
        </div>
      </div>
      <div class="modal-footer">
        <button @click="executeRequest" class="btn-primary">Execute</button>
        <button @click="showExecuteModal = false" class="btn-secondary">Cancel</button>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import { useRoute } from "vue-router";
import { API } from "@/utils/api";
import {
  formatSQL as formatSQLUtil,
  convertAnsiToHtml as convertAnsiToHtmlUtil,
  copyToClipboard,
  normalizeQuery as normalizeQueryUtil,
  applySyntaxHighlighting,
} from "@/utils/ui-utils";
import type { Server, ExecutionDetail, ExplainResponse, ExplainData, ExecuteResponse, SQLQuery } from "@/types";
import { Plan } from "pev2";

export default defineComponent({
  components: {
    pev2: Plan,
  },
  setup() {
    const route = useRoute();
    return { route };
  },
  data() {
    return {
      requestDetail: null as ExecutionDetail | null,
      loading: true,
      error: null as string | null,
      explainPlanData: null as ExplainData | null,
      showExplainPlanPanel: false,
      showRequestModal: false,
      showResponseModal: false,
      showExecuteModal: false,
      showLogModal: false, // For log details modal
      selectedLog: null, // Currently selected log for details view
      executeForm: {
        tokenOverride: "",
        devIdOverride: "",
        requestDataOverride: "",
        graphqlVariables: {} as Record<string, any>,
      },
      servers: [] as Server[],
      showLiveLogStream: false, // Toggle between saved logs and live stream
      refreshTimer: null, // Timer for auto-refreshing recent requests
      selectedOperationIndex: 0, // For GraphQL batch requests
      responseFilter: "", // Search/filter for response JSON
    };
  },

  computed: {
    filteredRequestLogs() {
      if (!this.requestDetail || !this.requestDetail.logs) {
        return [];
      }
      // Filter out TRACE level logs (case-insensitive) and parse fields JSON
      return this.requestDetail.logs
        .filter((log) => {
          const level = (log.level || "").toUpperCase();
          return level !== "TRC" && level !== "TRACE";
        })
        .map((log) => {
          // Parse fields if it's a JSON string
          if (typeof log.fields === "string" && log.fields) {
            try {
              return { ...log, fields: JSON.parse(log.fields) };
            } catch {
              return log;
            }
          }
          return log;
        });
    },

    requestViewerLink() {
      if (!this.requestDetail || !this.requestDetail.execution.requestIdHeader) {
        return null;
      }
      const requestId = this.requestDetail.execution.requestIdHeader;
      return `/logs/?request_id=${encodeURIComponent(requestId)}`;
    },

    statusClass() {
      if (!this.requestDetail) return "";
      const statusCode = this.requestDetail.execution.statusCode;
      const hasError = this.requestDetail.execution.error;
      if (!statusCode || statusCode === 0) return "pending";
      // Show error class even for 200 status if there's an error (e.g., GraphQL errors)
      if (hasError) return "error";
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
      if (Array.isArray(data)) {
        return data.some((item) => item && (item.query || item.operationName));
      }
      return !!(data && (data.query || data.operationName));
    },

    isGraphQLBatchRequest() {
      const data = this.parsedRequestBody;
      return Array.isArray(data) && data.some((item) => item && (item.query || item.operationName));
    },

    graphqlOperations() {
      if (!this.isGraphQLBatchRequest) return [];
      return this.parsedRequestBody.map((item, idx) => ({
        index: idx,
        operationName: item.operationName || `Operation ${idx + 1}`,
        query: item.query || "",
        variables: item.variables || null,
      }));
    },

    graphqlQuery() {
      if (!this.isGraphQLRequest) return null;
      if (this.isGraphQLBatchRequest) {
        const selected = this.graphqlOperations[this.selectedOperationIndex || 0];
        return selected ? selected.query : "";
      }
      return this.parsedRequestBody.query || "";
    },

    graphqlOperationName() {
      if (!this.isGraphQLRequest) return null;
      if (this.isGraphQLBatchRequest) {
        return null; // We'll show dropdown instead
      }
      return this.parsedRequestBody.operationName || null;
    },

    graphqlVariables() {
      if (!this.isGraphQLRequest) return null;
      if (this.isGraphQLBatchRequest) {
        const selected = this.graphqlOperations[this.selectedOperationIndex || 0];
        const variables = selected ? selected.variables : null;
        return variables ? JSON.stringify(variables, null, 2) : null;
      }
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

    filteredResponseBody() {
      if (!this.responseFilter.trim()) {
        return this.responseBody;
      }

      try {
        const data = JSON.parse(this.requestDetail.execution.responseBody);
        const filter = this.responseFilter.toLowerCase();

        // Simple JSON path filtering (supports basic jq-like syntax)
        const filtered = this.filterJSON(data, filter);
        return JSON.stringify(filtered, null, 2);
      } catch (e) {
        // If filtering fails, fall back to text search
        console.error("Error filtering response body:", e);
        const lines = this.responseBody.split("\n");
        const matchedLines = lines.filter((line) => line.toLowerCase().includes(this.responseFilter.toLowerCase()));
        return matchedLines.length > 0 ? matchedLines.join("\n") : "(no matches)";
      }
    },

    // Calculate the age of the request in minutes
    requestAgeMinutes() {
      if (!this.requestDetail?.execution.executedAt) return Infinity;
      const executedAt = new Date(this.requestDetail.execution.executedAt);
      const now = new Date();
      return (now.getTime() - executedAt.getTime()) / 1000 / 60; // Convert milliseconds to minutes
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
        normalized: normalizeQueryUtil(q.query),
      }));

      const totalQueries = queries.length;
      const totalDuration = queries.reduce((sum, q) => sum + q.duration, 0);
      const avgDuration = totalDuration / totalQueries;

      const queryGroups: Record<string, { queries: SQLQuery[]; count: number }> = {};
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

      const tables: Record<string, number> = {};
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
    const requestId = this.route.params.id as string;

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

  watch: {
    selectedOperationIndex(newIndex) {
      // Auto-populate response filter for batch requests
      if (this.isGraphQLBatchRequest)
        if (!this.responseFilter.trim() || this.responseFilter.trim().match(/^\.\[\d+\]$/)) {
          this.responseFilter = `.[${newIndex}]`;
        }

      // Re-apply syntax highlighting when operation selection changes
      this.$nextTick(() => {
        // Remove hljs class to force re-highlighting
        document.querySelectorAll(".graphql-query.hljs, .json-display.hljs").forEach((block) => {
          block.classList.remove("hljs");
        });
        this.applySyntaxHighlighting();
      });
    },

    responseFilter() {
      // Re-apply syntax highlighting when response filter changes
      this.$nextTick(() => {
        document.querySelectorAll(".json-display.hljs").forEach((block) => {
          block.classList.remove("hljs");
        });
        this.applySyntaxHighlighting();
      });
    },
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
      applySyntaxHighlighting();
    },
    async loadRequestDetail(requestId) {
      try {
        console.log(`Loading request detail for ID: ${requestId}`);
        this.requestDetail = await API.get<ExecutionDetail>(`/api/executions/${requestId}`);
        this.loading = false;
        console.log("Request detail loaded successfully:", this.requestDetail);

        // Auto-populate response filter for batch GraphQL requests
        if (this.isGraphQLBatchRequest && !this.responseFilter.trim()) {
          this.responseFilter = `.[${this.selectedOperationIndex}]`;
        }

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
        this.error = error.message || String(error);
        this.loading = false;
      }
    },

    setupRefreshTimer(requestId, interval = 500) {
      // Clear any existing timer
      if (this.refreshTimer) {
        clearTimeout(this.refreshTimer);
      }

      // Set up new timer to refresh in 30 seconds
      this.refreshTimer = setTimeout(async () => {
        console.log("Auto-refreshing request details...");
        try {
          this.requestDetail = await API.get<ExecutionDetail>(`/api/executions/${requestId}`);

          // Check if we should continue refreshing
          const ageMinutes = this.requestAgeMinutes;
          if (ageMinutes < 1) {
            // Still less than 1 minute old, refresh again
            this.setupRefreshTimer(requestId, Math.min(interval * 2, 10000));
          } else {
            console.log("Request is now over 1 minute old - stopping auto-refresh");
          }
        } catch (error) {
          console.error("Failed to auto-refresh request details:", error);
        }
      }, interval); // 10 seconds
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
      // Convert planData to JSON string for PEV2 component
      const planSource = typeof planData === "string" ? planData : JSON.stringify(planData, null, 2);

      this.explainPlanData = {
        planSource,
        planQuery: query,
        error: null,
      };

      this.showExplainPlanPanel = true;
    },

    closeExplainPlanModal() {
      this.showExplainPlanPanel = false;
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
          const query = this.explainPlanData.planQuery.toLowerCase();
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
        planInput.value = this.explainPlanData.planSource;
        form.appendChild(planInput);

        const queryInput = document.createElement("input");
        queryInput.type = "hidden";
        queryInput.name = "query";
        queryInput.value = formatSQLUtil(this.explainPlanData.planQuery) || this.explainPlanData.planQuery;
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
        // Convert variables to string map (backend expects map[string]string)
        const vars = {};
        if (Array.isArray(variables)) {
          for (let i = 0; i < variables.length; i++) {
            const value = variables[i];
            vars[`${i + 1}`] = typeof value === "string" ? value : JSON.stringify(value);
          }
        } else {
          for (const [key, value] of Object.entries(variables)) {
            vars[key] = typeof value === "string" ? value : JSON.stringify(value);
          }
        }

        const payload = {
          query: query,
          variables: vars,
          connectionString: connectionString,
        };

        const result = await API.post<ExplainResponse>("/api/explain", payload);

        if (result.error) {
          alert(`EXPLAIN Error: ${result.error}`);
        } else {
          const displayQuery = result.query || query;
          this.displayExplainPlan(result.queryPlan, formatSQLUtil(displayQuery));
        }
      } catch (error: Error | any) {
        alert(`Failed to run EXPLAIN: ${error instanceof Error ? error.message : String(error)}`);
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
        this.servers = await API.get<Server[]>("/api/servers");
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

        const result = await API.post<ExecuteResponse>("/api/execute", payload);

        // Navigate to new execution detail
        if (result.executionId) {
          window.location.href = `/requests/detail/?id=${result.executionId}`;
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

        const result = await API.post<ExecuteResponse>("/api/execute", payload);

        // Close modal
        this.showExecuteModal = false;

        // Navigate to new execution detail
        if (result.executionId) {
          window.location.href = `/requests/detail/?id=${result.executionId}`;
        }
      } catch (error) {
        console.error("Failed to execute request:", error);
        alert(`Failed to execute request: ${error.message}`);
      }
    },

    toggleLogStream() {
      this.showLiveLogStream = !this.showLiveLogStream;
    },

    filterJSON(obj, query) {
      // Support basic jq-like filtering
      // Examples: ".data", ".data.users", ".data.users[0]", ".data[1].name", etc.

      if (query.startsWith(".")) {
        // Path-based filtering (jq style) with array support
        const pathStr = query.substring(1);
        // Split by . but preserve array indices
        const pathParts = this.parseJSONPath(pathStr);

        let result = obj;
        for (const part of pathParts) {
          if (result === null || result === undefined) {
            return null;
          }

          if (part.type === "key") {
            if (typeof result === "object" && part.value in result) {
              result = result[part.value];
            } else {
              return null;
            }
          } else if (part.type === "index") {
            if (Array.isArray(result) && part.value >= 0 && part.value < result.length) {
              result = result[part.value];
            } else {
              return null;
            }
          } else if (part.type === "all") {
            // [] means all elements in array
            if (Array.isArray(result)) {
              return result;
            } else {
              return null;
            }
          }
        }
        return result;
      } else {
        // Recursive text search through JSON
        return this.searchInJSON(obj, query);
      }
    },

    parseJSONPath(path) {
      // Parse a path like "data.users[0].name" or "data[1]" into parts
      const parts = [];
      let current = "";
      let i = 0;

      while (i < path.length) {
        const char = path[i];

        if (char === "[") {
          // Save the current key if any
          if (current) {
            parts.push({ type: "key", value: current });
            current = "";
          }

          // Find the closing bracket
          const closeIdx = path.indexOf("]", i);
          if (closeIdx === -1) {
            throw new Error("Unclosed bracket in path");
          }

          const indexStr = path.substring(i + 1, closeIdx);
          if (indexStr === "") {
            // Empty brackets [] means all elements
            parts.push({ type: "all" });
          } else {
            const index = parseInt(indexStr, 10);
            if (isNaN(index)) {
              throw new Error(`Invalid array index: ${indexStr}`);
            }
            parts.push({ type: "index", value: index });
          }

          i = closeIdx + 1;
        } else if (char === ".") {
          // Save the current key
          if (current) {
            parts.push({ type: "key", value: current });
            current = "";
          }
          i++;
        } else {
          current += char;
          i++;
        }
      }

      // Save any remaining key
      if (current) {
        parts.push({ type: "key", value: current });
      }

      return parts;
    },

    searchInJSON(obj, query) {
      if (obj === null || obj === undefined) return null;

      if (typeof obj === "string" && obj.toLowerCase().includes(query)) {
        return obj;
      }

      if (Array.isArray(obj)) {
        const filtered = obj.map((item) => this.searchInJSON(item, query)).filter((item) => item !== null);
        return filtered.length > 0 ? filtered : null;
      }

      if (typeof obj === "object") {
        const result = {};
        let hasMatch = false;

        for (const [key, value] of Object.entries(obj)) {
          if (key.toLowerCase().includes(query)) {
            result[key] = value;
            hasMatch = true;
          } else {
            const filtered = this.searchInJSON(value, query);
            if (filtered !== null) {
              result[key] = filtered;
              hasMatch = true;
            }
          }
        }

        return hasMatch ? result : null;
      }

      return null;
    },

    // Methods for displaying saved logs similar to regular log viewer
    shouldShowField(key, value) {
      // Always show error field
      if (key === "error") return true;
      // Show fields less than 40 characters
      const s = String(value);
      return s.length < 40;
    },

    formatFieldValue(value) {
      const s = String(value);
      return s.length > 50 ? s.substring(0, 50) + "..." : s;
    },

    isJsonField(value) {
      const s = String(value);
      return s.trim().startsWith("{") || s.trim().startsWith("[");
    },

    formatJsonField(value) {
      try {
        const parsed = JSON.parse(value);
        return JSON.stringify(parsed, null, 2);
      } catch {
        return value;
      }
    },

    openLogDetails(log) {
      // Parse fields if it's a JSON string
      let parsedLog = log;
      if (typeof log.fields === "string" && log.fields) {
        try {
          parsedLog = { ...log, fields: JSON.parse(log.fields) };
        } catch {
          parsedLog = log;
        }
      }
      this.selectedLog = { entry: parsedLog };
      this.showLogModal = true;
      // Apply syntax highlighting after modal opens
      this.$nextTick(() => {
        this.applySyntaxHighlighting();
      });
    },

    // Wrapper methods for template usage (templates can't call imported functions directly)
    formatSQL: formatSQLUtil,
    copyToClipboard,
    convertAnsiToHtml: convertAnsiToHtmlUtil,

    isSQLMessage(message) {
      return message && message.includes("[sql]");
    },

    formatAndHighlightSQL(message) {
      if (!message) return message;
      if (message.includes("[sql]")) {
        const sqlMatch = message.match(/\[sql\]:\s*(.+)/i);
        if (sqlMatch) {
          const sql = sqlMatch[1].trim();
          const formatted = formatSQLUtil(sql);
          // Apply syntax highlighting if hljs is available
          if (typeof hljs !== "undefined") {
            try {
              const highlighted = hljs.highlight(formatted, { language: "sql" });
              return highlighted.value;
            } catch (e) {
              console.error("Error highlighting SQL:", e);
              return formatted;
            }
          }
          return formatted;
        }
      }
      return message;
    },
  },
});
</script>
