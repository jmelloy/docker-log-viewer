/// <reference types="vite/client" />

declare module "*.vue" {
  import type { DefineComponent } from "vue";
  const component: DefineComponent<{}, {}, any>;
  export default component;
}

// Declare global libraries loaded via script tags
declare global {
  const Vue: any;
  const hljs: any;
}

export {};
