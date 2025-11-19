<template>
  <div class="plan-node" :style="{ marginLeft: level * 0.5 + 'rem' }">
    <div class="node-header" :class="costClass">
      <span class="node-icon">{{ nodeIcon }}</span>
      <span class="node-type"
        >{{ node["Node Type"] }} {{ node["Subplan Name"] ? `(${node["Subplan Name"]})` : "" }}</span
      >
      <span v-if="costInfo" class="node-cost-info" :class="{ 'cost-highlight': isExpensive }">{{ costInfo }}</span>
    </div>

    <span v-if="relationInfo" class="relation-info">{{ relationInfo }}</span>
    <div v-if="node['Filter']" class="node-filter-info">Filter: {{ node["Filter"] }}</div>
    <div v-if="node['Rows Removed by Filter']" class="node-filter-info">
      Rows Removed by Filter: {{ node["Rows Removed by Filter"] }}
    </div>

    <plan-node-item
      v-for="(child, index) in node.Plans"
      :key="index"
      :node="child"
      :level="level + 1"
      :root-cost="rootCost"
    />
  </div>
</template>

<script lang="ts">
import { defineComponent, PropType } from "vue";
import type { PlanNodeType } from "@/types";
import { explainPlanLine } from "@/utils/ui-utils";

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
    rootCost: {
      type: Number as PropType<number | null>,
      default: null,
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
      return explainPlanLine(this.node);
    },
    relationInfo(): string {
      const parts = [];
      if (this.node["Index Cond"]) {
        parts.push(`Index Cond: ${this.node["Index Cond"]}`);
      }
      if (this.node["Hash Cond"]) {
        parts.push(`Hash Cond: ${this.node["Hash Cond"]}`);
      }
      return parts.join(" ");
    },
    isExpensive(): boolean {
      const nodeCost = this.node["Total Cost"];
      if (nodeCost === undefined || this.rootCost === null || this.rootCost < 100) {
        return false;
      }
      // Consider expensive if cost is > 30% of root cost
      return nodeCost > this.rootCost * 0.3 || nodeCost > 1000;
    },
    isVeryExpensive(): boolean {
      const nodeCost = this.node["Total Cost"];
      if (nodeCost === undefined || this.rootCost === null || this.rootCost < 100) {
        return false;
      }
      // Consider very expensive if cost is > 60% of root cost
      return nodeCost > this.rootCost * 0.6 || nodeCost > 10000;
    },
    costClass(): Record<string, boolean> {
      if (this.isVeryExpensive) {
        return { "cost-very-expensive": true };
      }
      if (this.isExpensive) {
        return { "cost-expensive": true };
      }
      return {};
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
  transition: all 0.2s ease;
}

.node-header.cost-expensive {
  border-left-color: #f0883e;
  background: rgba(240, 136, 62, 0.1);
}

.node-header.cost-very-expensive {
  border-left-color: #f85149;
  background: rgba(248, 81, 73, 0.15);
  box-shadow: 0 0 8px rgba(248, 81, 73, 0.2);
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
  transition: color 0.2s ease;
}

.node-cost-info.cost-highlight {
  color: #f0883e;
  font-weight: 600;
}

.node-filter-info {
  color: #8b949e;
  font-size: 0.8rem;
}
</style>
