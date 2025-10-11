class RequestManager {
  constructor() {
    this.requests = [];
    this.servers = [];
    this.selectedRequest = null;
    this.executions = [];
    this.init();
  }

  async init() {
    this.setupEventListeners();
    await this.loadServers();
    await this.loadRequests();
  }

  setupEventListeners() {
    // New request modal
    document.getElementById('newRequestBtn').addEventListener('click', () => {
      this.showNewRequestModal();
    });

    document.getElementById('closeNewRequestModal').addEventListener('click', () => {
      this.hideNewRequestModal();
    });

    document.getElementById('cancelNewRequestBtn').addEventListener('click', () => {
      this.hideNewRequestModal();
    });

    document.getElementById('saveRequestBtn').addEventListener('click', () => {
      this.saveNewRequest();
    });

    // Server selection change
    document.getElementById('newRequestServer').addEventListener('change', (e) => {
      this.handleServerChange(e.target.value);
    });

    // Execution modal
    document.getElementById('closeExecutionModal').addEventListener('click', () => {
      this.hideExecutionModal();
    });

    // Request actions
    document.getElementById('executeBtn').addEventListener('click', () => {
      this.executeRequest();
    });

    document.getElementById('deleteBtn').addEventListener('click', () => {
      this.deleteRequest();
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

  async loadRequests() {
    try {
      const response = await fetch('/api/requests');
      this.requests = await response.json();
      this.renderRequestsList();
    } catch (error) {
      console.error('Failed to load requests:', error);
    }
  }

  renderRequestsList() {
    const container = document.getElementById('requestsList');
    
    if (this.requests.length === 0) {
      container.innerHTML = '<p style="padding: 1rem; color: #6c757d; text-align: center;">No saved requests</p>';
      return;
    }

    container.innerHTML = this.requests.map(req => `
      <div class="request-item ${this.selectedRequest?.id === req.id ? 'active' : ''}" 
           data-id="${req.id}">
        <div class="request-item-name">${this.escapeHtml(req.name)}</div>
        <div class="request-item-url">${this.escapeHtml(req.server ? req.server.url : '(no server)')}</div>
        <div class="request-item-meta">
          <span>${new Date(req.createdAt).toLocaleDateString()}</span>
        </div>
      </div>
    `).join('');

    // Add click handlers
    container.querySelectorAll('.request-item').forEach(item => {
      item.addEventListener('click', () => {
        const id = parseInt(item.dataset.id);
        this.selectRequest(id);
      });
    });
  }

  async selectRequest(id) {
    const request = this.requests.find(r => r.id === id);
    if (!request) return;

    this.selectedRequest = request;
    this.renderRequestsList();
    this.showRequestDetail(request);
    await this.loadExecutions(id);
  }

  showRequestDetail(request) {
    document.getElementById('emptyState').classList.add('hidden');
    document.getElementById('requestDetail').classList.remove('hidden');

    document.getElementById('requestName').textContent = request.name;
    
    // Show server info if available
    if (request.server) {
      document.getElementById('requestURL').textContent = request.server.url;
    } else {
      document.getElementById('requestURL').textContent = '(no server configured)';
    }
    
    document.getElementById('requestCreated').textContent = new Date(request.createdAt).toLocaleString();
    
    // Pretty print JSON
    try {
      const data = JSON.parse(request.requestData);
      document.getElementById('requestData').textContent = JSON.stringify(data, null, 2);
    } catch (e) {
      document.getElementById('requestData').textContent = request.requestData;
    }
  }

  async loadExecutions(requestId) {
    try {
      const response = await fetch(`/api/executions?request_id=${requestId}`);
      this.executions = await response.json();
      this.renderExecutions();
    } catch (error) {
      console.error('Failed to load executions:', error);
      this.executions = [];
      this.renderExecutions();
    }
  }

  renderExecutions() {
    const container = document.getElementById('executionsList');
    
    if (this.executions.length === 0) {
      container.innerHTML = '<p style="color: #6c757d;">No executions yet. Click "Execute Request" to run this request.</p>';
      return;
    }

    container.innerHTML = this.executions.map(exec => {
      const statusClass = exec.statusCode >= 200 && exec.statusCode < 300 ? 'success' : 'error';
      return `
        <div class="execution-item" data-id="${exec.id}">
          <div class="execution-status ${statusClass}">${exec.statusCode || 'ERR'}</div>
          <div class="execution-info">
            <div class="execution-time">${new Date(exec.executedAt).toLocaleString()}</div>
            <div class="execution-stats">
              <span>ID: ${exec.requestIdHeader}</span>
            </div>
          </div>
          <div class="execution-duration">${exec.durationMs}ms</div>
          <div>→</div>
        </div>
      `;
    }).join('');

    // Add click handlers
    container.querySelectorAll('.execution-item').forEach(item => {
      item.addEventListener('click', () => {
        const id = parseInt(item.dataset.id);
        this.showExecutionDetail(id);
      });
    });
  }

  async showExecutionDetail(executionId) {
    try {
      const response = await fetch(`/api/executions/${executionId}`);
      const detail = await response.json();
      this.renderExecutionDetail(detail);
    } catch (error) {
      console.error('Failed to load execution detail:', error);
    }
  }

  renderExecutionDetail(detail) {
    const modal = document.getElementById('executionModal');
    
    // Overview stats
    const statusClass = detail.execution.statusCode >= 200 && detail.execution.statusCode < 300 ? 'success' : 'error';
    document.getElementById('execStatusCode').textContent = detail.execution.statusCode || 'Error';
    document.getElementById('execStatusCode').className = `stat-value ${statusClass}`;
    document.getElementById('execDuration').textContent = `${detail.execution.durationMs}ms`;
    document.getElementById('execRequestID').textContent = detail.execution.requestIdHeader;
    document.getElementById('execTime').textContent = new Date(detail.execution.executedAt).toLocaleString();

    // Response
    if (detail.execution.responseBody) {
      try {
        const data = JSON.parse(detail.execution.responseBody);
        document.getElementById('execResponse').textContent = JSON.stringify(data, null, 2);
      } catch (e) {
        document.getElementById('execResponse').textContent = detail.execution.responseBody;
      }
    } else {
      document.getElementById('execResponse').textContent = '(no response)';
    }

    // Error
    if (detail.execution.error) {
      document.getElementById('execErrorSection').style.display = 'block';
      document.getElementById('execError').textContent = detail.execution.error;
    } else {
      document.getElementById('execErrorSection').style.display = 'none';
    }

    // SQL Analysis
    if (detail.sqlAnalysis && detail.sqlAnalysis.totalQueries > 0) {
      document.getElementById('execSQLSection').style.display = 'block';
      document.getElementById('sqlTotalQueries').textContent = detail.sqlAnalysis.totalQueries;
      document.getElementById('sqlUniqueQueries').textContent = detail.sqlAnalysis.uniqueQueries;
      document.getElementById('sqlAvgDuration').textContent = `${detail.sqlAnalysis.avgDuration.toFixed(2)}ms`;
      document.getElementById('sqlTotalDuration').textContent = `${detail.sqlAnalysis.totalDuration.toFixed(2)}ms`;

      // Render SQL queries
      const sqlContainer = document.getElementById('sqlQueriesList');
      sqlContainer.innerHTML = detail.sqlQueries.map(q => `
        <div class="sql-query-item">
          <div class="sql-query-header">
            <span>${q.tableName || 'unknown'} - ${q.operation || 'SELECT'}</span>
            <span class="sql-query-duration">${q.durationMs.toFixed(2)}ms</span>
          </div>
          <div class="sql-query-text">${this.escapeHtml(q.query)}</div>
        </div>
      `).join('');
    } else {
      document.getElementById('execSQLSection').style.display = 'none';
    }

    // Logs
    document.getElementById('logsCount').textContent = detail.logs.length;
    const logsContainer = document.getElementById('execLogs');
    
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

  hideExecutionModal() {
    document.getElementById('executionModal').classList.add('hidden');
  }

  showNewRequestModal() {
    // Reset form
    document.getElementById('newRequestName').value = '';
    document.getElementById('newRequestURL').value = '';
    document.getElementById('newRequestData').value = '';
    document.getElementById('newRequestToken').value = '';
    document.getElementById('newRequestDevID').value = '';
    
    // Populate server dropdown
    const serverSelect = document.getElementById('newRequestServer');
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
    
    document.getElementById('newRequestModal').classList.remove('hidden');
  }

  handleServerChange(serverId) {
    const newServerFields = document.getElementById('newServerFields');
    if (serverId === '') {
      // New server mode - show URL/token/devID fields
      newServerFields.style.display = 'block';
      document.getElementById('newRequestURL').required = true;
    } else {
      // Existing server mode - hide URL/token/devID fields
      newServerFields.style.display = 'none';
      document.getElementById('newRequestURL').required = false;
    }
  }

  hideNewRequestModal() {
    document.getElementById('newRequestModal').classList.add('hidden');
  }

  async saveNewRequest() {
    const name = document.getElementById('newRequestName').value.trim();
    const requestData = document.getElementById('newRequestData').value.trim();
    const serverSelect = document.getElementById('newRequestServer');
    const serverId = serverSelect.value;

    if (!name || !requestData) {
      alert('Please fill in all required fields');
      return;
    }

    // Validate JSON
    try {
      JSON.parse(requestData);
    } catch (e) {
      alert('Invalid JSON in request data');
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
      const url = document.getElementById('newRequestURL').value.trim();
      const bearerToken = document.getElementById('newRequestToken').value.trim();
      const devId = document.getElementById('newRequestDevID').value.trim();

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
        throw new Error('Failed to save request');
      }

      this.hideNewRequestModal();
      await this.loadServers(); // Reload servers in case a new one was created
      await this.loadRequests();
    } catch (error) {
      console.error('Failed to save request:', error);
      alert('Failed to save request: ' + error.message);
    }
  }

  async executeRequest() {
    if (!this.selectedRequest) return;

    if (!confirm('Execute this request? This will make a live API call.')) {
      return;
    }

    try {
      const response = await fetch(`/api/requests/${this.selectedRequest.id}/execute`, {
        method: 'POST',
      });

      if (!response.ok) {
        throw new Error('Failed to execute request');
      }

      alert('Request execution started. Results will appear in the executions list.');
      
      // Reload executions after a delay
      setTimeout(() => {
        this.loadExecutions(this.selectedRequest.id);
      }, 12000); // Wait 12 seconds for logs to be collected
    } catch (error) {
      console.error('Failed to execute request:', error);
      alert('Failed to execute request: ' + error.message);
    }
  }

  async deleteRequest() {
    if (!this.selectedRequest) return;

    if (!confirm(`Delete request "${this.selectedRequest.name}"? This will also delete all executions.`)) {
      return;
    }

    try {
      const response = await fetch(`/api/requests/${this.selectedRequest.id}`, {
        method: 'DELETE',
      });

      if (!response.ok) {
        throw new Error('Failed to delete request');
      }

      this.selectedRequest = null;
      document.getElementById('emptyState').classList.remove('hidden');
      document.getElementById('requestDetail').classList.add('hidden');
      
      await this.loadRequests();
    } catch (error) {
      console.error('Failed to delete request:', error);
      alert('Failed to delete request: ' + error.message);
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
