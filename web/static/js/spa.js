// SPA Main Application
import { loadTemplate } from "/static/js/shared/template-loader.js";

const { createApp } = Vue;

// Path-based router using History API
class Router {
  constructor() {
    this.currentPath = '';
    this.listeners = [];
    
    // Listen for popstate (back/forward button)
    window.addEventListener('popstate', () => this.handleRouteChange());
    
    // Handle initial route
    this.handleRouteChange();
  }
  
  handleRouteChange() {
    const path = window.location.pathname;
    if (path !== this.currentPath) {
      this.currentPath = path;
      this.notifyListeners(path);
    }
  }
  
  push(path) {
    if (path !== this.currentPath) {
      window.history.pushState({}, '', path);
      this.handleRouteChange();
    }
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
              <a href="/" :class="{ active: activePage === 'viewer' }" @click.prevent="navigate('/')">Log Viewer</a>
              <a href="/requests" :class="{ active: activePage === 'requests' }" @click.prevent="navigate('/requests')">Request Manager</a>
              <a href="/graphql" :class="{ active: activePage === 'graphql-explorer' }" @click.prevent="navigate('/graphql')">GraphQL Explorer</a>
              <a href="/settings" :class="{ active: activePage === 'settings' }" @click.prevent="navigate('/settings')">Settings</a>
            </nav>
          `,
          props: ['activePage'],
          methods: {
            navigate(path) {
              // Use the router's push method for navigation
              router.push(path);
            },
          },
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
