<template>
  <div class="plan-node" :style="{ marginLeft: level * 0.5 + 'rem' }">
    <div class="node-header">
      <span class="node-icon">{{ nodeIcon }}</span>
      <span class="node-type">{{ node["Node Type"] }}</span>
      <span v-if="costInfo" class="node-cost-info">{{ costInfo }}</span>
    </div>

    <span v-if="relationInfo" class="relation-info"> on {{ relationInfo }}</span>
    <div v-if="node['Filter']" class="node-filter-info">Filter: {{ node["Filter"] }}</div>
    <plan-node-item v-for="(child, index) in node.Plans" :key="index" :node="child" :level="level + 1" />
  </div>
</template>

<script lang="ts">
import { defineComponent, PropType } from "vue";
import type { PlanNodeType } from "@/types";

export default defineComponent({
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
      if (nodeType.includes("Seq Scan")) return "📋 ";
      if (nodeType.includes("Index")) return "📇 ";
      if (nodeType.includes("Hash")) return "🔨 ";
      if (nodeType.includes("Nested Loop")) return "🔄 ";
      if (nodeType.includes("Merge")) return "🔀 ";
      if (nodeType.includes("Sort")) return "⬇️ ";
      if (nodeType.includes("Aggregate")) return "∑ ";
      if (nodeType.includes("Limit")) return "✂️ ";
      if (nodeType.includes("Bitmap")) return "🔍 ";
      if (nodeType.includes("Gather")) return "👥 ";
      if (nodeType.includes("Gather Merge")) return "👥🔀 ";
      if (nodeType.includes("Subquery Scan")) return "🔍 ";
      return "▪️";
    },
    costInfo(): string {
      const parts = [];
      if (this.node["Total Cost"] !== undefined) {
        parts.push(`Cost: ${this.node["Total Cost"].toFixed(2)}`);
      }
      if (this.node["Actual Rows"] !== undefined) {
        parts.push(`Rows: ${this.node["Actual Rows"]}`);
      }
      if (this.node["Plan Rows"] !== undefined) {
        parts.push(`Est. Rows: ${this.node["Plan Rows"]}`);
      }
      return parts.join(" | ");
    },
    relationInfo(): string {
      const parts = [];
      if (this.node["Relation Name"]) {
        parts.push(this.node["Relation Name"]);
      }
      if (this.node["Index Name"]) {
        parts.push(` using ${this.node["Index Name"]}`);
      }
      return parts.join(" ");
    },
  },
});
</script>

<style scoped>
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

.node-cost-info {
  color: #8b949e;
  font-size: 0.8rem;
  font-style: italic;
}

.node-filter-info {
  color: #8b949e;
  font-size: 0.8rem;
}
</style>
