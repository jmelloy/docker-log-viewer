class RequestManager {
  constructor() {
    this.sampleQueries = [];
    this.servers = [];
    this.selectedSampleQuery = null;
    this.requests = [];
    this.allRequests = [];
    this.init();
  }

  async init() {
    this.setupEventListeners();
    await this.loadServers();
    await this.loadSampleQueries();
    await this.loadAllRequests();
  }

  setupEventListeners() {
    // New sample query modal
    document.getElementById('newSampleQueryBtn').addEventListener('click', () => {
      this.showNewSampleQueryModal();
    });

    document.getElementById('closeNewSampleQueryModal').addEventListener('click', () => {
      this.hideNewSampleQueryModal();
    });

    document.getElementById('cancelNewSampleQueryBtn').addEventListener('click', () => {
      this.hideNewSampleQueryModal();
    });

    document.getElementById('saveSampleQueryBtn').addEventListener('click', () => {
      this.saveNewSampleQuery();
    });

    // Server selection change
    document.getElementById('newSampleQueryServer').addEventListener('change', (e) => {
      this.handleServerChange(e.target.value);
    });

    // Execute query modal
    document.getElementById('closeExecuteQueryModal').addEventListener('click', () => {
      this.hideExecuteQueryModal();
    });

    document.getElementById('cancelExecuteBtn').addEventListener('click', () => {
      this.hideExecuteQueryModal();
    });

    document.getElementById('confirmExecuteBtn').addEventListener('click', () => {
      this.confirmExecuteQuery();
    });

    // Request detail modal
    document.getElementById('closeRequestDetailModal').addEventListener('click', () => {
      this.hideRequestDetailModal();
    });

    // Sample query actions
    document.getElementById('executeBtn').addEventListener('click', () => {
      this.showExecuteQueryModal();
    });

    document.getElementById('deleteBtn').addEventListener('click', () => {
      this.deleteSampleQuery();
    });
  }

  async loadServers() {
    try {
      const response = await fetch('/api/servers');
      this.servers = await response.json();
    } catch (error) {
      console.error('Failed to load servers:', error);
      this.servers = [];
    }
  }

  async loadSampleQueries() {
    try {
      const response = await fetch('/api/requests');
      this.sampleQueries = await response.json();
      this.renderSampleQueriesList();
    } catch (error) {
      console.error('Failed to load sample queries:', error);
    }
  }

  async loadAllRequests() {
    try {
      const response = await fetch('/api/all-executions');
      this.allRequests = await response.json();
      this.renderAllRequestsList();
    } catch (error) {
      console.error('Failed to load requests:', error);
    }
  }

  renderSampleQueriesList() {
    const container = document.getElementById('sampleQueriesList');
    
    if (this.sampleQueries.length === 0) {
      container.innerHTML = '<p style="padding: 1rem; color: #6c757d; text-align: center;">No sample queries</p>';
      return;
    }

    container.innerHTML = this.sampleQueries.map(sq => {
      // Extract operation name from requestData
      let displayName = sq.name;
      try {
        const data = JSON.parse(sq.requestData);
        if (data.operationName) {
          displayName = data.operationName;
        }
      } catch (e) {
        // Use the original name if parsing fails
      }

      return `
        <div class="request-item ${this.selectedSampleQuery?.id === sq.id ? 'active' : ''}" 
             data-id="${sq.id}">
          <div class="request-item-name">${this.escapeHtml(displayName)}</div>
          <div class="request-item-meta">
            <span>${new Date(sq.createdAt).toLocaleDateString()}</span>
          </div>
        </div>
      `;
    }).join('');

    // Add click handlers
    container.querySelectorAll('.request-item').forEach(item => {
      item.addEventListener('click', () => {
        const id = parseInt(item.dataset.id);
        this.selectSampleQuery(id);
      });
    });
  }

  renderAllRequestsList() {
    const container = document.getElementById('requestsList');
    
    if (this.allRequests.length === 0) {
      container.innerHTML = '<p style="color: #6c757d;">No requests executed yet.</p>';
      return;
    }

    container.innerHTML = this.allRequests.map(req => {
      const statusClass = req.statusCode >= 200 && req.statusCode < 300 ? 'success' : 'error';
      const time = new Date(req.executedAt);
      const timeStr = time.toLocaleTimeString();
      
      return `
        <div class="execution-item-compact" data-id="${req.id}">
          <span class="exec-status ${statusClass}">${req.statusCode || 'ERR'}</span>
          <span class="exec-time">${timeStr}</span>
          <span class="exec-duration">${req.durationMs}ms</span>
          <span class="exec-id">req_id=${req.requestIdHeader}</span>
        </div>
      `;
    }).join('');

    // Add click handlers
    container.querySelectorAll('.execution-item-compact').forEach(item => {
      item.addEventListener('click', () => {
        const id = parseInt(item.dataset.id);
        this.showRequestDetail(id);
      });
    });
  }

  async selectSampleQuery(id) {
    const sampleQuery = this.sampleQueries.find(sq => sq.id === id);
    if (!sampleQuery) return;

    this.selectedSampleQuery = sampleQuery;
    this.renderSampleQueriesList();
    this.showSampleQueryDetail(sampleQuery);
    await this.loadRequests(id);
  }

  showSampleQueryDetail(sampleQuery) {
    document.getElementById('emptyState').classList.add('hidden');
    document.getElementById('sampleQueryDetail').classList.remove('hidden');

    document.getElementById('sampleQueryName').textContent = sampleQuery.name;
    
    // Show server info if available
    if (sampleQuery.server) {
      document.getElementById('sampleQueryURL').textContent = sampleQuery.server.url;
    } else {
      document.getElementById('sampleQueryURL').textContent = '(no server configured)';
    }
    
    document.getElementById('sampleQueryCreated').textContent = new Date(sampleQuery.createdAt).toLocaleString();
    
    // Pretty print JSON and extract variables
    try {
      const data = JSON.parse(sampleQuery.requestData);
      document.getElementById('sampleQueryData').textContent = JSON.stringify(data, null, 2);
      
      // Show variables if present
      if (data.variables) {
        document.getElementById('variablesSection').style.display = 'block';
        document.getElementById('sampleQueryVariables').textContent = JSON.stringify(data.variables, null, 2);
      } else {
        document.getElementById('variablesSection').style.display = 'none';
      }
    } catch (e) {
      document.getElementById('sampleQueryData').textContent = sampleQuery.requestData;
      document.getElementById('variablesSection').style.display = 'none';
    }
  }

  async loadRequests(sampleQueryId) {
    try {
      const response = await fetch(`/api/executions?request_id=${sampleQueryId}`);
      this.requests = await response.json();
      this.renderRequestsForSampleQuery();
    } catch (error) {
      console.error('Failed to load requests for sample query:', error);
      this.requests = [];
      this.renderRequestsForSampleQuery();
    }
  }

  renderRequestsForSampleQuery() {
    const container = document.getElementById('pastRequestsList');
    
    if (this.requests.length === 0) {
      container.innerHTML = '<p style="color: #6c757d;">No requests yet. Click "Execute Query" to run this query.</p>';
      return;
    }

    container.innerHTML = this.requests.map(req => {
      const statusClass = req.statusCode >= 200 && req.statusCode < 300 ? 'success' : 'error';
      return `
        <div class="execution-item" data-id="${req.id}">
          <div class="execution-status ${statusClass}">${req.statusCode || 'ERR'}</div>
          <div class="execution-info">
            <div class="execution-time">${new Date(req.executedAt).toLocaleString()}</div>
            <div class="execution-stats">
              <span>ID: ${req.requestIdHeader}</span>
            </div>
          </div>
          <div class="execution-duration">${req.durationMs}ms</div>
          <div>→</div>
        </div>
      `;
    }).join('');

    // Add click handlers
    container.querySelectorAll('.execution-item').forEach(item => {
      item.addEventListener('click', () => {
        const id = parseInt(item.dataset.id);
        this.showRequestDetail(id);
      });
    });
  }

  async showRequestDetail(requestId) {
    try {
      const response = await fetch(`/api/executions/${requestId}`);
      const detail = await response.json();
      this.renderRequestDetail(detail);
    } catch (error) {
      console.error('Failed to load request detail:', error);
    }
  }

  renderRequestDetail(detail) {
    const modal = document.getElementById('requestDetailModal');
    
    // Overview stats
    const statusClass = detail.execution.statusCode >= 200 && detail.execution.statusCode < 300 ? 'success' : 'error';
    document.getElementById('reqStatusCode').textContent = detail.execution.statusCode || 'Error';
    document.getElementById('reqStatusCode').className = `stat-value ${statusClass}`;
    document.getElementById('reqDuration').textContent = `${detail.execution.durationMs}ms`;
    document.getElementById('reqRequestID').textContent = detail.execution.requestIdHeader;
    document.getElementById('reqTime').textContent = new Date(detail.execution.executedAt).toLocaleString();

    // Response
    if (detail.execution.responseBody) {
      try {
        const data = JSON.parse(detail.execution.responseBody);
        document.getElementById('reqResponse').textContent = JSON.stringify(data, null, 2);
      } catch (e) {
        document.getElementById('reqResponse').textContent = detail.execution.responseBody;
      }
    } else {
      document.getElementById('reqResponse').textContent = '(no response)';
    }

    // Error
    if (detail.execution.error) {
      document.getElementById('reqErrorSection').style.display = 'block';
      document.getElementById('reqError').textContent = detail.execution.error;
    } else {
      document.getElementById('reqErrorSection').style.display = 'none';
    }

    // SQL Analysis
    if (detail.sqlAnalysis && detail.sqlAnalysis.totalQueries > 0) {
      document.getElementById('reqSQLSection').style.display = 'block';
      document.getElementById('sqlTotalQueries').textContent = detail.sqlAnalysis.totalQueries;
      document.getElementById('sqlUniqueQueries').textContent = detail.sqlAnalysis.uniqueQueries;
      document.getElementById('sqlAvgDuration').textContent = `${detail.sqlAnalysis.avgDuration.toFixed(2)}ms`;
      document.getElementById('sqlTotalDuration').textContent = `${detail.sqlAnalysis.totalDuration.toFixed(2)}ms`;

      // Render SQL queries
      const sqlContainer = document.getElementById('sqlQueriesList');
      sqlContainer.innerHTML = detail.sqlQueries.map((q, idx) => {
        const hasPlan = q.explainPlan && q.explainPlan.length > 0;
        return `
          <div class="sql-query-item">
            <div class="sql-query-header">
              <span>${q.tableName || 'unknown'} - ${q.operation || 'SELECT'}</span>
              <span class="sql-query-duration">${q.durationMs.toFixed(2)}ms</span>
            </div>
            <div class="sql-query-text">${this.escapeHtml(q.query)}</div>
            <div class="query-actions">
              <button class="btn-explain" data-query-idx="${idx}">Run EXPLAIN</button>
              ${hasPlan ? `<button class="btn-explain btn-secondary" data-query-idx="${idx}" data-show-plan="true">View Saved Plan</button>` : ''}
            </div>
          </div>
        `;
      }).join('');

      // Add EXPLAIN click handlers
      sqlContainer.querySelectorAll('.btn-explain').forEach(btn => {
        btn.addEventListener('click', (e) => {
          const idx = parseInt(e.target.dataset.queryIdx);
          const query = detail.sqlQueries[idx];
          
          if (e.target.dataset.showPlan === 'true') {
            // Show saved plan
            try {
              const plan = JSON.parse(query.explainPlan);
              console.log('Saved EXPLAIN Plan:', plan);
              alert('Saved EXPLAIN plan logged to console. Check browser console for details.');
            } catch (err) {
              alert('Error parsing saved plan: ' + err.message);
            }
          } else {
            // Run new EXPLAIN
            const variables = query.variables ? JSON.parse(query.variables) : {};
            this.runExplain(query.query, variables);
          }
        });
      });
    } else {
      document.getElementById('reqSQLSection').style.display = 'none';
    }

    // Logs
    document.getElementById('logsCount').textContent = detail.logs.length;
    const logsContainer = document.getElementById('reqLogs');
    
    if (detail.logs.length === 0) {
      logsContainer.innerHTML = '<p style="color: #6c757d;">No logs captured</p>';
    } else {
      logsContainer.innerHTML = detail.logs.map(log => `
        <div class="log-entry">
          <div class="log-entry-header">
            <span class="log-level ${log.level || 'INFO'}">${log.level || 'INFO'}</span>
            <span class="log-timestamp">${new Date(log.timestamp).toLocaleTimeString()}</span>
          </div>
          <div class="log-message">${this.escapeHtml(log.message || log.rawLog)}</div>
        </div>
      `).join('');
    }

    modal.classList.remove('hidden');
  }

  hideRequestDetailModal() {
    document.getElementById('requestDetailModal').classList.add('hidden');
  }

  showNewSampleQueryModal() {
    // Reset form
    document.getElementById('newSampleQueryName').value = '';
    document.getElementById('newSampleQueryURL').value = '';
    document.getElementById('newSampleQueryData').value = '';
    document.getElementById('newSampleQueryToken').value = '';
    document.getElementById('newSampleQueryDevID').value = '';
    
    // Populate server dropdown
    const serverSelect = document.getElementById('newSampleQueryServer');
    serverSelect.innerHTML = '<option value="">-- New Server --</option>';
    this.servers.forEach(server => {
      const option = document.createElement('option');
      option.value = server.id;
      option.textContent = `${server.name} (${server.url})`;
      serverSelect.appendChild(option);
    });
    
    // Reset to new server mode
    serverSelect.value = '';
    this.handleServerChange('');
    
    document.getElementById('newSampleQueryModal').classList.remove('hidden');
  }

  handleServerChange(serverId) {
    const newServerFields = document.getElementById('newServerFields');
    if (serverId === '') {
      // New server mode - show URL/token/devID fields
      newServerFields.style.display = 'block';
      document.getElementById('newSampleQueryURL').required = true;
    } else {
      // Existing server mode - hide URL/token/devID fields
      newServerFields.style.display = 'none';
      document.getElementById('newSampleQueryURL').required = false;
    }
  }

  hideNewSampleQueryModal() {
    document.getElementById('newSampleQueryModal').classList.add('hidden');
  }

  async saveNewSampleQuery() {
    const name = document.getElementById('newSampleQueryName').value.trim();
    const requestData = document.getElementById('newSampleQueryData').value.trim();
    const serverSelect = document.getElementById('newSampleQueryServer');
    const serverId = serverSelect.value;

    if (!name || !requestData) {
      alert('Please fill in all required fields');
      return;
    }

    // Validate JSON
    try {
      JSON.parse(requestData);
    } catch (e) {
      alert('Invalid JSON in query data');
      return;
    }

    // Build request payload
    const payload = {
      name,
      requestData,
    };

    if (serverId) {
      // Use existing server
      payload.serverId = parseInt(serverId);
    } else {
      // Create new server with provided details
      const url = document.getElementById('newSampleQueryURL').value.trim();
      const bearerToken = document.getElementById('newSampleQueryToken').value.trim();
      const devId = document.getElementById('newSampleQueryDevID').value.trim();

      if (!url) {
        alert('Please provide a URL for the new server');
        return;
      }

      payload.url = url;
      if (bearerToken) payload.bearerToken = bearerToken;
      if (devId) payload.devId = devId;
    }

    try {
      const response = await fetch('/api/requests', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        throw new Error('Failed to save sample query');
      }

      this.hideNewSampleQueryModal();
      await this.loadServers(); // Reload servers in case a new one was created
      await this.loadSampleQueries();
    } catch (error) {
      console.error('Failed to save sample query:', error);
      alert('Failed to save sample query: ' + error.message);
    }
  }

  showExecuteQueryModal() {
    if (!this.selectedSampleQuery) return;

    // Populate server dropdown
    const serverSelect = document.getElementById('executeServer');
    serverSelect.innerHTML = '<option value="">-- Select Server --</option>';
    this.servers.forEach(server => {
      const option = document.createElement('option');
      option.value = server.id;
      option.textContent = `${server.name} (${server.url})`;
      serverSelect.appendChild(option);
    });

    // Pre-select the sample query's server if available
    if (this.selectedSampleQuery.server) {
      serverSelect.value = this.selectedSampleQuery.server.id;
    }

    // Clear override fields
    document.getElementById('executeToken').value = '';
    document.getElementById('executeDevID').value = '';

    document.getElementById('executeQueryModal').classList.remove('hidden');
  }

  hideExecuteQueryModal() {
    document.getElementById('executeQueryModal').classList.add('hidden');
  }

  async confirmExecuteQuery() {
    if (!this.selectedSampleQuery) return;

    const serverSelect = document.getElementById('executeServer');
    const serverId = serverSelect.value;
    const tokenOverride = document.getElementById('executeToken').value.trim();
    const devIdOverride = document.getElementById('executeDevID').value.trim();

    if (!serverId) {
      alert('Please select a server');
      return;
    }

    const payload = {
      serverId: parseInt(serverId),
    };

    if (tokenOverride) {
      payload.bearerTokenOverride = tokenOverride;
    }
    if (devIdOverride) {
      payload.devIdOverride = devIdOverride;
    }

    try {
      const response = await fetch(`/api/requests/${this.selectedSampleQuery.id}/execute`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        throw new Error('Failed to execute query');
      }

      this.hideExecuteQueryModal();
      alert('Query execution started. Results will appear in the requests list.');
      
      // Reload requests after a delay
      setTimeout(() => {
        this.loadRequests(this.selectedSampleQuery.id);
        this.loadAllRequests();
      }, 12000); // Wait 12 seconds for logs to be collected
    } catch (error) {
      console.error('Failed to execute query:', error);
      alert('Failed to execute query: ' + error.message);
    }
  }

  async deleteSampleQuery() {
    if (!this.selectedSampleQuery) return;

    if (!confirm(`Delete sample query "${this.selectedSampleQuery.name}"? This will also delete all requests.`)) {
      return;
    }

    try {
      const response = await fetch(`/api/requests/${this.selectedSampleQuery.id}`, {
        method: 'DELETE',
      });

      if (!response.ok) {
        throw new Error('Failed to delete sample query');
      }

      this.selectedSampleQuery = null;
      document.getElementById('emptyState').classList.remove('hidden');
      document.getElementById('sampleQueryDetail').classList.add('hidden');
      
      await this.loadSampleQueries();
      await this.loadAllRequests();
    } catch (error) {
      console.error('Failed to delete sample query:', error);
      alert('Failed to delete sample query: ' + error.message);
    }
  }

  async runExplain(query, variables = {}) {
    try {
      const payload = {
        query: query,
        variables: variables,
      };

      const response = await fetch("/api/explain", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(payload),
      });

      const result = await response.json();
      
      // Show EXPLAIN result in a simple alert for now
      // TODO: Integrate PEV2 visualization
      if (result.error) {
        alert(`EXPLAIN Error: ${result.error}`);
      } else {
        const planText = result.queryPlan ? JSON.stringify(result.queryPlan, null, 2) : 'No plan available';
        console.log('EXPLAIN Plan:', planText);
        alert('EXPLAIN plan logged to console. Check browser console for details.');
      }
    } catch (error) {
      alert(`Failed to run EXPLAIN: ${error.message}`);
    }
  }

  escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }
}

// Initialize app
const app = new RequestManager();
