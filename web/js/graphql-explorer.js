import { createAppHeader } from "./shared/navigation.js";
import { API } from "./shared/api.js";
import { GraphQLEditorManager } from "./graphql-editor-manager.js";
import { createLogStreamComponent } from "./shared/log-stream-component.js";
import { loadTemplate } from "./shared/template-loader.js";

const { createApp } = Vue;

// Load template
const template = await loadTemplate("/templates/graphql-explorer-main.html");

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
      requestIdHeader: null, // Request ID for log filtering
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
      schemaFilter: "", // Filter text for schema sidebar
      showLogs: false, // Show/hide log panel during execution
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

    filteredQueryFields() {
      if (!this.schema || !this.queryType) return [];
      const queryTypeObj = this.schemaTypes.find(
        (t) => t.name === this.queryType.name
      );
      if (!queryTypeObj || !queryTypeObj.fields) return [];

      if (!this.schemaFilter) return queryTypeObj.fields;

      const filterLower = this.schemaFilter.toLowerCase();
      return queryTypeObj.fields.filter(
        (field) =>
          field.name.toLowerCase().includes(filterLower) ||
          (field.description &&
            field.description.toLowerCase().includes(filterLower))
      );
    },

    filteredMutationFields() {
      if (!this.schema || !this.mutationType) return [];
      const mutationTypeObj = this.schemaTypes.find(
        (t) => t.name === this.mutationType.name
      );
      if (!mutationTypeObj || !mutationTypeObj.fields) return [];

      if (!this.schemaFilter) return mutationTypeObj.fields;

      const filterLower = this.schemaFilter.toLowerCase();
      return mutationTypeObj.fields.filter(
        (field) =>
          field.name.toLowerCase().includes(filterLower) ||
          (field.description &&
            field.description.toLowerCase().includes(filterLower))
      );
    },

    filteredObjectTypes() {
      const objectTypes = this.getObjectTypes();
      if (!this.schemaFilter) return objectTypes;

      const filterLower = this.schemaFilter.toLowerCase();
      return objectTypes.filter(
        (type) =>
          type.name.toLowerCase().includes(filterLower) ||
          (type.description &&
            type.description.toLowerCase().includes(filterLower))
      );
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
      this.requestIdHeader = null;
      this.showLogs = true; // Show log panel when execution starts

      // Clear logs in the log stream component
      if (this.$refs.logStream) {
        this.$refs.logStream.clearLogs();
      }

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

        // Execute via API with async flag
        const payload = {
          serverId: parseInt(this.selectedServerId),
          requestData: JSON.stringify(requestData),
          sync: false,
        };

        const response = await API.post("/api/execute", payload);

        if (response.executionId) {
          this.executionId = response.executionId;

          // Poll for result
          await this.pollForResult(response.executionId);
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

    async pollForResult(executionId, maxAttempts = 30, intervalMs = 1000) {
      for (let attempt = 0; attempt < maxAttempts; attempt++) {
        try {
          const execution = await API.get(`/api/executions/${executionId}`);

          console.log("execution", execution);

          // Extract request ID header for log filtering
          if (execution.execution.requestIdHeader && !this.requestIdHeader) {
            this.requestIdHeader = execution.execution.requestIdHeader;
            console.log(
              "Set requestIdHeader for log filtering:",
              this.requestIdHeader
            );
          }

          if (execution.execution.responseBody) {
            this.result = execution.execution.responseBody;
            return;
          }

          if (execution.execution.error) {
            this.error = execution.execution.error;
            return;
          }

          // Wait before next attempt
          await new Promise((resolve) => setTimeout(resolve, intervalMs));
        } catch (error) {
          console.error("Polling error:", error);
          // Continue polling on error
        }
      }

      this.error = "Timeout waiting for execution result";
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

        // Execute via API with sync flag
        const payload = {
          serverId: parseInt(this.selectedServerId),
          requestData: JSON.stringify(introspectionQuery),
          sync: true,
        };

        const response = await API.post("/api/execute", payload);

        if (response.executionId) {
          // Response is returned synchronously
          if (response.error) {
            this.schemaError = response.error;
          } else {
            const result = JSON.parse(response.responseBody);
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

    insertFieldIntoQuery(fieldName, args, typeName = "Query", returnType) {
      // Generate a complete query structure with operation name, variables, and return types
      const operationType = typeName.toLowerCase();
      const operationName = this.capitalize(fieldName);

      // Build variables declaration
      let variablesDecl = "";
      let variablesObj = {};
      let fieldArgs = "";

      if (args && args.length > 0) {
        const varDecls = [];
        const argPairs = [];

        args.forEach((arg) => {
          const varName = arg.name;
          const varType = this.getTypeString(arg.type);
          varDecls.push(`$${varName}: ${varType}`);
          argPairs.push(`${arg.name}: $${varName}`);

          // Generate example value for variables
          variablesObj[varName] = this.getExampleValue(arg.type);
        });

        variablesDecl = `(${varDecls.join(", ")})`;
        fieldArgs = `(${argPairs.join(", ")})`;
      }

      // Get fields for the return type
      const returnFields = this.getFieldsForType(returnType);

      // Build the complete query
      let snippet = `${operationType} ${operationName}${variablesDecl} {\n`;
      snippet += `  ${fieldName}${fieldArgs} {\n`;
      snippet += returnFields;
      snippet += `  }\n`;
      snippet += `}`;

      // Update variables if we have them
      if (Object.keys(variablesObj).length > 0) {
        this.variables = JSON.stringify(variablesObj, null, 2);
        if (this.editorManager) {
          this.editorManager.setVariablesValue(this.variables);
        }
      }

      // Insert into the query editor
      if (this.editorManager && this.editorManager.queryEditor) {
        this.editorManager.setQueryValue(snippet);
        this.query = snippet;
      } else {
        // Fallback: replace query
        this.query = snippet;
      }
    },

    capitalize(str) {
      return str.charAt(0).toUpperCase() + str.slice(1);
    },

    getExampleValue(type) {
      // Unwrap type to get the base type
      let baseType = type;
      let isList = false;

      while (baseType && baseType.ofType) {
        if (baseType.kind === "LIST") {
          isList = true;
        }
        baseType = baseType.ofType;
      }

      const typeName = baseType?.name || "String";
      const typeKind = baseType?.kind;

      // Handle INPUT_OBJECT types by looking up their fields
      if (typeKind === "INPUT_OBJECT") {
        const inputType = this.schemaTypes.find((t) => t.name === typeName);
        if (inputType && inputType.inputFields) {
          const inputObj = {};
          inputType.inputFields.forEach((field) => {
            inputObj[field.name] = this.getExampleValue(field.type);
          });
          return isList ? [inputObj] : inputObj;
        }
        return isList ? [{}] : {};
      }

      // Handle scalar types
      let scalarValue;
      switch (typeName) {
        case "ID":
          scalarValue = "1";
          break;
        case "Int":
          scalarValue = 0;
          break;
        case "Float":
          scalarValue = 0.0;
          break;
        case "Boolean":
          scalarValue = false;
          break;
        case "String":
        default:
          scalarValue = "";
          break;
      }

      return isList ? [scalarValue] : scalarValue;
    },

    getFieldsForType(type) {
      if (!type || !this.schema) return "    # Add fields here\n";

      // Unwrap type to get the actual type name
      let actualType = type;
      while (actualType && actualType.ofType) {
        actualType = actualType.ofType;
      }

      const typeName = actualType?.name;
      if (!typeName) return "    # Add fields here\n";

      // Find the type in schema
      const typeObj = this.schemaTypes.find((t) => t.name === typeName);
      if (!typeObj || !typeObj.fields) return "    # Add fields here\n";

      // Get scalar fields (avoid nested objects to keep it simple)
      const scalarFields = typeObj.fields.filter((f) => {
        const fieldType = this.getBaseType(f.type);
        return this.isScalarType(fieldType);
      });

      if (scalarFields.length === 0) {
        // If no scalar fields, just add id if available, or a comment
        const idField = typeObj.fields.find((f) => f.name === "id");
        if (idField) {
          return "    id\n";
        }
        return "    # Add fields here\n";
      }

      // Return up to 5 scalar fields
      return (
        scalarFields
          .slice(0, 5)
          .map((f) => `    ${f.name}`)
          .join("\n") + "\n"
      );
    },

    getBaseType(type) {
      let baseType = type;
      while (baseType && baseType.ofType) {
        baseType = baseType.ofType;
      }
      return baseType;
    },

    isScalarType(type) {
      if (!type || !type.name) return false;
      const scalarTypes = ["ID", "String", "Int", "Float", "Boolean"];
      return (
        scalarTypes.includes(type.name) ||
        type.kind === "SCALAR" ||
        type.kind === "ENUM"
      );
    },

    getReturnTypeFields(type) {
      if (!type || !this.schema) return [];

      // Unwrap type to get the actual type name
      let actualType = type;
      while (actualType && actualType.ofType) {
        actualType = actualType.ofType;
      }

      const typeName = actualType?.name;
      if (!typeName) return [];

      // Find the type in schema
      const typeObj = this.schemaTypes.find((t) => t.name === typeName);
      if (!typeObj || !typeObj.fields) return [];

      // Return all fields (not just scalar ones, to show full structure)
      return typeObj.fields;
    },

    getObjectTypes() {
      if (!this.schema) return [];
      return this.schemaTypes.filter(
        (t) =>
          !t.name.startsWith("__") &&
          t.kind === "OBJECT" &&
          t.name !== this.queryType?.name &&
          t.name !== this.mutationType?.name
      );
    },

    getInputTypes() {
      if (!this.schema) return [];
      return this.schemaTypes.filter(
        (t) => !t.name.startsWith("__") && t.kind === "INPUT_OBJECT"
      );
    },

    getEnumTypes() {
      if (!this.schema) return [];
      return this.schemaTypes.filter(
        (t) => !t.name.startsWith("__") && t.kind === "ENUM"
      );
    },

    getScalarTypes() {
      if (!this.schema) return [];
      return this.schemaTypes.filter(
        (t) => !t.name.startsWith("__") && t.kind === "SCALAR"
      );
    },

    handleLogClick(log) {
      console.log("Log clicked:", log);
      // Could open a modal with log details if needed
    },

    toggleLogs() {
      this.showLogs = !this.showLogs;
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

  template,
});

app.component("app-header", createAppHeader("graphql-explorer"));
app.component("log-stream", createLogStreamComponent());

app.mount("#app");
