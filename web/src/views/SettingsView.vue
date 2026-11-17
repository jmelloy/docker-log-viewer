<template>
  <div class="app-container">
    <app-header activePage="settings"></app-header>

    <div class="settings-container">
      <section class="settings-section">
        <div class="section-header">
          <h2>Servers</h2>
          <button @click="openNewServerModal" class="btn btn-primary">+ New Server</button>
        </div>

        <div v-if="servers.length === 0" class="empty-state-box">
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
              <td>{{ server.defaultDatabase ? server.defaultDatabase.name : "None" }}</td>
              <td>
                <button @click="openEditServerModal(server)" class="btn btn-sm btn-outline-primary">Edit</button>
                <button
                  @click="deleteServer(server.id)"
                  class="btn btn-sm btn-outline-danger"
                  style="margin-left: 0.5rem"
                >
                  Delete
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </section>

      <section class="settings-section">
        <div class="section-header">
          <h2>Database URLs</h2>
          <button @click="openNewDatabaseModal" class="btn btn-primary">+ New Database URL</button>
        </div>

        <div v-if="databaseURLs.length === 0" class="empty-state-box">
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
              <td style="max-width: 400px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap">
                {{ db.connectionString }}
              </td>
              <td>
                <button @click="openEditDatabaseModal(db)" class="btn btn-sm btn-outline-primary">Edit</button>
                <button
                  @click="deleteDatabase(db.id)"
                  class="btn btn-sm btn-outline-danger"
                  style="margin-left: 0.5rem"
                >
                  Delete
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </section>

      <section class="settings-section">
        <div class="section-header">
          <h2>EKS Clusters</h2>
          <button @click="openEKSModal(null)" class="btn btn-primary">+ Add EKS Cluster</button>
        </div>

        <div v-if="eksClusters.length === 0" class="empty-state-box">
          No EKS clusters configured. Pod logs from added clusters appear on the Logs page under "eks:&lt;cluster&gt;".
        </div>

        <table v-else class="table table-striped">
          <thead>
            <tr>
              <th>Cluster</th>
              <th>Profile / Region</th>
              <th>Namespace</th>
              <th>Label Selector</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="cluster in eksClusters" :key="cluster.id">
              <td>{{ cluster.name }}</td>
              <td>{{ cluster.profile || "(default)" }} / {{ cluster.region }}</td>
              <td>{{ cluster.namespace || "(all)" }}</td>
              <td>{{ cluster.labelSelector || "(all pods)" }}</td>
              <td>
                <span v-if="!cluster.enabled">Disabled</span>
                <span
                  v-else-if="eksStatus(cluster.name)?.error"
                  class="text-danger"
                  :title="eksStatus(cluster.name)?.error"
                >
                  Error: {{ eksStatus(cluster.name)?.error }}
                </span>
                <span v-else-if="eksStatus(cluster.name)?.connected" class="text-success">
                  Connected ({{ eksStatus(cluster.name)?.containers }} containers)
                </span>
                <span v-else>Connecting…</span>
              </td>
              <td>
                <button @click="openEKSModal(cluster)" class="btn btn-sm btn-outline-primary">Edit</button>
                <button
                  @click="eksLogin(cluster.profile)"
                  class="btn btn-sm btn-outline-secondary"
                  style="margin-left: 0.5rem"
                >
                  SSO Login
                </button>
                <button
                  @click="deleteEKSCluster(cluster.id)"
                  class="btn btn-sm btn-outline-danger"
                  style="margin-left: 0.5rem"
                >
                  Delete
                </button>
              </td>
            </tr>
          </tbody>
        </table>
        <pre v-if="eksLoginOutput" class="empty-state-box" style="white-space: pre-wrap">{{ eksLoginOutput }}</pre>
      </section>
    </div>

    <!-- Server Modal -->
    <div
      v-if="showServerModal"
      class="modal"
      style="display: block; background: rgba(0, 0, 0, 0.5)"
      @click.self="closeServerModal"
    >
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
              <input
                v-model="serverForm.url"
                type="text"
                class="form-control"
                placeholder="https://api.example.com/graphql"
                required
              />
            </div>
            <div class="mb-3">
              <label class="form-label">Bearer Token</label>
              <input v-model="serverForm.bearerToken" type="text" class="form-control" />
            </div>
            <div class="mb-3">
              <label for="serverHeaders" class="form-label">Custom Headers (JSON)</label>
              <textarea
                id="serverHeaders"
                v-model="serverForm.headers"
                class="form-control"
                rows="5"
                placeholder='{"X-Custom-Header": "value"}'
              ></textarea>
              <small>Default headers for requests sent to this server.</small>
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
    <div
      v-if="showDatabaseModal"
      class="modal"
      style="display: block; background: rgba(0, 0, 0, 0.5)"
      @click.self="closeDatabaseModal"
    >
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
              <input
                v-model="databaseForm.connectionString"
                type="text"
                class="form-control"
                placeholder="postgresql://user:pass@localhost:5432/dbname"
                required
              />
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="closeDatabaseModal">Cancel</button>
            <button type="button" class="btn btn-primary" @click="saveDatabase">Save</button>
          </div>
        </div>
      </div>
    </div>

    <!-- EKS Cluster Modal -->
    <div
      v-if="showEKSModal"
      class="modal"
      style="display: block; background: rgba(0, 0, 0, 0.5)"
      @click.self="showEKSModal = false"
    >
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ eksForm.id ? "Edit EKS Cluster" : "Add EKS Cluster" }}</h5>
            <button type="button" class="btn-close" @click="showEKSModal = false"></button>
          </div>
          <div class="modal-body">
            <div class="mb-3">
              <label for="eksProfile" class="form-label">AWS Profile</label>
              <input id="eksProfile" v-model="eksForm.profile" type="text" class="form-control" placeholder="default" />
              <small>From ~/.aws/config. Leave empty for env vars / default credential chain.</small>
            </div>
            <div class="mb-3">
              <label for="eksRegion" class="form-label">Region *</label>
              <input id="eksRegion" v-model="eksForm.region" type="text" class="form-control" placeholder="us-east-1" />
            </div>
            <div class="mb-3">
              <button type="button" class="btn btn-sm btn-outline-secondary" @click="eksLogin(eksForm.profile)">
                SSO Login
              </button>
              <button
                type="button"
                class="btn btn-sm btn-outline-primary"
                style="margin-left: 0.5rem"
                :disabled="eksDiscovering"
                @click="discoverEKSClusters"
              >
                {{ eksDiscovering ? "Discovering…" : "Discover Clusters" }}
              </button>
              <div v-if="eksIdentity">
                <small>Signed in as {{ eksIdentity }}</small>
              </div>
              <pre v-if="eksLoginOutput" style="white-space: pre-wrap; font-size: 0.8em">{{ eksLoginOutput }}</pre>
            </div>
            <div class="mb-3">
              <label for="eksName" class="form-label">Cluster Name *</label>
              <input id="eksName" v-model="eksForm.name" type="text" class="form-control" list="eksDiscovered" />
              <datalist id="eksDiscovered">
                <option v-for="name in eksDiscovered" :key="name" :value="name"></option>
              </datalist>
            </div>
            <div class="mb-3">
              <label for="eksNamespace" class="form-label">Namespace</label>
              <input
                id="eksNamespace"
                v-model="eksForm.namespace"
                type="text"
                class="form-control"
                placeholder="all namespaces"
              />
            </div>
            <div class="mb-3">
              <label for="eksSelector" class="form-label">Label Selector</label>
              <input
                id="eksSelector"
                v-model="eksForm.labelSelector"
                type="text"
                class="form-control"
                placeholder="app=api,tier!=batch"
              />
            </div>
            <div class="mb-3">
              <label for="eksSince" class="form-label">Initial history (minutes)</label>
              <input id="eksSince" v-model.number="eksForm.sinceMinutes" type="number" min="0" class="form-control" />
            </div>
            <div class="form-check">
              <input id="eksEnabled" v-model="eksForm.enabled" type="checkbox" class="form-check-input" />
              <label for="eksEnabled" class="form-check-label">Stream logs from this cluster</label>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="showEKSModal = false">Cancel</button>
            <button type="button" class="btn btn-primary" @click="saveEKSCluster">Save</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import { parseHeaders, serverHeaders } from "@/utils/headers";
import { defineComponent } from "vue";
import { API } from "@/utils/api";
import type {
  Server,
  DatabaseURL,
  CreateServerResponse,
  CreateDatabaseURLResponse,
  EKSCluster,
  EKSClusterStatus,
  EKSClustersResponse,
} from "@/types";

const emptyEKSForm = (): EKSCluster => ({
  id: 0,
  name: "",
  region: "us-east-1",
  profile: "",
  namespace: "",
  labelSelector: "",
  sinceMinutes: 15,
  enabled: true,
});

export default defineComponent(
  // Export component definition (template will be provided by SPA loader)
  {
    data() {
      return {
        servers: [] as Server[],
        databaseURLs: [] as DatabaseURL[],
        // Modal visibility
        showServerModal: false,
        showDatabaseModal: false,
        // Form data
        serverForm: {
          id: null as number | null,
          name: "",
          url: "",
          bearerToken: "",
          headers: "{}",
          defaultDatabaseId: null as number | null,
        },
        databaseForm: {
          id: null as number | null,
          name: "",
          connectionString: "",
          databaseType: "postgresql",
        },
        // Editing state
        editingServer: null as Server | null,
        editingDatabase: null as DatabaseURL | null,
        // EKS
        eksClusters: [] as EKSCluster[],
        eksStatuses: [] as EKSClusterStatus[],
        showEKSModal: false,
        eksForm: emptyEKSForm(),
        eksDiscovered: [] as string[],
        eksDiscovering: false,
        eksIdentity: "",
        eksLoginOutput: "",
        eksPollTimer: undefined as ReturnType<typeof setInterval> | undefined,
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
      await this.loadEKSClusters();
      // Connection status changes as the backend polls clusters.
      this.eksPollTimer = setInterval(() => this.loadEKSClusters(), 5000);
    },

    unmounted() {
      clearInterval(this.eksPollTimer);
    },

    methods: {
      async loadServers() {
        try {
          this.servers = await API.get<Server[]>("/api/servers");
        } catch (error) {
          console.error("Error loading servers:", error);
        }
      },

      async loadDatabaseURLs() {
        try {
          this.databaseURLs = await API.get<DatabaseURL[]>("/api/database-urls");
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
          headers: "{}",
          defaultDatabaseId: null,
        };
        this.showServerModal = true;
      },

      openEditServerModal(server: Server) {
        this.editingServer = server;
        this.serverForm = {
          id: server.id,
          name: server.name,
          url: server.url,
          bearerToken: server.bearerToken || "",
          headers: JSON.stringify(serverHeaders(server), null, 2),
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
          const payload: Partial<Server> = {
            name: this.serverForm.name,
            url: this.serverForm.url,
            bearerToken: this.serverForm.bearerToken,
            headers: parseHeaders(this.serverForm.headers),
            devId: "",
            experimentalMode: "",
            defaultDatabaseId: this.serverForm.defaultDatabaseId || null,
          };

          let response: Response;
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
          alert(`Error saving server: ${error.message}`);
        }
      },

      async deleteServer(id: number) {
        if (!confirm("Are you sure you want to delete this server?")) {
          return;
        }

        try {
          await API.delete(`/api/servers/${id}`);
          await this.loadServers();
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

      openEditDatabaseModal(database: DatabaseURL) {
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
          const payload: Partial<DatabaseURL> = {
            name: this.databaseForm.name,
            connectionString: this.databaseForm.connectionString,
            databaseType: this.databaseForm.databaseType,
          };

          let response: Response;
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

      async deleteDatabase(id: number) {
        if (!confirm("Are you sure you want to delete this database URL?")) {
          return;
        }

        try {
          await API.delete(`/api/database-urls/${id}`);
          await this.loadDatabaseURLs();
        } catch (error) {
          console.error("Error deleting database URL:", error);
          alert("Error deleting database URL");
        }
      },

      async loadEKSClusters() {
        try {
          const resp = await API.get<EKSClustersResponse>("/api/eks/clusters");
          this.eksClusters = resp.clusters || [];
          this.eksStatuses = resp.status || [];
        } catch (error) {
          console.error("Error loading EKS clusters:", error);
        }
      },

      eksStatus(name: string): EKSClusterStatus | undefined {
        return this.eksStatuses.find((s) => s.name === name);
      },

      openEKSModal(cluster: EKSCluster | null) {
        this.eksForm = cluster ? { ...cluster } : emptyEKSForm();
        this.eksDiscovered = [];
        this.eksIdentity = "";
        this.eksLoginOutput = "";
        this.showEKSModal = true;
      },

      async discoverEKSClusters() {
        this.eksDiscovering = true;
        try {
          const params = new URLSearchParams({ profile: this.eksForm.profile, region: this.eksForm.region });
          const resp = await API.get<{ identity: string; clusters: string[] }>(`/api/eks/discover?${params}`);
          this.eksIdentity = resp.identity;
          this.eksDiscovered = resp.clusters;
          if (resp.clusters.length === 0) {
            alert("No EKS clusters found for this profile/region.");
          } else if (!this.eksForm.name) {
            this.eksForm.name = resp.clusters[0];
          }
        } catch (error) {
          alert(`Discovery failed: ${(error as Error).message}`);
        } finally {
          this.eksDiscovering = false;
        }
      },

      async eksLogin(profile: string) {
        this.eksLoginOutput = "Starting aws sso login…";
        try {
          const resp = await API.post<{ output: string }>("/api/eks/login", { profile });
          this.eksLoginOutput =
            resp.output + "\nComplete the sign-in in your browser; clusters reconnect automatically.";
        } catch (error) {
          this.eksLoginOutput = `SSO login failed: ${(error as Error).message}`;
        }
      },

      async saveEKSCluster() {
        try {
          if (this.eksForm.id) {
            await API.put(`/api/eks/clusters/${this.eksForm.id}`, this.eksForm);
          } else {
            await API.post("/api/eks/clusters", this.eksForm);
          }
          this.showEKSModal = false;
          await this.loadEKSClusters();
        } catch (error) {
          alert(`Failed to save EKS cluster: ${(error as Error).message}`);
        }
      },

      async deleteEKSCluster(id: number) {
        if (!confirm("Delete this EKS cluster? Its log streams will stop.")) {
          return;
        }
        try {
          await API.delete(`/api/eks/clusters/${id}`);
          await this.loadEKSClusters();
        } catch (error) {
          alert(`Error deleting EKS cluster: ${(error as Error).message}`);
        }
      },

      getDatabaseName(id: number | null | undefined): string {
        if (!id) return "None";
        const db = this.databaseURLs.find((d) => d.id === id);
        return db ? db.name : "None";
      },
    },
  }
);
</script>
