export function createNavigation(activePage) {
  return {
    template: `
      <nav class="app-nav">
        <a href="/" :class="{ active: activePage === 'viewer' }">Log Viewer</a>
        <a href="/requests.html" :class="{ active: activePage === 'requests' }">Request Manager</a>
        <a href="/graphql-explorer.html" :class="{ active: activePage === 'graphql-explorer' }">GraphQL Explorer</a>
        <a href="/settings.html" :class="{ active: activePage === 'settings' }">Settings</a>
      </nav>
    `,
    data() {
      return {
        activePage,
      };
    },
  };
}

export function createAppHeader(activePage) {
  return {
    template: `
      <header class="app-header">
        <div class="app-header-content">
          <h1>🔱 Logseidon</h1>
          <app-nav></app-nav>
          <slot></slot>
        </div>
      </header>
    `,
    components: {
      "app-nav": createNavigation(activePage),
    },
  };
}
