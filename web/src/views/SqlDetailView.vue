<template>
  <div class="app-container">
    <app-header activePage="requests"></app-header>

    <div class="main-layout">
      <main class="content content-padded">
        <div v-if="loading" class="text-center p-3">
          <p>Loading SQL query details...</p>
        </div>

        <div v-if="error" class="text-center p-3">
          <div class="alert alert-danger">{{ error }}</div>
          <button @click="goBack" class="btn-secondary">Go Back</button>
        </div>

        <div v-if="!loading && !error && sqlDetail">
          <div class="flex-between mb-1_5">
            <div>
              <button @click="goBack" class="btn-secondary mb-0_5">← Back</button>
              <h2 class="m-0">SQL Query Details</h2>
              <p class="text-muted mt-0_25">{{ sqlDetail.operation }} on {{ sqlDetail.tableName }}</p>
            </div>
          </div>

          <!-- Statistics Overview -->
          <div class="execution-overview">
            <div class="stat-item">
              <span class="stat-label">Total Executions</span>
              <span class="stat-value">{{ sqlDetail.totalExecutions }}</span>
            </div>
            <div class="stat-item">
              <span class="stat-label">Avg Duration</span>
              <span class="stat-value">{{ sqlDetail.avgDuration.toFixed(2) }}ms</span>
            </div>
            <div class="stat-item">
              <span class="stat-label">Min Duration</span>
              <span class="stat-value">{{ sqlDetail.minDuration.toFixed(2) }}ms</span>
            </div>
            <div class="stat-item">
              <span class="stat-label">Max Duration</span>
              <span class="stat-value">{{ sqlDetail.maxDuration.toFixed(2) }}ms</span>
            </div>
            <div class="stat-item">
              <span class="stat-label">Operation</span>
              <span class="stat-value">{{ sqlDetail.operation }}</span>
            </div>
          </div>

          <!-- SQL Query -->
          <div class="modal-section">
            <div
              style="
                display: flex;
                justify-content: space-between;
                align-items: center;
                margin-bottom: 0.5rem;
                gap: 0.5rem;
              "
            >
              <h4 style="margin: 0">SQL Query</h4>
              <button
                @click="copyToClipboard(sqlDetail.query)"
                class="btn-secondary"
                style="padding: 0.25rem 0.5rem; font-size: 0.75rem"
                title="Copy SQL query"
              >
                📋 Copy
              </button>
            </div>
            <pre class="json-display" style="white-space: pre-wrap; max-height: 20em">{{
              formatSQL(sqlDetail.query)
            }}</pre>
          </div>

          <!-- Normalized Query -->
          <div class="modal-section">
            <h4>Normalized Query</h4>
            <pre class="json-display" style="white-space: pre-wrap; max-height: 15em">{{
              formatSQL(sqlDetail.normalizedQuery)
            }}</pre>
          </div>

          <!-- EXPLAIN Plan -->
          <div v-if="sqlDetail.explainPlan" class="modal-section">
            <h4>EXPLAIN Plan</h4>
            <pre class="json-display" style="white-space: pre-wrap; max-height: 30em">{{
              formatExplainPlan(sqlDetail.explainPlan)
            }}</pre>
          </div>
          <div v-else class="modal-section">
            <h4>EXPLAIN Plan</h4>
            <p class="text-muted">No EXPLAIN plan available for this query.</p>
          </div>

          <!-- Index Recommendations -->
          <div
            v-if="
              sqlDetail.indexAnalysis &&
              sqlDetail.indexAnalysis.recommendations &&
              sqlDetail.indexAnalysis.recommendations.length > 0
            "
            class="modal-section"
          >
            <h4>Index Recommendations</h4>
            <div class="index-recommendations-list">
              <div
                v-for="(rec, index) in sqlDetail.indexAnalysis.recommendations"
                :key="index"
                class="index-recommendation-item"
              >
                <div class="index-rec-header">
                  <span class="index-priority-badge" :class="'priority-' + rec.priority">
                    {{ rec.priority.toUpperCase() }}
                  </span>
                  <span class="index-rec-table">{{ rec.tableName }}</span>
                </div>
                <div class="index-rec-reason">{{ rec.reason }}</div>
                <div class="index-rec-columns"><strong>Columns:</strong> {{ rec.columns.join(", ") }}</div>
                <div class="index-rec-impact">{{ rec.estimatedImpact }}</div>
                <div v-if="rec.sql" class="index-rec-sql">
                  <strong>SQL:</strong>
                  <pre style="margin-top: 0.25rem">{{ rec.sql }}</pre>
                </div>
              </div>
            </div>
          </div>

          <!-- Sequential Scans -->
          <div
            v-if="
              sqlDetail.indexAnalysis &&
              sqlDetail.indexAnalysis.sequentialScans &&
              sqlDetail.indexAnalysis.sequentialScans.length > 0
            "
            class="modal-section"
          >
            <h4>Sequential Scan Issues</h4>
            <div class="index-issues-list">
              <div v-for="(issue, idx) in sqlDetail.indexAnalysis.sequentialScans" :key="idx" class="index-issue-item">
                <div class="index-issue-header">
                  <span class="index-issue-table">{{ issue.tableName }}</span>
                  <span class="index-issue-stats">
                    {{ issue.occurrences }}x · {{ issue.durationMs.toFixed(2) }}ms · cost: {{ issue.cost.toFixed(0) }}
                  </span>
                </div>
                <div v-if="issue.filterCondition" class="index-issue-filter">Filter: {{ issue.filterCondition }}</div>
              </div>
            </div>
          </div>

          <!-- Related Executions -->
          <div class="modal-section">
            <h4>
              Appears in {{ sqlDetail.relatedExecutions.length }} Request{{
                sqlDetail.relatedExecutions.length !== 1 ? "s" : ""
              }}
            </h4>
            <div class="executions-list">
              <div
                v-for="exec in sqlDetail.relatedExecutions"
                :key="exec.id"
                class="execution-item-compact"
                @click="navigateToRequest(exec.id)"
                style="cursor: pointer"
              >
                <span class="exec-status" :class="getStatusClass(exec.statusCode)">{{ exec.statusCode }}</span>
                <span class="exec-name">{{ exec.displayName }}</span>
                <span class="exec-time">{{ new Date(exec.executedAt).toLocaleString() }}</span>
                <span class="exec-duration">{{ exec.durationMs.toFixed(2) }}ms</span>
                <span class="exec-id">{{ exec.requestIdHeader }}</span>
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import AppHeader from "@/components/AppHeader.vue";
import type { SQLQueryDetail } from "@/types";

export default defineComponent({
  name: "SqlDetailView",
  components: {
    AppHeader,
  },
  data() {
    return {
      queryHash: "" as string,
      sqlDetail: null as SQLQueryDetail | null,
      loading: true,
      error: null as string | null,
    };
  },
  mounted() {
    this.queryHash = this.$route.params.hash as string;
    this.loadSQLDetail();
  },
  methods: {
    async loadSQLDetail() {
      this.loading = true;
      this.error = null;

      try {
        const response = await fetch(`/api/sql/${this.queryHash}`);
        if (!response.ok) {
          throw new Error(`Failed to load SQL query: ${response.statusText}`);
        }
        this.sqlDetail = await response.json();
      } catch (err: any) {
        this.error = err.message || "Failed to load SQL query details";
        console.error("Error loading SQL detail:", err);
      } finally {
        this.loading = false;
      }
    },

    formatSQL(sql: string): string {
      if (!sql) return "";
      // Basic SQL formatting - add newlines before major keywords
      return sql
        .replace(
          /\s+(SELECT|FROM|WHERE|JOIN|LEFT JOIN|RIGHT JOIN|INNER JOIN|ORDER BY|GROUP BY|HAVING|LIMIT|OFFSET)\s+/gi,
          "\n$1 "
        )
        .trim();
    },

    formatExplainPlan(plan: string): string {
      if (!plan) return "";
      try {
        // Try to parse as JSON and pretty-print
        const parsed = JSON.parse(plan);
        return JSON.stringify(parsed, null, 2);
      } catch {
        // If not JSON, return as-is
        return plan;
      }
    },

    copyToClipboard(text: string) {
      navigator.clipboard.writeText(text).then(
        () => {
          alert("Copied to clipboard!");
        },
        (err) => {
          console.error("Failed to copy:", err);
        }
      );
    },

    getStatusClass(statusCode: number): string {
      if (statusCode >= 200 && statusCode < 300) {
        return "status-success";
      } else if (statusCode >= 400 && statusCode < 500) {
        return "status-client-error";
      } else if (statusCode >= 500) {
        return "status-server-error";
      }
      return "";
    },

    navigateToRequest(requestId: number) {
      this.$router.push(`/requests/${requestId}`);
    },

    goBack() {
      this.$router.back();
    },
  },
});
</script>

<style scoped>
.execution-item-compact {
  transition: background-color 0.15s ease;
}

.execution-item-compact:hover {
  background-color: rgba(110, 118, 129, 0.15);
}

.index-recommendations-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.index-recommendation-item {
  background: #161b22;
  border: 1px solid #30363d;
  border-radius: 6px;
  padding: 1rem;
}

.index-rec-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
}

.index-rec-table {
  font-weight: 600;
  color: #c9d1d9;
}

.index-rec-reason {
  color: #8b949e;
  font-size: 0.875rem;
  margin-bottom: 0.5rem;
}

.index-rec-columns {
  color: #c9d1d9;
  font-size: 0.875rem;
  margin-bottom: 0.5rem;
}

.index-rec-impact {
  color: #7ee787;
  font-size: 0.875rem;
  margin-bottom: 0.5rem;
}

.index-rec-sql pre {
  background: #0d1117;
  border: 1px solid #30363d;
  border-radius: 4px;
  padding: 0.5rem;
  font-size: 0.75rem;
  overflow-x: auto;
}

.index-priority-badge {
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
}

.index-priority-badge.priority-high {
  background: rgba(248, 81, 73, 0.2);
  color: #f85149;
}

.index-priority-badge.priority-medium {
  background: rgba(187, 128, 9, 0.2);
  color: #bb8009;
}

.index-priority-badge.priority-low {
  background: rgba(56, 139, 253, 0.2);
  color: #388bfd;
}

.index-issues-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.index-issue-item {
  background: #161b22;
  border: 1px solid #30363d;
  border-radius: 6px;
  padding: 0.75rem;
}

.index-issue-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.5rem;
}

.index-issue-table {
  font-weight: 600;
  color: #c9d1d9;
}

.index-issue-stats {
  color: #8b949e;
  font-size: 0.875rem;
}

.index-issue-filter {
  color: #8b949e;
  font-size: 0.875rem;
  font-family: monospace;
}

.status-success {
  background-color: #238636 !important;
  color: white !important;
}

.status-client-error {
  background-color: #bb8009 !important;
  color: white !important;
}

.status-server-error {
  background-color: #f85149 !important;
  color: white !important;
}
</style>
