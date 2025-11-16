<template>
  <div class="app-container">
    <app-header activePage="requests"></app-header>

    <!-- Tab Navigation -->
    <div class="tab-navigation">
      <button
        @click="setActiveTab('requests')"
        :class="['tab-btn', { active: activeTab === 'requests' }]"
      >
        Request Manager
      </button>
      <button
        @click="setActiveTab('graphql-explorer')"
        :class="['tab-btn', { active: activeTab === 'graphql-explorer' }]"
      >
        GraphQL Explorer
      </button>
    </div>

    <!-- Content -->
    <component :is="currentComponent" :key="activeTab" />
  </div>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import AppHeader from "@/components/AppHeader.vue";
import RequestsViewInner from "./RequestsViewInner.vue";
import GraphQLExplorerViewInner from "./GraphQLExplorerViewInner.vue";

export default defineComponent({
  name: "RequestManagerView",
  components: {
    AppHeader,
    RequestsViewInner,
    GraphQLExplorerViewInner,
  },
  data() {
    return {
      activeTab: "requests" as "requests" | "graphql-explorer",
    };
  },
  computed: {
    currentComponent() {
      return this.activeTab === "requests" ? "RequestsViewInner" : "GraphQLExplorerViewInner";
    },
  },
  mounted() {
    // Check URL hash or query parameter to determine initial tab
    const urlParams = new URLSearchParams(window.location.search);
    const tabParam = urlParams.get("tab");
    const hash = window.location.hash.replace("#", "");

    if (tabParam === "graphql-explorer" || tabParam === "explorer" || hash === "graphql-explorer" || hash === "explorer") {
      this.activeTab = "graphql-explorer";
    }
  },
  methods: {
    setActiveTab(tab: "requests" | "graphql-explorer") {
      this.activeTab = tab;
      // Update URL without reload
      const url = new URL(window.location.href);
      if (tab === "graphql-explorer") {
        url.searchParams.set("tab", "graphql-explorer");
      } else {
        url.searchParams.delete("tab");
      }
      window.history.pushState({}, "", url);
    },
  },
});
</script>

<style scoped>
.tab-navigation {
  display: flex;
  gap: 0.5rem;
  padding: 1rem 2rem;
  background: #0d1117;
  border-bottom: 1px solid #30363d;
}

.tab-btn {
  padding: 0.75rem 1.5rem;
  background: #161b22;
  border: 1px solid #30363d;
  border-radius: 6px;
  color: #c9d1d9;
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 500;
  transition: all 0.2s;
}

.tab-btn:hover {
  background: #21262d;
  border-color: #58a6ff;
}

.tab-btn.active {
  background: #1f6feb;
  border-color: #1f6feb;
  color: white;
}
</style>
