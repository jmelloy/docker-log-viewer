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
            <div style="display: flex; gap: 0.5rem">
              <button @click="exportAsMarkdown" class="btn-secondary" title="Export as Markdown">
                📄 Export Markdown
              </button>
              <button @click="exportToNotion" class="btn-secondary" title="Export to Notion">
                📘 Export to Notion
              </button>
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
          <div class="modal-section">
            <h4>EXPLAIN Plan</h4>
            <explain-plan-formatter
              :explain-plan="sqlDetail.explainPlan || ''"
              :query="sqlDetail.query"
              default-mode="visual"
            />
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
import ExplainPlanFormatter from "@/components/ExplainPlanFormatter.vue";
import type { SQLQueryDetail } from "@/types";

export default defineComponent({
  name: "SqlDetailView",
  components: {
    AppHeader,
    ExplainPlanFormatter,
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

    generateMarkdown(): string {
      if (!this.sqlDetail) return "";

      const now = new Date();
      const formattedDate = now.toLocaleString();

      let markdown = `# SQL Query Analysis Report\n\n`;
      markdown += `**Generated:** ${formattedDate}\n\n`;

      // Add request IDs if available
      if (this.sqlDetail.relatedExecutions && this.sqlDetail.relatedExecutions.length > 0) {
        const firstExec = this.sqlDetail.relatedExecutions[0];
        markdown += `**Request ID:** ${firstExec.requestIdHeader}\n`;
        markdown += `**Execution Date:** ${new Date(firstExec.executedAt).toLocaleString()}\n\n`;
      }

      // Query metadata
      markdown += `## Query Information\n\n`;
      markdown += `- **Operation:** ${this.sqlDetail.operation}\n`;
      markdown += `- **Table:** ${this.sqlDetail.tableName}\n`;
      markdown += `- **Total Executions:** ${this.sqlDetail.totalExecutions}\n`;
      markdown += `- **Average Duration:** ${this.sqlDetail.avgDuration.toFixed(2)}ms\n`;
      markdown += `- **Min Duration:** ${this.sqlDetail.minDuration.toFixed(2)}ms\n`;
      markdown += `- **Max Duration:** ${this.sqlDetail.maxDuration.toFixed(2)}ms\n\n`;

      // SQL Query
      markdown += `## SQL Query\n\n`;
      markdown += `\`\`\`sql\n${this.formatSQL(this.sqlDetail.query)}\n\`\`\`\n\n`;

      // Normalized Query
      markdown += `## Normalized Query\n\n`;
      markdown += `\`\`\`sql\n${this.formatSQL(this.sqlDetail.normalizedQuery)}\n\`\`\`\n\n`;

      // EXPLAIN Plan (text version)
      if (this.sqlDetail.explainPlan) {
        markdown += `## EXPLAIN Plan\n\n`;
        try {
          // Try to format as text if it's JSON
          const parsed = JSON.parse(this.sqlDetail.explainPlan);
          markdown += `\`\`\`\n${this.formatExplainPlanAsText(parsed)}\n\`\`\`\n\n`;
        } catch {
          // If not JSON, use as-is
          markdown += `\`\`\`\n${this.sqlDetail.explainPlan}\n\`\`\`\n\n`;
        }
      }

      // Index recommendations
      if (
        this.sqlDetail.indexAnalysis?.recommendations &&
        this.sqlDetail.indexAnalysis.recommendations.length > 0
      ) {
        markdown += `## Index Recommendations\n\n`;
        this.sqlDetail.indexAnalysis.recommendations.forEach((rec: any, index: number) => {
          markdown += `### ${index + 1}. ${rec.tableName} (${rec.priority.toUpperCase()})\n\n`;
          markdown += `- **Reason:** ${rec.reason}\n`;
          markdown += `- **Columns:** ${rec.columns.join(", ")}\n`;
          markdown += `- **Impact:** ${rec.estimatedImpact}\n`;
          if (rec.sql) {
            markdown += `- **SQL:**\n  \`\`\`sql\n  ${rec.sql}\n  \`\`\`\n`;
          }
          markdown += `\n`;
        });
      }

      // Related executions
      if (this.sqlDetail.relatedExecutions && this.sqlDetail.relatedExecutions.length > 0) {
        markdown += `## Related Executions\n\n`;
        markdown += `| Request ID | Display Name | Status | Duration | Executed At |\n`;
        markdown += `|------------|--------------|--------|----------|-------------|\n`;
        this.sqlDetail.relatedExecutions.forEach((exec: any) => {
          markdown += `| ${exec.requestIdHeader} | ${exec.displayName} | ${exec.statusCode} | ${exec.durationMs.toFixed(2)}ms | ${new Date(exec.executedAt).toLocaleString()} |\n`;
        });
      }

      return markdown;
    },

    formatExplainPlanAsText(plan: any, indent: number = 0): string {
      // Convert JSON explain plan to human-readable text format
      const indentStr = "  ".repeat(indent);
      let text = "";

      if (Array.isArray(plan)) {
        plan.forEach((item) => {
          text += this.formatExplainPlanAsText(item, indent);
        });
      } else if (typeof plan === "object" && plan !== null) {
        if (plan.Plan) {
          // PostgreSQL EXPLAIN format
          text += `${indentStr}${plan.Plan["Node Type"] || "Unknown"}`;
          if (plan.Plan["Relation Name"]) {
            text += ` on ${plan.Plan["Relation Name"]}`;
          }
          if (plan.Plan["Alias"]) {
            text += ` (${plan.Plan["Alias"]})`;
          }
          text += `\n`;
          text += `${indentStr}  Cost: ${plan.Plan["Startup Cost"]?.toFixed(2) || 0}..${plan.Plan["Total Cost"]?.toFixed(2) || 0}`;
          text += ` Rows: ${plan.Plan["Plan Rows"] || 0}`;
          if (plan.Plan["Actual Rows"] !== undefined) {
            text += ` Actual: ${plan.Plan["Actual Rows"]}`;
          }
          text += `\n`;

          if (plan.Plan["Filter"]) {
            text += `${indentStr}  Filter: ${plan.Plan["Filter"]}\n`;
          }
          if (plan.Plan["Index Cond"]) {
            text += `${indentStr}  Index Cond: ${plan.Plan["Index Cond"]}\n`;
          }

          if (plan.Plan["Plans"]) {
            plan.Plan["Plans"].forEach((subPlan: any) => {
              text += this.formatExplainPlanAsText({ Plan: subPlan }, indent + 1);
            });
          }

          if (plan["Planning Time"]) {
            text += `\nPlanning Time: ${plan["Planning Time"].toFixed(3)}ms\n`;
          }
          if (plan["Execution Time"]) {
            text += `Execution Time: ${plan["Execution Time"].toFixed(3)}ms\n`;
          }
        } else {
          // Simple object format
          Object.entries(plan).forEach(([key, value]) => {
            if (typeof value === "object" && value !== null) {
              text += `${indentStr}${key}:\n`;
              text += this.formatExplainPlanAsText(value, indent + 1);
            } else {
              text += `${indentStr}${key}: ${value}\n`;
            }
          });
        }
      } else {
        text += `${indentStr}${plan}\n`;
      }

      return text;
    },

    exportAsMarkdown() {
      if (!this.sqlDetail) return;

      const markdown = this.generateMarkdown();
      const blob = new Blob([markdown], { type: "text/markdown;charset=utf-8" });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      const filename = `sql-query-${this.sqlDetail.queryHash}-${Date.now()}.md`;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
    },

    async exportToNotion() {
      if (!this.sqlDetail) return;

      try {
        const response = await fetch(`/api/sql/${this.queryHash}/export-notion`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
        });

        if (!response.ok) {
          const error = await response.text();
          throw new Error(error || "Failed to export to Notion");
        }

        const result = await response.json();
        if (result.url) {
          alert(`Successfully exported to Notion!\nPage URL: ${result.url}`);
          // Optionally open the page
          window.open(result.url, "_blank");
        } else {
          alert("Successfully exported to Notion!");
        }
      } catch (err: any) {
        console.error("Error exporting to Notion:", err);
        alert(`Failed to export to Notion: ${err.message}`);
      }
    },
  },
});
</script>

