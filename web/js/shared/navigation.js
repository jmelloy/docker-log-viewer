// Shared navigation component for all pages
// Returns a Vue component configuration object

/**
 * Creates a navigation component with the specified active page
 * @param {string} activePage - The currently active page: 'viewer', 'requests', 'settings', or 'request-detail'
 * @returns {Object} Vue component configuration
 */
export function createNavigation(activePage) {
  return {
    template: `
      <nav style="display: flex; gap: 1rem; align-items: center;">
        <a href="/" :class="{ active: activePage === 'viewer' }">Log Viewer</a>
        <a href="/requests.html" :class="{ active: activePage === 'requests' }">Request Manager</a>
        <a href="/graphql-explorer.html" :class="{ active: activePage === 'graphql-explorer' }">GraphQL Explorer</a>
        <a href="/settings.html" :class="{ active: activePage === 'settings' }">Settings</a>
      </nav>
    `,
    data() {
      return {
        activePage
      };
    }
  };
}

/**
 * Creates a header component with navigation and optional controls
 * @param {string} activePage - The currently active page
 * @param {Object} options - Optional header controls configuration
 * @param {boolean} options.showSearch - Whether to show search box
 * @param {Object} options.searchProps - Props for search box (v-model, placeholder)
 * @param {Object} options.traceFilters - Trace filters configuration
 * @returns {Object} Vue component configuration
 */
export function createHeader(activePage, options = {}) {
  const { showSearch = false, searchProps = {}, traceFilters = {} } = options;
  
  return {
    template: `
      <header class="app-header">
        <div style="display: flex; align-items: center; gap: 1rem; width: 100%;">
          <h1 style="margin: 0">🔱 Logseidon</h1>
          <app-nav></app-nav>
          <div v-if="showSearch" class="header-controls">
            <div class="search-box">
              <input type="text" v-model="searchQuery" :placeholder="searchPlaceholder">
              <button @click="$emit('clear-search')" class="clear-btn" title="Clear search">✕</button>
            </div>
            <div class="trace-filter-display" v-if="hasTraceFilters">
              <span v-for="([key, value], index) in Array.from(traceFilters.entries())" :key="key" class="trace-filter-badge">
                <span class="filter-key">{{ key }}</span>=<span class="filter-value">{{ value }}</span>
                <button @click="$emit('remove-trace-filter', key)" class="filter-remove" title="Remove filter">×</button>
              </span>
              <button @click="$emit('save-trace')" class="btn-star" title="Save trace to request manager">⭐</button>
              <button @click="$emit('clear-trace-filters')" class="clear-btn" title="Clear all filters">✕</button>
            </div>
          </div>
        </div>
      </header>
    `,
    components: {
      'app-nav': createNavigation(activePage)
    },
    props: {
      searchQuery: String,
      searchPlaceholder: {
        type: String,
        default: 'Search...'
      },
      traceFilters: {
        type: Map,
        default: () => new Map()
      }
    },
    computed: {
      showSearch() {
        return showSearch;
      },
      hasTraceFilters() {
        return this.traceFilters && this.traceFilters.size > 0;
      }
    }
  };
}
