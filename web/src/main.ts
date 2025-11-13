import { createApp } from "vue";
import App from "./App.vue";
import router from "./router";
import AppHeader from "./components/AppHeader.vue";

// Import global styles
import "../static/css/styles.css";
import "pev2/dist/pev2.css";
import "highlight.js/styles/github-dark.css";
import "../static/css/codemirror-graphql.css";

// Declare global types for external libraries
declare global {
  interface Window {
    Vue: any;
    pev2: any;
    hljs: any;
  }
}

// Create app instance
const app = createApp(App);

// Register global components
app.component("app-header", AppHeader);

// Use router
app.use(router);

// Mount app
app.mount("#app");
