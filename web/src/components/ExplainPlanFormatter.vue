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
          @click="displayMode = 'visual'"
          :disabled="!hasValidPlan"
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

      <!-- Visual view using PEV2 -->
      <div v-if="displayMode === 'visual' && hasValidPlan" class="visual-view">
        <pev2 :plan-source="formattedPlan" :plan-query="query || ''"></pev2>
      </div>

      <!-- JSON view -->
      <div v-if="displayMode === 'json'" class="json-view">
        <div class="view-header">
          <button @click="copyToClipboard(formattedPlan)" class="btn-copy" title="Copy to clipboard">📋 Copy</button>
        </div>
        <pre class="json-display">{{ formattedPlan }}</pre>
      </div>

      <!-- Text view (plain) -->
      <div v-if="displayMode === 'text'" class="text-view">
        <div class="view-header">
          <button @click="copyToClipboard(explainPlan)" class="btn-copy" title="Copy to clipboard">📋 Copy</button>
        </div>
        <pre class="text-display">{{ explainPlan }}</pre>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import { Plan } from "pev2";

export default defineComponent({
  name: "ExplainPlanFormatter",
  components: {
    pev2: Plan,
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
