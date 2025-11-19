<template>
  <div class="explain-plan-formatter">
    <div v-if="error" class="alert alert-danger">
      {{ error }}
    </div>
    <div v-else-if="!explainPlan" class="text-muted">
      No EXPLAIN plan available.
    </div>
    <div v-else class="explain-plan-content">
      <!-- Display mode tabs -->
      <div class="display-mode-tabs">
        <button
          :class="['tab-button', { active: displayMode === 'visual' }]"
          @click="displayMode = 'visual'"
          :disabled="!hasValidPlan"
        >
          📊 Visual
        </button>
        <button
          :class="['tab-button', { active: displayMode === 'json' }]"
          @click="displayMode = 'json'"
        >
          📄 JSON
        </button>
        <button
          :class="['tab-button', { active: displayMode === 'text' }]"
          @click="displayMode = 'text'"
        >
          📝 Text
        </button>
      </div>

      <!-- Visual view using simple query plan viewer -->
      <div v-if="displayMode === 'visual' && hasValidPlan" class="visual-view">
        <SimpleQueryPlanViewer :plan-source="formattedPlan" />
      </div>

      <!-- JSON view -->
      <div v-if="displayMode === 'json'" class="json-view">
        <div class="view-header">
          <button @click="copyToClipboard(formattedPlan)" class="btn-copy" title="Copy to clipboard">
            📋 Copy
          </button>
        </div>
        <pre class="json-display">{{ formattedPlan }}</pre>
      </div>

      <!-- Text view (plain) -->
      <div v-if="displayMode === 'text'" class="text-view">
        <div class="view-header">
          <button @click="copyToClipboard(explainPlan)" class="btn-copy" title="Copy to clipboard">
            📋 Copy
          </button>
        </div>
        <pre class="text-display">{{ explainPlan }}</pre>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import SimpleQueryPlanViewer from "./SimpleQueryPlanViewer.vue";

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
      } catch (e) {
        // If not valid JSON, return as-is
        return this.explainPlan;
      }
    },

    hasValidPlan(): boolean {
      if (!this.explainPlan) return false;
      try {
        const parsed = JSON.parse(this.explainPlan);
        // Check if it's a valid PostgreSQL EXPLAIN plan format
        return Array.isArray(parsed) || (typeof parsed === "object" && (parsed.Plan || parsed["Execution Time"]));
      } catch (e) {
        return false;
      }
    },
  },
  watch: {
    // Reset to default mode when plan changes
    explainPlan() {
      if (!this.hasValidPlan && this.displayMode === "visual") {
        this.displayMode = "json";
      }
    },
  },
  mounted() {
    // If visual mode is selected but plan is not valid, switch to JSON
    if (this.displayMode === "visual" && !this.hasValidPlan) {
      this.displayMode = "json";
    }
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

<style scoped>
.explain-plan-formatter {
  background: #161b22;
  border: 1px solid #30363d;
  border-radius: 6px;
  overflow: hidden;
}

.explain-plan-content {
  display: flex;
  flex-direction: column;
  min-height: 300px;
}

.display-mode-tabs {
  display: flex;
  gap: 0.5rem;
  padding: 1rem 1rem 0 1rem;
  background: #0d1117;
  border-bottom: 1px solid #30363d;
}

.tab-button {
  background: transparent;
  border: none;
  color: #8b949e;
  padding: 0.5rem 1rem;
  border-radius: 6px 6px 0 0;
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 500;
  transition: all 0.2s;
  border-bottom: 2px solid transparent;
}

.tab-button:hover:not(:disabled) {
  color: #c9d1d9;
  background: rgba(110, 118, 129, 0.1);
}

.tab-button.active {
  color: #58a6ff;
  background: #161b22;
  border-bottom: 2px solid #58a6ff;
}

.tab-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.visual-view,
.json-view,
.text-view {
  padding: 1rem;
  flex: 1;
  overflow: auto;
}

.view-header {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 0.5rem;
}

.btn-copy {
  background: #21262d;
  border: 1px solid #30363d;
  color: #c9d1d9;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  cursor: pointer;
  transition: background 0.2s;
}

.btn-copy:hover {
  background: #30363d;
}

.json-display,
.text-display {
  background: #0d1117;
  border: 1px solid #30363d;
  border-radius: 4px;
  padding: 1rem;
  overflow-x: auto;
  color: #c9d1d9;
  font-family: "SF Mono", Monaco, "Courier New", monospace;
  font-size: 0.875rem;
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
  max-height: 500px;
}

.alert {
  padding: 1rem;
  margin: 1rem;
  border-radius: 4px;
}

.alert-danger {
  background: rgba(248, 81, 73, 0.1);
  border: 1px solid rgba(248, 81, 73, 0.4);
  color: #f85149;
}

.text-muted {
  color: #8b949e;
  padding: 1rem;
  text-align: center;
}

/* Visual view styling */
.visual-view {
  background: transparent;
}
</style>
