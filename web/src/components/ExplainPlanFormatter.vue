<template>
  <div class="explain-plan-formatter">
    <div v-if="error" class="alert alert-danger">
      {{ error }}
    </div>
    <div v-else-if="!explainPlan" class="text-muted">No EXPLAIN plan available.</div>
    <div v-else class="explain-plan-content">
      <!-- Display mode tabs -->
      <div class="display-mode-tabs">
        <button
          :class="['tab-button', { active: displayMode === 'visual' }]"
          :disabled="!hasValidPlan"
          @click="displayMode = 'visual'"
        >
          📊 Visual
        </button>
        <button :class="['tab-button', { active: displayMode === 'json' }]" @click="displayMode = 'json'">
          📄 JSON
        </button>
        <button :class="['tab-button', { active: displayMode === 'text' }]" @click="displayMode = 'text'">
          📝 Text
        </button>
      </div>

      <!-- Visual view using simple query plan viewer -->
      <div v-if="displayMode === 'visual' && hasValidPlan" class="visual-view">
        <div v-if="formattedQuery && showQuery" class="query-section">
          <div class="query-header">
            <h4>Query</h4>
            <button class="btn-copy" title="Copy query to clipboard" @click="copyToClipboard(query)">📋 Copy</button>
          </div>
          <pre class="sql-query-display"><code class="language-sql">{{ formattedQuery }}</code></pre>
        </div>
        <SimpleQueryPlanViewer :plan-source="formattedPlan" />
      </div>

      <!-- JSON view -->
      <div v-if="displayMode === 'json'" class="json-view">
        <div v-if="formattedQuery && showQuery" class="query-section">
          <div class="query-header">
            <h4>Query</h4>
            <button class="btn-copy" title="Copy query to clipboard" @click="copyToClipboard(query)">📋 Copy</button>
          </div>
          <pre class="sql-query-display"><code class="language-sql">{{ formattedQuery }}</code></pre>
        </div>
        <div class="view-header">
          <button class="btn-copy" title="Copy to clipboard" @click="copyToClipboard(formattedPlan)">📋 Copy</button>
        </div>
        <pre class="json-display" style="max-height: 75vh">{{ formattedPlan }}</pre>
      </div>

      <!-- Text view (plain) -->
      <div v-if="displayMode === 'text'" class="text-view">
        <div v-if="formattedQuery && showQuery" class="query-section">
          <div class="query-header">
            <h4>Query</h4>
            <button class="btn-copy" title="Copy query to clipboard" @click="copyToClipboard(query)">📋 Copy</button>
          </div>
          <pre class="sql-query-display"><code class="language-sql">{{ formattedQuery }}</code></pre>
        </div>
        <div class="view-header">
          <button class="btn-copy" title="Copy to clipboard" @click="copyToClipboard(explainPlan)">📋 Copy</button>
        </div>
        <pre class="text-display" style="max-height: 75vh">{{ textPlan }}</pre>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import SimpleQueryPlanViewer from "./SimpleQueryPlanViewer.vue";
import { formatExplainPlanAsText, formatSQL, applySyntaxHighlighting } from "@/utils/ui-utils";

export default defineComponent({
  name: "ExplainPlanFormatter",
  components: {
    SimpleQueryPlanViewer,
  },
  props: {
    explainPlan: {
      type: String,
      default: "",
    },
    query: {
      type: String,
      default: "",
    },
    defaultMode: {
      type: String,
      default: "visual", // 'visual', 'json', or 'text'
      validator: (value: string) => ["visual", "json", "text"].includes(value),
    },
    showQuery: {
      type: Boolean,
      default: true,
    },
  },
  data() {
    return {
      displayMode: this.defaultMode as string,
      error: null as string | null,
    };
  },
  computed: {
    formattedPlan(): string {
      if (!this.explainPlan) return "";
      try {
        const parsed = JSON.parse(this.explainPlan);
        return JSON.stringify(parsed, null, 2);
      } catch {
        // If not valid JSON, return as-is
        return this.explainPlan;
      }
    },

    textPlan(): string {
      if (!this.explainPlan) return "";
      try {
        const parsed = JSON.parse(this.explainPlan);
        return formatExplainPlanAsText(parsed);
      } catch {
        return this.explainPlan;
      }
    },

    hasValidPlan(): boolean {
      if (!this.explainPlan) return false;
      try {
        const parsed = JSON.parse(this.explainPlan);
        // Check if it's a valid PostgreSQL EXPLAIN plan format
        return Array.isArray(parsed) || (typeof parsed === "object" && (parsed.Plan || parsed["Execution Time"]));
      } catch {
        return false;
      }
    },

    formattedQuery(): string {
      if (!this.query) return "";
      return formatSQL(this.query);
    },
  },
  watch: {
    // Reset to default mode when plan changes
    explainPlan() {
      if (!this.hasValidPlan && this.displayMode === "visual") {
        this.displayMode = "json";
      }
    },
    // Re-apply syntax highlighting when switching tabs
    displayMode() {
      this.$nextTick(() => {
        applySyntaxHighlighting({ sqlSelector: ".sql-query-display code" });
      });
    },
    // Re-apply syntax highlighting when query changes
    formattedQuery() {
      this.$nextTick(() => {
        applySyntaxHighlighting({ sqlSelector: ".sql-query-display code" });
      });
    },
  },
  mounted() {
    // If visual mode is selected but plan is not valid, switch to JSON
    if (this.displayMode === "visual" && !this.hasValidPlan) {
      this.displayMode = "json";
    }
    // Apply syntax highlighting to SQL queries
    this.$nextTick(() => {
      applySyntaxHighlighting({ sqlSelector: ".sql-query-display code" });
    });
  },
  updated() {
    // Re-apply syntax highlighting when content changes
    this.$nextTick(() => {
      applySyntaxHighlighting({ sqlSelector: ".sql-query-display code" });
    });
  },
  methods: {
    copyToClipboard(text: string) {
      navigator.clipboard.writeText(text).then(
        () => {
          alert("Copied to clipboard!");
        },
        (err) => {
          console.error("Failed to copy:", err);
          alert("Failed to copy to clipboard");
        }
      );
    },
  },
});
</script>
