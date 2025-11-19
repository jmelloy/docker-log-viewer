<template>
  <div class="query-plan-viewer">
    <div v-if="error" class="alert alert-danger">
      {{ error }}
    </div>
    <div v-else-if="!plan" class="text-muted">No query plan available.</div>
    <div v-else class="plan-tree">
      <plan-node-item :node="plan" :level="0" />
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent, PropType } from "vue";

interface PlanNodeType {
  "Node Type": string;
  "Relation Name"?: string;
  "Startup Cost"?: number;
  "Total Cost"?: number;
  "Plan Rows"?: number;
  "Plan Width"?: number;
  "Actual Rows"?: number;
  "Actual Loops"?: number;
  "Index Name"?: string;
  "Scan Direction"?: string;
  Plans?: PlanNodeType[];
  [key: string]: any;
}

// Recursive component for rendering plan nodes
const PlanNodeItem = defineComponent({
  name: "PlanNodeItem",
  props: {
    node: {
      type: Object as PropType<PlanNodeType>,
      required: true,
    },
    level: {
      type: Number,
      required: true,
    },
  },
  computed: {
    nodeIcon(): string {
      const nodeType = this.node["Node Type"] || "";
      if (nodeType.includes("Seq Scan")) return "📋";
      if (nodeType.includes("Index")) return "📇";
      if (nodeType.includes("Hash")) return "🔨";
      if (nodeType.includes("Nested Loop")) return "🔄";
      if (nodeType.includes("Merge")) return "🔀";
      if (nodeType.includes("Sort")) return "⬇️";
      if (nodeType.includes("Aggregate")) return "∑";
      if (nodeType.includes("Limit")) return "✂️";
      return "▪️";
    },
    costInfo(): string {
      const parts = [];
      if (this.node["Total Cost"] !== undefined) {
        parts.push(`Cost: ${this.node["Total Cost"].toFixed(2)}`);
      }
      if (this.node["Actual Rows"] !== undefined) {
        parts.push(`Rows: ${this.node["Actual Rows"]}`);
      } else if (this.node["Plan Rows"] !== undefined) {
        parts.push(`Rows: ${this.node["Plan Rows"]}`);
      }
      return parts.join(" | ");
    },
    relationInfo(): string {
      const parts = [];
      if (this.node["Relation Name"]) {
        parts.push(this.node["Relation Name"]);
      }
      if (this.node["Index Name"]) {
        parts.push(`using ${this.node["Index Name"]}`);
      }
      return parts.join(" ");
    },
  },
  template: `
    <div class="plan-node" :style="{ marginLeft: level * 20 + 'px' }">
      <div class="node-header">
        <span class="node-icon">{{ nodeIcon }}</span>
        <span class="node-type">{{ node["Node Type"] }}</span>
        <span v-if="relationInfo" class="relation-info">on {{ relationInfo }}</span>
      </div>
      <div v-if="costInfo" class="node-details">{{ costInfo }}</div>
      <plan-node-item
        v-for="(child, index) in node.Plans"
        :key="index"
        :node="child"
        :level="level + 1"
      />
    </div>
  `,
});

export default defineComponent({
  name: "SimpleQueryPlanViewer",
  components: {
    PlanNodeItem,
  },
  props: {
    planSource: {
      type: String,
      required: true,
    },
  },
  data() {
    return {
      plan: null as PlanNodeType | null,
      error: null as string | null,
    };
  },
  watch: {
    planSource: {
      immediate: true,
      handler(newPlan: string) {
        this.parsePlan(newPlan);
      },
    },
  },
  methods: {
    parsePlan(planJson: string) {
      if (!planJson) {
        this.plan = null;
        this.error = null;
        return;
      }

      try {
        const parsed = JSON.parse(planJson);
        // PostgreSQL EXPLAIN returns an array with a Plan property
        if (Array.isArray(parsed) && parsed.length > 0 && parsed[0].Plan) {
          this.plan = parsed[0].Plan;
          this.error = null;
        } else if (parsed.Plan) {
          this.plan = parsed.Plan;
          this.error = null;
        } else {
          this.error = "Invalid query plan format";
          this.plan = null;
        }
      } catch (e) {
        this.error = `Failed to parse plan: ${e.message}`;
        this.plan = null;
      }
    },
  },
});
</script>

<style scoped>
.query-plan-viewer {
  font-family: "SF Mono", Monaco, "Courier New", monospace;
  font-size: 0.875rem;
  color: #c9d1d9;
  padding: 1rem;
  background: #0d1117;
  border-radius: 6px;
  overflow-x: auto;
}

.alert {
  padding: 1rem;
  margin-bottom: 1rem;
  border-radius: 4px;
}

.alert-danger {
  background: rgba(248, 81, 73, 0.1);
  border: 1px solid rgba(248, 81, 73, 0.4);
  color: #f85149;
}

.text-muted {
  color: #8b949e;
  text-align: center;
  padding: 2rem;
}

.plan-tree {
  padding: 0.5rem 0;
}

.plan-node {
  margin-bottom: 0.5rem;
}

.node-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem;
  background: #161b22;
  border-left: 3px solid #58a6ff;
  border-radius: 4px;
  font-weight: 500;
}

.node-icon {
  font-size: 1rem;
  flex-shrink: 0;
}

.node-type {
  color: #58a6ff;
  font-weight: 600;
}

.relation-info {
  color: #8b949e;
  font-style: italic;
  font-size: 0.85rem;
}

.node-details {
  margin-left: 2rem;
  padding: 0.25rem 0.5rem;
  color: #8b949e;
  font-size: 0.8rem;
}
</style>
