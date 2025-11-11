// SPA Main Application
import { loadTemplate } from "/static/js/shared/template-loader.js";

const { createApp } = Vue;

// Simple hash-based router
class Router {
  constructor() {
    this.currentPath = '';
    this.listeners = [];
    
    // If there's no hash but the pathname indicates a specific page, set the hash
    if (!window.location.hash && window.location.pathname !== '/' && window.location.pathname !== '/index.html') {
      const pathname = window.location.pathname;
      if (pathname.startsWith('/logs')) {
        window.location.hash = '/';
      } else if (pathname.startsWith('/requests')) {
        window.location.hash = '/requests';
      } else if (pathname.startsWith('/graphql')) {
        window.location.hash = '/graphql';
      } else if (pathname.startsWith('/settings')) {
        window.location.hash = '/settings';
      }
    }
    
    window.addEventListener('hashchange', () => this.handleRouteChange());
    this.handleRouteChange();
  }
  
  handleRouteChange() {
    const hash = window.location.hash.slice(1) || '/';
    if (hash !== this.currentPath) {
      this.currentPath = hash;
      this.notifyListeners(hash);
    }
  }
  
  push(path) {
    window.location.hash = path;
  }
  
  onChange(callback) {
    this.listeners.push(callback);
    // Call immediately with current path
    callback(this.currentPath);
  }
  
  notifyListeners(path) {
    this.listeners.forEach(cb => cb(path));
  }
}

const router = new Router();

// Load navigation template
const navigationTemplate = await loadTemplate("/static/js/shared/navigation-header-template.html");

// Main SPA app
const app = createApp({
  data() {
    return {
      currentPath: router.currentPath || '/',
      currentPageComponent: null,
      isLoading: false,
    };
  },
  
  computed: {
    activePage() {
      if (this.currentPath === '/' || this.currentPath.startsWith('/logs')) {
        return 'viewer';
      } else if (this.currentPath.startsWith('/requests')) {
        return 'requests';
      } else if (this.currentPath.startsWith('/graphql')) {
        return 'graphql-explorer';
      } else if (this.currentPath.startsWith('/settings')) {
        return 'settings';
      }
      return 'viewer';
    },
  },
  
  async mounted() {
    router.onChange(async (path) => {
      this.currentPath = path;
      await this.loadPage(path);
    });
  },
  
  methods: {
    async loadPage(path) {
      this.isLoading = true;
      
      try {
        // Determine which page to load based on path
        let pageModule, template, pageName;
        
        if (path === '/' || path.startsWith('/logs')) {
          template = await loadTemplate("/logs/template.html");
          pageModule = await import('/logs/app-component.js');
          pageName = 'viewer';
        } else if (path.startsWith('/requests')) {
          template = await loadTemplate("/requests/template.html");
          pageModule = await import('/requests/requests-component.js');
          pageName = 'requests';
        } else if (path.startsWith('/graphql')) {
          template = await loadTemplate("/graphql-explorer/template.html");
          pageModule = await import('/graphql-explorer/graphql-component.js');
          pageName = 'graphql-explorer';
        } else if (path.startsWith('/settings')) {
          template = await loadTemplate("/settings/template.html");
          pageModule = await import('/settings/settings-component.js');
          pageName = 'settings';
        } else {
          // Default to logs page
          template = await loadTemplate("/logs/template.html");
          pageModule = await import('/logs/app-component.js');
          pageName = 'viewer';
        }
        
        // Create the page component with the loaded template and module
        this.currentPageComponent = {
          template,
          ...pageModule.default,
        };
        
      } catch (error) {
        console.error('Failed to load page:', error);
      } finally {
        this.isLoading = false;
      }
    },
  },
  
  components: {
    'app-header': {
      template: navigationTemplate,
      props: ['activePage'],
      components: {
        'app-nav': {
          template: `
            <nav class="app-nav">
              <a href="#/" :class="{ active: activePage === 'viewer' }">Log Viewer</a>
              <a href="#/requests" :class="{ active: activePage === 'requests' }">Request Manager</a>
              <a href="#/graphql" :class="{ active: activePage === 'graphql-explorer' }">GraphQL Explorer</a>
              <a href="#/settings" :class="{ active: activePage === 'settings' }">Settings</a>
            </nav>
          `,
          props: ['activePage'],
        },
      },
    },
  },
  
  template: `
    <div>
      <app-header :active-page="activePage" />
      <div v-if="isLoading" class="loading-container">
        <div class="spinner-border text-primary" role="status">
          <span class="visually-hidden">Loading...</span>
        </div>
      </div>
      <component v-else-if="currentPageComponent" :is="currentPageComponent" :key="currentPath" />
    </div>
  `,
});

// Register pev2 globally for all pages
app.component("pev2", pev2.Plan);

app.mount('#app');
