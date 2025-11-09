import { createNavigation } from "./shared/navigation.js";
import { API } from "./shared/api.js";
import { GraphQLEditorManager } from "./graphql-editor-manager.js";

const { createApp } = Vue;

const app = createApp({
  data() {
    return {
      servers: [],
      selectedServerId: "",
      query: "",
      operationName: "",
      variables: "{}",
      executing: false,
      result: null,
      error: null,
      executionId: null,
      showSampleQueries: false,
      sampleQueries: [],
      schema: null,
      loadingSchema: false,
      schemaError: null,
      showSchemaSidebar: false,
      editorManager: null,
      expandedSections: {
        queries: true,
        mutations: true,
        types: false,
      },
      expandedFields: {}, // Maps field key to expanded state
      expandedTypes: {}, // Maps type name to expanded state
    };
  },

  computed: {
    selectedServer() {
      if (!this.selectedServerId) return null;
      return this.servers.find((s) => s.id === parseInt(this.selectedServerId));
    },

    formattedResult() {
      if (!this.result) return "";
      // Check if result is already valid JSON string
      try {
        const parsed = JSON.parse(this.result);
        return JSON.stringify(parsed, null, 2);
      } catch (e) {
        // If not valid JSON, return as-is
        return this.result;
      }
    },

    canExecute() {
      return this.selectedServerId && this.query.trim();
    },

    canLoadSchema() {
      return this.selectedServerId && this.selectedServer;
    },

    schemaTypes() {
      if (!this.schema) return [];
      return this.schema.types || [];
    },

    queryType() {
      if (!this.schema) return null;
      return this.schema.queryType;
    },

    mutationType() {
      if (!this.schema) return null;
      return this.schema.mutationType;
    },
  },

  async mounted() {
    await this.loadServers();
    await this.loadSampleQueries();

    // Initialize CodeMirror editors
    this.editorManager = new GraphQLEditorManager();

    // Wait for next tick to ensure DOM is ready
    this.$nextTick(() => {
      this.initializeEditors();
    });

    // Load example query if nothing is set
    if (!this.query) {
      this.query = `query ExampleQuery {
  # Add your GraphQL query here
}`;
    }
  },

  methods: {
    async loadServers() {
      try {
        this.servers = await API.get("/api/servers");
        // Auto-select first server if available
        if (this.servers.length > 0 && !this.selectedServerId) {
          this.selectedServerId = String(this.servers[0].id);
        }
      } catch (error) {
        console.error("Failed to load servers:", error);
        this.servers = [];
      }
    },

    async loadSampleQueries() {
      try {
        this.sampleQueries = await API.get("/api/requests");
      } catch (error) {
        console.error("Failed to load sample queries:", error);
        this.sampleQueries = [];
      }
    },

    async executeQuery() {
      if (!this.canExecute) return;

      this.executing = true;
      this.error = null;
      this.result = null;
      this.executionId = null;

      try {
        // Build request body
        const requestData = {
          query: this.query,
        };

        if (this.operationName.trim()) {
          requestData.operationName = this.operationName.trim();
        }

        // Parse variables
        if (this.variables.trim() && this.variables.trim() !== "{}") {
          try {
            requestData.variables = JSON.parse(this.variables);
          } catch (e) {
            this.error = `Invalid JSON in variables: ${e.message}`;
            this.executing = false;
            return;
          }
        }

        // Execute via API
        const payload = {
          serverId: parseInt(this.selectedServerId),
          requestData: JSON.stringify(requestData),
        };

        const response = await API.post("/api/execute", payload);

        if (response.executionId) {
          this.executionId = response.executionId;

          // Fetch execution details
          const detail = await API.get(
            `/api/executions/${response.executionId}`
          );

          if (detail.execution.error) {
            this.error = detail.execution.error;
          } else {
            this.result = detail.execution.responseBody;
          }
        } else {
          this.error = "No execution ID returned";
        }
      } catch (error) {
        console.error("Failed to execute query:", error);
        this.error = error.message;
      } finally {
        this.executing = false;
      }
    },

    async loadSampleQuery(sampleQuery) {
      try {
        const data = JSON.parse(sampleQuery.requestData);
        this.query = data.query || "";
        this.operationName = data.operationName || "";
        this.variables = data.variables
          ? JSON.stringify(data.variables, null, 2)
          : "{}";

        if (sampleQuery.serverId) {
          this.selectedServerId = String(sampleQuery.serverId);
        }

        // Update editors if they exist
        if (this.editorManager) {
          this.editorManager.setQueryValue(this.query);
          this.editorManager.setVariablesValue(this.variables);
        }

        this.showSampleQueries = false;
      } catch (e) {
        console.error("Failed to load sample query:", e);
        alert("Failed to load sample query: " + e.message);
      }
    },

    clearQuery() {
      this.query = "";
      this.operationName = "";
      this.variables = "{}";
      this.result = null;
      this.error = null;
      this.executionId = null;

      // Update editors if they exist
      if (this.editorManager) {
        this.editorManager.setQueryValue("");
        this.editorManager.setVariablesValue("{}");
      }
    },

    async loadGraphQLSchema() {
      if (!this.canLoadSchema) {
        alert("Please select a server first");
        return;
      }

      this.loadingSchema = true;
      this.schemaError = null;

      try {
        // GraphQL introspection query
        const introspectionQuery = {
          query: `
            query IntrospectionQuery {
              __schema {
                queryType { name }
                mutationType { name }
                subscriptionType { name }
                types {
                  ...FullType
                }
                directives {
                  name
                  description
                  locations
                  args {
                    ...InputValue
                  }
                }
              }
            }
            
            fragment FullType on __Type {
              kind
              name
              description
              fields(includeDeprecated: true) {
                name
                description
                args {
                  ...InputValue
                }
                type {
                  ...TypeRef
                }
                isDeprecated
                deprecationReason
              }
              inputFields {
                ...InputValue
              }
              interfaces {
                ...TypeRef
              }
              enumValues(includeDeprecated: true) {
                name
                description
                isDeprecated
                deprecationReason
              }
              possibleTypes {
                ...TypeRef
              }
            }
            
            fragment InputValue on __InputValue {
              name
              description
              type {
                ...TypeRef
              }
              defaultValue
            }
            
            fragment TypeRef on __Type {
              kind
              name
              ofType {
                kind
                name
                ofType {
                  kind
                  name
                  ofType {
                    kind
                    name
                    ofType {
                      kind
                      name
                      ofType {
                        kind
                        name
                        ofType {
                          kind
                          name
                          ofType {
                            kind
                            name
                          }
                        }
                      }
                    }
                  }
                }
              }
            }
          `,
        };

        // Execute via API using the same logic as regular query execution
        const payload = {
          serverId: parseInt(this.selectedServerId),
          requestData: JSON.stringify(introspectionQuery),
        };

        const response = await API.post("/api/execute", payload);

        if (response.executionId) {
          let detail = null;
          do {
            detail = await API.get(`/api/executions/${response.executionId}`);

            await new Promise((resolve) => setTimeout(resolve, 1000));
          } while (!detail.execution.statusCode && !detail.execution.error);

          if (detail.execution.error) {
            this.schemaError = detail.execution.error;
          } else {
            const result = JSON.parse(detail.execution.responseBody);
            if (result.data && result.data.__schema) {
              this.schema = result.data.__schema;
              this.showSchemaSidebar = true;
            } else if (result.errors) {
              this.schemaError = result.errors.map((e) => e.message).join(", ");
            } else {
              this.schemaError = "Invalid schema response";
            }
          }
        } else {
          this.schemaError = "No execution ID returned";
        }
      } catch (error) {
        console.error("Failed to load schema:", error);
        this.schemaError = error.message;
      } finally {
        this.loadingSchema = false;
      }
    },

    viewExecutionDetail() {
      if (this.executionId) {
        window.location.href = `/request-detail.html?id=${this.executionId}`;
      }
    },

    async copyToClipboard(text) {
      try {
        await navigator.clipboard.writeText(text);
        const notification = document.createElement("div");
        notification.textContent = "Copied to clipboard!";
        notification.style.cssText =
          "position: fixed; top: 20px; right: 20px; background: #238636; color: white; padding: 0.75rem 1rem; border-radius: 4px; z-index: 10000; font-size: 0.875rem;";
        document.body.appendChild(notification);
        setTimeout(() => notification.remove(), 2000);
      } catch (err) {
        console.error("Failed to copy:", err);
        alert("Failed to copy to clipboard");
      }
    },

    initializeEditors() {
      const queryContainer = this.$refs.queryEditorContainer;
      const variablesContainer = this.$refs.variablesEditorContainer;

      if (queryContainer) {
        this.editorManager.initQueryEditor(
          queryContainer,
          (value) => {
            this.query = value;
          },
          this.query
        );
        
        // Focus the query editor
        if (this.editorManager.queryEditor) {
          this.editorManager.queryEditor.focus();
        }
      }

      if (variablesContainer) {
        this.editorManager.initVariablesEditor(
          variablesContainer,
          (value) => {
            this.variables = value;
          },
          this.variables
        );
      }
    },

    updateEditorSchema() {
      if (this.editorManager && this.schema) {
        this.editorManager.updateSchema(this.schema);
      }
    },

    toggleSection(section) {
      this.expandedSections[section] = !this.expandedSections[section];
    },

    toggleField(key) {
      if (this.expandedFields[key]) {
        delete this.expandedFields[key];
      } else {
        this.expandedFields[key] = true;
      }
    },

    toggleType(typeName) {
      if (this.expandedTypes[typeName]) {
        delete this.expandedTypes[typeName];
      } else {
        this.expandedTypes[typeName] = true;
      }
    },

    isFieldExpanded(key) {
      return !!this.expandedFields[key];
    },

    isTypeExpanded(typeName) {
      return !!this.expandedTypes[typeName];
    },

    getTypeString(type) {
      if (!type) return "Unknown";
      
      let typeStr = "";
      let currentType = type;
      let nonNull = false;
      let isList = false;

      // Unwrap the type structure
      while (currentType) {
        if (currentType.kind === "NON_NULL") {
          nonNull = true;
          currentType = currentType.ofType;
        } else if (currentType.kind === "LIST") {
          isList = true;
          currentType = currentType.ofType;
        } else {
          typeStr = currentType.name || "Unknown";
          break;
        }
      }

      if (isList) {
        typeStr = `[${typeStr}]`;
      }
      if (nonNull) {
        typeStr += "!";
      }

      return typeStr;
    },

    insertFieldIntoQuery(fieldName, args, typeName = "Query") {
      // Generate a query snippet with the field
      let snippet = `${fieldName}`;
      
      if (args && args.length > 0) {
        const argNames = args.map(a => `${a.name}: `).join(", ");
        snippet += `(${argNames})`;
      }
      
      snippet += ` {\n  \n}`;
      
      // Insert into the query editor
      if (this.editorManager && this.editorManager.queryEditor) {
        this.editorManager.insertTextAtCursor(snippet);
      } else {
        // Fallback: append to query
        this.query += `\n${snippet}`;
      }
    },

    getObjectTypes() {
      if (!this.schema) return [];
      return this.schemaTypes.filter(t => 
        !t.name.startsWith('__') && 
        t.kind === 'OBJECT' && 
        t.name !== this.queryType?.name && 
        t.name !== this.mutationType?.name
      );
    },

    getInputTypes() {
      if (!this.schema) return [];
      return this.schemaTypes.filter(t => 
        !t.name.startsWith('__') && 
        t.kind === 'INPUT_OBJECT'
      );
    },

    getEnumTypes() {
      if (!this.schema) return [];
      return this.schemaTypes.filter(t => 
        !t.name.startsWith('__') && 
        t.kind === 'ENUM'
      );
    },

    getScalarTypes() {
      if (!this.schema) return [];
      return this.schemaTypes.filter(t => 
        !t.name.startsWith('__') && 
        t.kind === 'SCALAR'
      );
    },
  },

  watch: {
    schema(newSchema) {
      if (newSchema) {
        this.updateEditorSchema();
      }
    },
  },

  beforeUnmount() {
    if (this.editorManager) {
      this.editorManager.destroy();
    }
  },

  template: `
    <div class="app-container">
      <header class="app-header">
        <div style="display: flex; align-items: center; gap: 1rem">
          <h1 style="margin: 0">🔱 Logseidon</h1>
          <app-nav></app-nav>
        </div>
      </header>

      <div class="main-layout">
        <!-- Schema Sidebar -->
        <aside v-if="showSchemaSidebar" class="sidebar" style="max-width: 350px; overflow-y: auto;">
          <div class="section">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
              <h3 style="margin: 0;">GraphQL Schema</h3>
              <button @click="showSchemaSidebar = false" class="btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;">✕</button>
            </div>

            <div v-if="loadingSchema" style="color: #8b949e; padding: 1rem; text-align: center;">
              Loading schema...
            </div>

            <div v-if="schemaError" class="alert alert-danger" style="display: block; margin-bottom: 1rem; font-size: 0.875rem;">
              {{ schemaError }}
            </div>

            <div v-if="schema && !loadingSchema">
              <!-- Query Type -->
              <div v-if="queryType" class="schema-section" style="margin-bottom: 1rem;">
                <div 
                  @click="toggleSection('queries')" 
                  style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer; padding: 0.5rem; background: #161b22; border-radius: 4px; margin-bottom: 0.5rem;"
                  @mouseover="$event.currentTarget.style.background = '#21262d'"
                  @mouseout="$event.currentTarget.style.background = '#161b22'">
                  <span :style="{ transform: expandedSections.queries ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.2s', display: 'inline-block' }">▶</span>
                  <h4 style="color: #58a6ff; font-size: 0.875rem; margin: 0; text-transform: uppercase; flex: 1;">Queries</h4>
                  <span style="color: #8b949e; font-size: 0.75rem;">{{ schemaTypes.filter(t => t.name === queryType.name)[0]?.fields?.length || 0 }}</span>
                </div>
                <div v-if="expandedSections.queries" v-for="type in schemaTypes.filter(t => t.name === queryType.name)" :key="type.name">
                  <div v-if="type.fields" style="font-size: 0.8rem; margin-left: 0.5rem;">
                    <div v-for="field in type.fields" :key="field.name" style="margin-bottom: 0.5rem;">
                      <div 
                        @click="toggleField('query-' + field.name)" 
                        style="padding: 0.5rem; background: #0d1117; border-radius: 4px; border: 1px solid #30363d; cursor: pointer;"
                        @mouseover="$event.currentTarget.style.borderColor = '#58a6ff'"
                        @mouseout="$event.currentTarget.style.borderColor = '#30363d'">
                        <div style="display: flex; align-items: center; gap: 0.5rem;">
                          <span :style="{ transform: isFieldExpanded('query-' + field.name) ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.2s', display: 'inline-block', fontSize: '0.7rem' }">▶</span>
                          <div style="flex: 1;">
                            <span style="font-weight: 500; color: #79c0ff;">{{ field.name }}</span>
                            <span style="color: #8b949e; font-size: 0.7rem; margin-left: 0.5rem;">: {{ getTypeString(field.type) }}</span>
                          </div>
                          <button 
                            @click.stop="insertFieldIntoQuery(field.name, field.args, 'Query')"
                            style="background: #238636; color: white; border: none; padding: 0.15rem 0.4rem; border-radius: 3px; font-size: 0.65rem; cursor: pointer;"
                            @mouseover="$event.currentTarget.style.background = '#2ea043'"
                            @mouseout="$event.currentTarget.style.background = '#238636'"
                            title="Insert into query">+</button>
                        </div>
                        <div v-if="isFieldExpanded('query-' + field.name)" style="margin-top: 0.5rem; padding-top: 0.5rem; border-top: 1px solid #30363d;">
                          <div v-if="field.description" style="color: #8b949e; font-size: 0.7rem; margin-bottom: 0.5rem; font-style: italic;">{{ field.description }}</div>
                          <div v-if="field.args && field.args.length > 0" style="font-size: 0.7rem;">
                            <div style="color: #8b949e; margin-bottom: 0.25rem; font-weight: 500;">Arguments:</div>
                            <div v-for="arg in field.args" :key="arg.name" style="margin-left: 0.5rem; margin-bottom: 0.25rem;">
                              <span style="color: #a5d6ff;">{{ arg.name }}</span>
                              <span style="color: #8b949e;">: {{ getTypeString(arg.type) }}</span>
                              <div v-if="arg.description" style="color: #6e7681; font-size: 0.65rem; margin-left: 0.5rem;">{{ arg.description }}</div>
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Mutation Type -->
              <div v-if="mutationType" class="schema-section" style="margin-bottom: 1rem;">
                <div 
                  @click="toggleSection('mutations')" 
                  style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer; padding: 0.5rem; background: #161b22; border-radius: 4px; margin-bottom: 0.5rem;"
                  @mouseover="$event.currentTarget.style.background = '#21262d'"
                  @mouseout="$event.currentTarget.style.background = '#161b22'">
                  <span :style="{ transform: expandedSections.mutations ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.2s', display: 'inline-block' }">▶</span>
                  <h4 style="color: #f0883e; font-size: 0.875rem; margin: 0; text-transform: uppercase; flex: 1;">Mutations</h4>
                  <span style="color: #8b949e; font-size: 0.75rem;">{{ schemaTypes.filter(t => t.name === mutationType.name)[0]?.fields?.length || 0 }}</span>
                </div>
                <div v-if="expandedSections.mutations" v-for="type in schemaTypes.filter(t => t.name === mutationType.name)" :key="type.name">
                  <div v-if="type.fields" style="font-size: 0.8rem; margin-left: 0.5rem;">
                    <div v-for="field in type.fields" :key="field.name" style="margin-bottom: 0.5rem;">
                      <div 
                        @click="toggleField('mutation-' + field.name)" 
                        style="padding: 0.5rem; background: #0d1117; border-radius: 4px; border: 1px solid #30363d; cursor: pointer;"
                        @mouseover="$event.currentTarget.style.borderColor = '#f0883e'"
                        @mouseout="$event.currentTarget.style.borderColor = '#30363d'">
                        <div style="display: flex; align-items: center; gap: 0.5rem;">
                          <span :style="{ transform: isFieldExpanded('mutation-' + field.name) ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.2s', display: 'inline-block', fontSize: '0.7rem' }">▶</span>
                          <div style="flex: 1;">
                            <span style="font-weight: 500; color: #f0883e;">{{ field.name }}</span>
                            <span style="color: #8b949e; font-size: 0.7rem; margin-left: 0.5rem;">: {{ getTypeString(field.type) }}</span>
                          </div>
                          <button 
                            @click.stop="insertFieldIntoQuery(field.name, field.args, 'Mutation')"
                            style="background: #da3633; color: white; border: none; padding: 0.15rem 0.4rem; border-radius: 3px; font-size: 0.65rem; cursor: pointer;"
                            @mouseover="$event.currentTarget.style.background = '#f85149'"
                            @mouseout="$event.currentTarget.style.background = '#da3633'"
                            title="Insert into query">+</button>
                        </div>
                        <div v-if="isFieldExpanded('mutation-' + field.name)" style="margin-top: 0.5rem; padding-top: 0.5rem; border-top: 1px solid #30363d;">
                          <div v-if="field.description" style="color: #8b949e; font-size: 0.7rem; margin-bottom: 0.5rem; font-style: italic;">{{ field.description }}</div>
                          <div v-if="field.args && field.args.length > 0" style="font-size: 0.7rem;">
                            <div style="color: #8b949e; margin-bottom: 0.25rem; font-weight: 500;">Arguments:</div>
                            <div v-for="arg in field.args" :key="arg.name" style="margin-left: 0.5rem; margin-bottom: 0.25rem;">
                              <span style="color: #a5d6ff;">{{ arg.name }}</span>
                              <span style="color: #8b949e;">: {{ getTypeString(arg.type) }}</span>
                              <div v-if="arg.description" style="color: #6e7681; font-size: 0.65rem; margin-left: 0.5rem;">{{ arg.description }}</div>
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Types -->
              <div class="schema-section">
                <div 
                  @click="toggleSection('types')" 
                  style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer; padding: 0.5rem; background: #161b22; border-radius: 4px; margin-bottom: 0.5rem;"
                  @mouseover="$event.currentTarget.style.background = '#21262d'"
                  @mouseout="$event.currentTarget.style.background = '#161b22'">
                  <span :style="{ transform: expandedSections.types ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.2s', display: 'inline-block' }">▶</span>
                  <h4 style="color: #8b949e; font-size: 0.875rem; margin: 0; text-transform: uppercase; flex: 1;">Types</h4>
                  <span style="color: #8b949e; font-size: 0.75rem;">{{ getObjectTypes().length }}</span>
                </div>
                <div v-if="expandedSections.types" style="max-height: 400px; overflow-y: auto; margin-left: 0.5rem;">
                  <div v-for="type in getObjectTypes()" :key="type.name" style="margin-bottom: 0.5rem;">
                    <div 
                      @click="toggleType(type.name)" 
                      style="padding: 0.5rem; background: #0d1117; border-radius: 4px; border: 1px solid #30363d; cursor: pointer;"
                      @mouseover="$event.currentTarget.style.borderColor = '#8b949e'"
                      @mouseout="$event.currentTarget.style.borderColor = '#30363d'">
                      <div style="display: flex; align-items: center; gap: 0.5rem;">
                        <span v-if="type.fields && type.fields.length > 0" :style="{ transform: isTypeExpanded(type.name) ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.2s', display: 'inline-block', fontSize: '0.7rem' }">▶</span>
                        <span v-else style="width: 0.7rem; display: inline-block;"></span>
                        <span style="font-weight: 500; color: #c9d1d9; flex: 1; font-size: 0.75rem;">{{ type.name }}</span>
                        <span v-if="type.fields" style="color: #8b949e; font-size: 0.65rem;">{{ type.fields.length }} fields</span>
                      </div>
                      <div v-if="isTypeExpanded(type.name) && type.fields" style="margin-top: 0.5rem; padding-top: 0.5rem; border-top: 1px solid #30363d;">
                        <div v-if="type.description" style="color: #8b949e; font-size: 0.7rem; margin-bottom: 0.5rem; font-style: italic;">{{ type.description }}</div>
                        <div v-for="field in type.fields" :key="field.name" style="margin-bottom: 0.25rem; font-size: 0.7rem; margin-left: 0.5rem;">
                          <span style="color: #a5d6ff;">{{ field.name }}</span>
                          <span style="color: #8b949e;">: {{ getTypeString(field.type) }}</span>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </aside>

        <main class="content" style="margin: 0; padding: 2rem;">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem;">
            <h2 style="margin: 0;">GraphQL Explorer</h2>
            <div style="display: flex; gap: 0.5rem;">
              <button @click="showSampleQueries = !showSampleQueries" class="btn-secondary">
                {{ showSampleQueries ? 'Hide' : 'Load' }} Sample Queries
              </button>
              <button 
                @click="loadGraphQLSchema" 
                :disabled="!canLoadSchema || loadingSchema" 
                class="btn-secondary"
                :style="{ opacity: !canLoadSchema || loadingSchema ? 0.5 : 1 }">
                {{ loadingSchema ? 'Loading...' : '📖 Schema' }}
              </button>
              <button @click="clearQuery" class="btn-secondary">Clear</button>
              <button 
                @click="executeQuery" 
                :disabled="!canExecute || executing" 
                class="btn-primary"
                :style="{ opacity: !canExecute || executing ? 0.5 : 1 }">
                {{ executing ? 'Executing...' : '▶ Execute' }}
              </button>
            </div>
          </div>

          <!-- Sample Queries Panel -->
          <div v-if="showSampleQueries" class="modal-section" style="margin-bottom: 1rem;">
            <h4>Sample Queries</h4>
            <div v-if="sampleQueries.length === 0" style="color: #8b949e;">
              No sample queries available. Create one from the Requests page.
            </div>
            <div v-else style="display: grid; grid-template-columns: repeat(auto-fill, minmax(250px, 1fr)); gap: 0.75rem;">
              <div 
                v-for="sq in sampleQueries" 
                :key="sq.id"
                @click="loadSampleQuery(sq)"
                style="background: #161b22; border: 1px solid #30363d; border-radius: 4px; padding: 0.75rem; cursor: pointer; transition: border-color 0.2s;"
                @mouseover="$event.currentTarget.style.borderColor = '#58a6ff'"
                @mouseout="$event.currentTarget.style.borderColor = '#30363d'">
                <div style="font-weight: 500; margin-bottom: 0.25rem;">{{ sq.name }}</div>
                <div style="font-size: 0.75rem; color: #8b949e;">{{ sq.server?.url || 'No server' }}</div>
              </div>
            </div>
          </div>

          <!-- Configuration Section -->
          <div class="modal-section" style="margin-bottom: 1rem;">
            <div class="form-group" style="margin-bottom: 0;">
              <label for="serverSelect">Server:</label>
              <select id="serverSelect" v-model="selectedServerId" style="width: 100%; padding: 0.5rem; background: #0d1117; border: 1px solid #30363d; border-radius: 4px; color: #c9d1d9;">
                <option value="">-- Select Server --</option>
                <option v-for="server in servers" :key="server.id" :value="server.id">
                  {{ server.name }} ({{ server.url }})
                </option>
              </select>
            </div>
          </div>

          <!-- Query Editor -->
          <div class="modal-section" style="margin-bottom: 1rem;">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem;">
              <h4 style="margin: 0;">GraphQL Query</h4>
              <button @click="copyToClipboard(query)" class="btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;">📋 Copy</button>
            </div>
            <div class="form-group" style="margin-bottom: 0.5rem;">
              <label for="operationName">Operation Name (optional):</label>
              <input 
                type="text" 
                id="operationName" 
                v-model="operationName" 
                placeholder="e.g., FetchUsers"
                style="width: 100%; padding: 0.5rem; background: #0d1117; border: 1px solid #30363d; border-radius: 4px; color: #c9d1d9; font-family: monospace;" />
            </div>
            <div class="graphql-editor-container" ref="queryEditorContainer"></div>
          </div>

          <!-- Variables Editor -->
          <div class="modal-section" style="margin-bottom: 1rem;">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem;">
              <h4 style="margin: 0;">Variables (JSON)</h4>
              <button @click="copyToClipboard(variables)" class="btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;">📋 Copy</button>
            </div>
            <div class="variables-editor-container" ref="variablesEditorContainer"></div>
          </div>

          <!-- Response Section - Always visible after execution -->
          <div v-if="error || result || executionId" class="modal-section">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem;">
              <h4 style="margin: 0;">Response</h4>
              <div style="display: flex; gap: 0.5rem;">
                <button v-if="executionId" @click="viewExecutionDetail" class="btn-primary" style="padding: 0.35rem 0.75rem; font-size: 0.875rem;">
                  View Full Details →
                </button>
                <button v-if="result" @click="copyToClipboard(formattedResult)" class="btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;">📋 Copy</button>
              </div>
            </div>
            
            <div v-if="error" class="alert alert-danger" style="display: block; margin-bottom: 1rem;">
              {{ error }}
            </div>
            
            <div v-if="result">
              <pre class="json-display" style="max-height: 500px; overflow: auto;">{{ formattedResult }}</pre>
            </div>

            <div v-if="!result && !error && executionId" style="color: #8b949e; padding: 1rem; text-align: center;">
              Execution started. Click "View Full Details" to see logs and SQL queries.
            </div>
          </div>
        </main>
      </div>
    </div>
  `,
});

// Register components
app.component("app-nav", createNavigation("graphql-explorer"));

app.mount("#app");
