import { createApp } from "vue";
import App from "./App.vue";
import router from "./router";
import AppHeader from "./components/AppHeader.vue";
import TippyPlugin from "vue-tippy";
import "tippy.js/dist/tippy.css";

// Import global styles
import "../static/css/styles.css";
import "highlight.js/styles/github-dark.css";

// Set up highlight.js globally
import hljs from "highlight.js/lib/core";
import json from "highlight.js/lib/languages/json";
import sql from "highlight.js/lib/languages/sql";
import graphql from "highlight.js/lib/languages/graphql";

// Register languages
hljs.registerLanguage("json", json);
hljs.registerLanguage("sql", sql);
hljs.registerLanguage("graphql", graphql);

// Make hljs available globally
if (typeof window !== "undefined") {
  (window as any).hljs = hljs;
}

// Declare global types for external libraries
declare global {
  interface Window {
    Vue: any;
    hljs: any;
  }
}

// Create app instance
const app = createApp(App);

// Register global components
app.component("AppHeader", AppHeader);

// Use plugins
app.use(router);
app.use(TippyPlugin);

// Mount app
app.mount("#app");
