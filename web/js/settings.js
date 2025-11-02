import { createNavigation } from './shared/navigation.js';
import { API } from './shared/api.js';

const { createApp } = Vue;

const app = createApp({
  data() {
    return {
      servers: [],
      databaseURLs: [],
      // Modal visibility
      showServerModal: false,
      showDatabaseModal: false,
      // Form data
      serverForm: {
        id: null,
        name: "",
        url: "",
        bearerToken: "",
        devId: "",
        defaultDatabaseId: null,
      },
      databaseForm: {
        id: null,
        name: "",
        connectionString: "",
        databaseType: "postgresql",
      },
      // Editing state
      editingServer: null,
      editingDatabase: null,
    };
  },

  computed: {
    serverModalTitle() {
      return this.editingServer ? "Edit Server" : "New Server";
    },
    databaseModalTitle() {
      return this.editingDatabase ? "Edit Database URL" : "New Database URL";
    },
  },

  async mounted() {
    await this.loadServers();
    await this.loadDatabaseURLs();
  },

  methods: {
    async loadServers() {
      try {
        const response = await fetch("/api/servers");
        if (response.ok) {
          this.servers = await response.json();
        } else {
          console.error("Failed to load servers:", response.statusText);
        }
      } catch (error) {
        console.error("Error loading servers:", error);
      }
    },

    async loadDatabaseURLs() {
      try {
        const response = await fetch("/api/database-urls");
        if (response.ok) {
          this.databaseURLs = await response.json();
        } else {
          console.error("Failed to load database URLs:", response.statusText);
        }
      } catch (error) {
        console.error("Error loading database URLs:", error);
      }
    },

    openNewServerModal() {
      this.editingServer = null;
      this.serverForm = {
        id: null,
        name: "",
        url: "",
        bearerToken: "",
        devId: "",
        defaultDatabaseId: null,
      };
      this.showServerModal = true;
    },

    openEditServerModal(server) {
      this.editingServer = server;
      this.serverForm = {
        id: server.id,
        name: server.name,
        url: server.url,
        bearerToken: server.bearerToken || "",
        devId: server.devId || "",
        defaultDatabaseId: server.defaultDatabaseId || null,
      };
      this.showServerModal = true;
    },

    closeServerModal() {
      this.showServerModal = false;
      this.editingServer = null;
    },

    async saveServer() {
      try {
        const payload = {
          name: this.serverForm.name,
          url: this.serverForm.url,
          bearerToken: this.serverForm.bearerToken,
          devId: this.serverForm.devId,
          defaultDatabaseId: this.serverForm.defaultDatabaseId || null,
        };

        let response;
        if (this.editingServer) {
          // Update existing server
          response = await fetch(`/api/servers/${this.serverForm.id}`, {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(payload),
          });
        } else {
          // Create new server
          response = await fetch("/api/servers", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(payload),
          });
        }

        if (response.ok) {
          await this.loadServers();
          this.closeServerModal();
        } else {
          const errorText = await response.text();
          alert(`Failed to save server: ${errorText}`);
        }
      } catch (error) {
        console.error("Error saving server:", error);
        alert("Error saving server");
      }
    },

    async deleteServer(id) {
      if (!confirm("Are you sure you want to delete this server?")) {
        return;
      }

      try {
        const response = await fetch(`/api/servers/${id}`, {
          method: "DELETE",
        });

        if (response.ok) {
          await this.loadServers();
        } else {
          const errorText = await response.text();
          alert(`Failed to delete server: ${errorText}`);
        }
      } catch (error) {
        console.error("Error deleting server:", error);
        alert("Error deleting server");
      }
    },

    openNewDatabaseModal() {
      this.editingDatabase = null;
      this.databaseForm = {
        id: null,
        name: "",
        connectionString: "",
        databaseType: "postgresql",
      };
      this.showDatabaseModal = true;
    },

    openEditDatabaseModal(database) {
      this.editingDatabase = database;
      this.databaseForm = {
        id: database.id,
        name: database.name,
        connectionString: database.connectionString,
        databaseType: database.databaseType || "postgresql",
      };
      this.showDatabaseModal = true;
    },

    closeDatabaseModal() {
      this.showDatabaseModal = false;
      this.editingDatabase = null;
    },

    async saveDatabase() {
      try {
        const payload = {
          name: this.databaseForm.name,
          connectionString: this.databaseForm.connectionString,
          databaseType: this.databaseForm.databaseType,
        };

        let response;
        if (this.editingDatabase) {
          // Update existing database URL
          response = await fetch(`/api/database-urls/${this.databaseForm.id}`, {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(payload),
          });
        } else {
          // Create new database URL
          response = await fetch("/api/database-urls", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(payload),
          });
        }

        if (response.ok) {
          await this.loadDatabaseURLs();
          this.closeDatabaseModal();
        } else {
          const errorText = await response.text();
          alert(`Failed to save database URL: ${errorText}`);
        }
      } catch (error) {
        console.error("Error saving database URL:", error);
        alert("Error saving database URL");
      }
    },

    async deleteDatabase(id) {
      if (!confirm("Are you sure you want to delete this database URL?")) {
        return;
      }

      try {
        const response = await fetch(`/api/database-urls/${id}`, {
          method: "DELETE",
        });

        if (response.ok) {
          await this.loadDatabaseURLs();
        } else {
          const errorText = await response.text();
          alert(`Failed to delete database URL: ${errorText}`);
        }
      } catch (error) {
        console.error("Error deleting database URL:", error);
        alert("Error deleting database URL");
      }
    },

    getDatabaseName(id) {
      const db = this.databaseURLs.find((d) => d.id === id);
      return db ? db.name : "None";
    },
  },

  template: `
    <div class="app-container">
      <header class="app-header">
        <div style="display: flex; align-items: center; gap: 1rem">
          <h1 style="margin: 0">🔱 Logseidon</h1>
          <app-nav></app-nav>
        </div>
      </header>

      <div class="container" style="max-width: 1200px; margin: 2rem auto; padding: 0 2rem">
        <!-- Servers Section -->
        <section style="margin-bottom: 3rem">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem">
            <h2>Servers</h2>
            <button @click="openNewServerModal" class="btn btn-primary">+ New Server</button>
          </div>
          
          <div v-if="servers.length === 0" style="padding: 2rem; text-align: center; color: #6c757d; border: 1px solid #dee2e6; border-radius: 4px">
            No servers configured. Click "New Server" to add one.
          </div>

          <table v-else class="table table-striped">
            <thead>
              <tr>
                <th>Name</th>
                <th>URL</th>
                <th>Default Database</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="server in servers" :key="server.id">
                <td>{{ server.name }}</td>
                <td>{{ server.url }}</td>
                <td>{{ server.defaultDatabase ? server.defaultDatabase.name : 'None' }}</td>
                <td>
                  <button @click="openEditServerModal(server)" class="btn btn-sm btn-outline-primary">Edit</button>
                  <button @click="deleteServer(server.id)" class="btn btn-sm btn-outline-danger" style="margin-left: 0.5rem">Delete</button>
                </td>
              </tr>
            </tbody>
          </table>
        </section>

        <!-- Database URLs Section -->
        <section>
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem">
            <h2>Database URLs</h2>
            <button @click="openNewDatabaseModal" class="btn btn-primary">+ New Database URL</button>
          </div>
          
          <div v-if="databaseURLs.length === 0" style="padding: 2rem; text-align: center; color: #6c757d; border: 1px solid #dee2e6; border-radius: 4px">
            No database URLs configured. Click "New Database URL" to add one.
          </div>

          <table v-else class="table table-striped">
            <thead>
              <tr>
                <th>Name</th>
                <th>Type</th>
                <th>Connection String</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="db in databaseURLs" :key="db.id">
                <td>{{ db.name }}</td>
                <td>{{ db.databaseType }}</td>
                <td style="max-width: 400px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap">{{ db.connectionString }}</td>
                <td>
                  <button @click="openEditDatabaseModal(db)" class="btn btn-sm btn-outline-primary">Edit</button>
                  <button @click="deleteDatabase(db.id)" class="btn btn-sm btn-outline-danger" style="margin-left: 0.5rem">Delete</button>
                </td>
              </tr>
            </tbody>
          </table>
        </section>
      </div>

      <!-- Server Modal -->
      <div v-if="showServerModal" class="modal" style="display: block; background: rgba(0,0,0,0.5)" @click.self="closeServerModal">
        <div class="modal-dialog">
          <div class="modal-content">
            <div class="modal-header">
              <h5 class="modal-title">{{ serverModalTitle }}</h5>
              <button type="button" class="btn-close" @click="closeServerModal"></button>
            </div>
            <div class="modal-body">
              <div class="mb-3">
                <label class="form-label">Name *</label>
                <input v-model="serverForm.name" type="text" class="form-control" required />
              </div>
              <div class="mb-3">
                <label class="form-label">URL *</label>
                <input v-model="serverForm.url" type="text" class="form-control" placeholder="https://api.example.com/graphql" required />
              </div>
              <div class="mb-3">
                <label class="form-label">Bearer Token</label>
                <input v-model="serverForm.bearerToken" type="text" class="form-control" />
              </div>
              <div class="mb-3">
                <label class="form-label">Dev ID</label>
                <input v-model="serverForm.devId" type="text" class="form-control" />
              </div>
              <div class="mb-3">
                <label class="form-label">Default Database</label>
                <select v-model="serverForm.defaultDatabaseId" class="form-select">
                  <option :value="null">None</option>
                  <option v-for="db in databaseURLs" :key="db.id" :value="db.id">{{ db.name }}</option>
                </select>
              </div>
            </div>
            <div class="modal-footer">
              <button type="button" class="btn btn-secondary" @click="closeServerModal">Cancel</button>
              <button type="button" class="btn btn-primary" @click="saveServer">Save</button>
            </div>
          </div>
        </div>
      </div>

      <!-- Database URL Modal -->
      <div v-if="showDatabaseModal" class="modal" style="display: block; background: rgba(0,0,0,0.5)" @click.self="closeDatabaseModal">
        <div class="modal-dialog">
          <div class="modal-content">
            <div class="modal-header">
              <h5 class="modal-title">{{ databaseModalTitle }}</h5>
              <button type="button" class="btn-close" @click="closeDatabaseModal"></button>
            </div>
            <div class="modal-body">
              <div class="mb-3">
                <label class="form-label">Name *</label>
                <input v-model="databaseForm.name" type="text" class="form-control" required />
              </div>
              <div class="mb-3">
                <label class="form-label">Database Type *</label>
                <select v-model="databaseForm.databaseType" class="form-select">
                  <option value="postgresql">PostgreSQL</option>
                  <option value="mysql">MySQL</option>
                </select>
              </div>
              <div class="mb-3">
                <label class="form-label">Connection String *</label>
                <input v-model="databaseForm.connectionString" type="text" class="form-control" placeholder="postgresql://user:pass@localhost:5432/dbname" required />
              </div>
            </div>
            <div class="modal-footer">
              <button type="button" class="btn btn-secondary" @click="closeDatabaseModal">Cancel</button>
              <button type="button" class="btn btn-primary" @click="saveDatabase">Save</button>
            </div>
          </div>
        </div>
      </div>
    </div>
  `,
});

// Register components
app.component('app-nav', createNavigation('settings'));

app.mount("#app");
