import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import path from "path";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  optimizeDeps: {
    include: ["graphql", "cm6-graphql", "highlight.js", "pev2"],
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    sourcemap: true,
    commonjsOptions: {
      include: [/node_modules/],
      transformMixedEsModules: true,
    },
  },
  server: {
    port: 5173,
    host: "0.0.0.0",
    proxy: {
      "/api": {
        target: process.env.API_URL || "http://localhost:9000",
        changeOrigin: true,
        ws: true, // Enable WebSocket proxying
      },
    },
  },
  publicDir: "static", // Copy static directory to dist
});
