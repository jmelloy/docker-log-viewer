import { createRouter, createWebHistory } from "vue-router";
import LogsView from "@/views/LogsView.vue";
import RequestsView from "@/views/RequestsView.vue";
import RequestDetailView from "@/views/RequestDetailView.vue";
import SqlDetailView from "@/views/SqlDetailView.vue";
import GraphQLExplorerView from "@/views/GraphQLExplorerView.vue";
import SettingsView from "@/views/SettingsView.vue";

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: "/",
      redirect: "/logs",
    },
    {
      path: "/logs",
      name: "logs",
      component: LogsView,
    },
    {
      path: "/requests",
      name: "requests",
      component: RequestsView,
    },
    {
      path: "/requests/:id",
      name: "request-detail",
      component: RequestDetailView,
    },
    {
      path: "/sql/:hash",
      name: "sql-detail",
      component: SqlDetailView,
    },
    {
      path: "/graphql-explorer",
      name: "graphql-explorer",
      component: GraphQLExplorerView,
    },
    {
      path: "/settings",
      name: "settings",
      component: SettingsView,
    },
  ],
});

export default router;
