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
import { defineComponent } from "vue";
import type { PlanNodeType } from "@/types";
import PlanNodeItem from "./PlanNodeItem.vue";

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
</style>
