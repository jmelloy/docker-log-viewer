import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import AppHeader from './components/AppHeader.vue'

// Import global styles
import '../static/css/styles.css'
import '../static/lib/pev2.css'
import '../static/lib/highlight.css'
import '../static/css/codemirror-graphql.css'

// Create app instance
const app = createApp(App)

// Register global components
app.component('app-header', AppHeader)

// Use router
app.use(router)

// Make pev2 and hljs available globally (loaded via CDN in index.html)
declare global {
  interface Window {
    pev2: any
    hljs: any
  }
}

// Mount app
app.mount('#app')
