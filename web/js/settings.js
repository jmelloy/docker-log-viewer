import { createAppHeader } from './shared/navigation.js';
import { API } from './shared/api.js';
import { loadTemplate } from './shared/template-loader.js';

const template = await loadTemplate("/templates/settings-main.html");

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
        this.servers = await API.get("/api/servers");
      } catch (error) {
        console.error("Error loading servers:", error);
      }
    },

    async loadDatabaseURLs() {
      try {
        this.databaseURLs = await API.get("/api/database-urls");
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
        await API.delete(`/api/database-urls/${id}`);
        await this.loadDatabaseURLs();
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

  template,
});

app.component('app-header', createAppHeader('settings'));

app.mount("#app");
