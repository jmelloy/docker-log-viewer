import { createRouter, createWebHistory } from "vue-router";
import LogsView from "@/views/LogsView.vue";
import RequestManagerView from "@/views/RequestManagerView.vue";
import RequestDetailView from "@/views/RequestDetailView.vue";
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
      component: RequestManagerView,
    },
    {
      path: "/requests/:id",
      name: "request-detail",
      component: RequestDetailView,
    },
    {
      path: "/graphql-explorer",
      redirect: "/requests?tab=graphql-explorer",
    },
    {
      path: "/settings",
      name: "settings",
      component: SettingsView,
    },
  ],
});

export default router;
