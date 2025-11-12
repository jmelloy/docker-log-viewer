import { createApp } from 'vue'
import App from './App.vue'
import router from './router'

// Import global styles
import '../static/css/styles.css'
import '../static/lib/pev2.css'
import '../static/lib/highlight.css'
import '../static/css/codemirror-graphql.css'

// Create app instance
const app = createApp(App)

// Use router
app.use(router)

// Mount app
app.mount('#app')
