/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}

// Declare global libraries
declare global {
  const Vue: typeof import('vue')
  const pev2: any
  const hljs: any
}

export {}
