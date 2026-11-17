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
          <div>
            <button @click="importKubeconfig" class="btn btn-outline-secondary" :disabled="eksImporting">
              {{ eksImporting ? "Reading kubeconfig…" : "Refresh from kubeconfig" }}
            </button>
            <button @click="openEKSModal(null)" class="btn btn-primary" style="margin-left: 0.5rem">
              + Add EKS Cluster
            </button>
          </div>
        </div>

        <div v-if="eksAuthMessage" class="empty-state-box" style="white-space: pre-wrap; text-align: left">
          {{ eksAuthMessage }}
          <div v-if="eksAuthURL">
            <a :href="eksAuthURL" target="_blank" rel="noopener">Open the AWS sign-in page</a>
          </div>
        </div>

        <div v-if="eksClusters.length === 0" class="empty-state-box">
          No EKS clusters configured. Use "Refresh from kubeconfig" to import contexts from ~/.kube/config, or add one
          manually. Pod logs appear on the Logs page under "eks:&lt;cluster&gt;".
        </div>

        <table v-else class="table table-striped">
          <thead>
            <tr>
              <th>On</th>
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
              <td>
                <input
                  type="checkbox"
                  :checked="cluster.enabled"
                  :aria-label="`Stream logs from ${cluster.name}`"
                  @change="toggleEKSCluster(cluster)"
                />
              </td>
              <td>
                {{ cluster.name }}
                <small v-if="cluster.clusterName && cluster.clusterName !== cluster.name">
                  ({{ cluster.clusterName }})
                </small>
              </td>
              <td>{{ cluster.profile || "(default)" }} / {{ cluster.region }}</td>
              <td>{{ cluster.namespace || "(all)" }}</td>
              <td>{{ cluster.labelSelector || "(all pods)" }}</td>
              <td>
                <span v-if="!cluster.enabled">Disabled</span>
                <template v-else>
                  <span :class="eksStateClass(cluster.name)">{{ eksStateLabel(cluster.name) }}</span>
                  <div v-if="eksStatus(cluster.name)?.error">
                    <small>{{ eksStatus(cluster.name)?.error }}</small>
                  </div>
                </template>
              </td>
              <td style="white-space: nowrap">
                <button @click="openEKSModal(cluster)" class="btn btn-sm btn-outline-primary">Edit</button>
                <button
                  v-if="cluster.enabled"
                  @click="reconnectEKS(cluster.name)"
                  class="btn btn-sm btn-outline-secondary"
                  style="margin-left: 0.5rem"
                  :disabled="eksReconnecting === cluster.name"
                >
                  {{ eksReconnecting === cluster.name ? "Reconnecting…" : "Reconnect" }}
                </button>
                <button
                  v-if="cluster.enabled"
                  @click="reauthenticate(cluster.profile)"
                  :class="[
                    'btn',
                    'btn-sm',
                    eksStatus(cluster.name)?.state === 'auth_expired' ? 'btn-warning' : 'btn-outline-secondary',
                  ]"
                  style="margin-left: 0.5rem"
                >
                  Re-authenticate
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
              <button type="button" class="btn btn-sm btn-outline-secondary" @click="reauthenticate(eksForm.profile)">
                Re-authenticate
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
              <pre v-if="eksAuthMessage" style="white-space: pre-wrap; font-size: 0.8em">{{ eksAuthMessage }}</pre>
            </div>
            <div class="mb-3">
              <label for="eksClusterName" class="form-label">EKS Cluster Name *</label>
              <input
                id="eksClusterName"
                v-model="eksForm.clusterName"
                type="text"
                class="form-control"
                list="eksDiscovered"
                @change="eksForm.name = eksForm.name || eksForm.clusterName"
              />
              <datalist id="eksDiscovered">
                <option v-for="name in eksDiscovered" :key="name" :value="name"></option>
              </datalist>
            </div>
            <div class="mb-3">
              <label for="eksName" class="form-label">Label *</label>
              <input id="eksName" v-model="eksForm.name" type="text" class="form-control" />
              <small
                >Unique name shown in the Logs page (distinguishes same-named clusters in different accounts).</small
              >
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
  EKSLoginResponse,
  EKSAuthStatus,
} from "@/types";

const EKS_STATE_LABELS: Record<EKSClusterStatus["state"], string> = {
  connecting: "Connecting…",
  connected: "Connected",
  auth_expired: "Auth expired",
  needs_reauth: "Needs re-auth",
  error: "Error",
};

const emptyEKSForm = (): EKSCluster => ({
  id: 0,
  name: "",
  clusterName: "",
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
        eksAuthMessage: "",
        eksAuthURL: "",
        eksAuthPoll: undefined as ReturnType<typeof setInterval> | undefined,
        eksReconnecting: "",
        eksImporting: false,
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
      clearInterval(this.eksAuthPoll);
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
          } else if (!this.eksForm.clusterName) {
            this.eksForm.clusterName = resp.clusters[0];
            this.eksForm.name = this.eksForm.name || resp.clusters[0];
          }
        } catch (error) {
          alert(`Discovery failed: ${(error as Error).message}`);
        } finally {
          this.eksDiscovering = false;
        }
      },

      eksStateLabel(name: string): string {
        const st = this.eksStatus(name);
        if (!st) return "Connecting…";
        return st.state === "connected" ? `Connected (${st.containers} containers)` : EKS_STATE_LABELS[st.state];
      },

      eksStateClass(name: string): string {
        const state = this.eksStatus(name)?.state;
        if (state === "connected") return "text-success";
        if (state === "auth_expired" || state === "needs_reauth") return "text-warning";
        if (state === "error") return "text-danger";
        return "";
      },

      // Re-authenticate: for SSO profiles, start the device-code login and open the AWS
      // verification page in the user's browser (the Okta/IdP sign-in happens there; we
      // don't automate it). For other profiles (credentials written by an Okta tool), show
      // what to run. Then poll until the credentials are valid and reconnect the clusters.
      async reauthenticate(profile: string) {
        // Open the tab synchronously in the click handler so popup blockers allow it.
        const tab = window.open("", "_blank");
        this.eksAuthURL = "";
        this.eksAuthMessage = "Starting sign-in…";
        try {
          const resp = await API.post<EKSLoginResponse>("/api/eks/login", { profile });
          if (resp.url) {
            this.eksAuthURL = resp.url;
            if (tab) tab.location.href = resp.url;
            this.eksAuthMessage = "Complete the sign-in in the opened browser tab. Waiting for new credentials…";
          } else {
            tab?.close();
            this.eksAuthMessage = resp.error
              ? `${resp.error}\n${resp.output || ""}\n${resp.hint}`
              : `${resp.hint}\nWaiting for new credentials…`;
          }
        } catch (error) {
          tab?.close();
          this.eksAuthMessage = `Sign-in failed: ${(error as Error).message}`;
          return;
        }
        this.pollAuth(profile);
      },

      pollAuth(profile: string) {
        clearInterval(this.eksAuthPoll);
        const started = Date.now();
        this.eksAuthPoll = setInterval(async () => {
          if (Date.now() - started > 5 * 60 * 1000) {
            clearInterval(this.eksAuthPoll);
            this.eksAuthMessage += "\nStopped waiting after 5 minutes. Click Reconnect once you've signed in.";
            return;
          }
          try {
            const params = new URLSearchParams({ profile });
            const st = await API.get<EKSAuthStatus>(`/api/eks/auth-status?${params}`);
            if (!st.valid) return;
            clearInterval(this.eksAuthPoll);
            this.eksAuthURL = "";
            this.eksAuthMessage = `Signed in as ${st.identity}. Reconnecting clusters…`;
            const clusters = this.eksClusters.filter((c) => c.enabled && c.profile === profile);
            const failures: string[] = [];
            for (const c of clusters) {
              try {
                await API.post(`/api/eks/reconnect/${encodeURIComponent(c.name)}`, {});
              } catch (error) {
                failures.push(`${c.name}: ${(error as Error).message}`);
              }
            }
            this.eksAuthMessage = failures.length
              ? `Signed in as ${st.identity}, but some clusters failed:\n${failures.join("\n")}`
              : `Signed in as ${st.identity}. ${clusters.length} cluster(s) reconnected.`;
            await this.loadEKSClusters();
          } catch (error) {
            console.error("auth status check failed:", error);
          }
        }, 3000);
      },

      async reconnectEKS(name: string) {
        this.eksReconnecting = name;
        try {
          await API.post(`/api/eks/reconnect/${encodeURIComponent(name)}`, {});
        } catch (error) {
          alert(`Reconnect failed: ${(error as Error).message}`);
        } finally {
          this.eksReconnecting = "";
          await this.loadEKSClusters();
        }
      },

      async importKubeconfig() {
        this.eksImporting = true;
        try {
          const resp = await API.post<{ added: number }>("/api/eks/kubeconfig/import", {});
          await this.loadEKSClusters();
          alert(
            resp.added
              ? `Imported ${resp.added} cluster(s) from kubeconfig. They're disabled; tick "On" to stream logs.`
              : "No new EKS clusters found in kubeconfig."
          );
        } catch (error) {
          alert(`Kubeconfig import failed: ${(error as Error).message}`);
        } finally {
          this.eksImporting = false;
        }
      },

      async toggleEKSCluster(cluster: EKSCluster) {
        try {
          await API.put(`/api/eks/clusters/${cluster.id}`, { ...cluster, enabled: !cluster.enabled });
        } catch (error) {
          alert(`Failed to update cluster: ${(error as Error).message}`);
        }
        await this.loadEKSClusters();
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
