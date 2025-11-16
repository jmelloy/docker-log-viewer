<template>
<div class="app-container">
  <app-header activePage="graphql-explorer"></app-header>

  <div class="main-layout">
    <!-- Schema Sidebar -->
    <aside v-if="showSchemaSidebar" class="sidebar sidebar-schema">
      <div class="section">
        <div class="flex-between mb-1">
          <h3 class="m-0">GraphQL Schema</h3>
          <button @click="showSchemaSidebar = false" class="btn-secondary btn-sm">✕</button>
        </div>

        <div v-if="loadingSchema" class="text-muted text-center p-1">Loading schema...</div>

        <div v-if="schemaError" class="alert alert-danger mb-1">{{ schemaError }}</div>

        <div v-if="schema && !loadingSchema" class="mb-1">
          <input
            v-model="schemaFilter"
            type="text"
            placeholder="Filter schema..."
            style="
              width: 100%;
              padding: 0.5rem;
              background: #0d1117;
              border: 1px solid #30363d;
              border-radius: 4px;
              color: #c9d1d9;
              font-size: 0.875rem;
            "
            @focus="$event.target.style.borderColor = '#58a6ff'"
            @blur="$event.target.style.borderColor = '#30363d'"
          />
        </div>

        <div v-if="schema && !loadingSchema">
          <!-- Query Type -->
          <div v-if="queryType" class="schema-section" style="margin-bottom: 1rem">
            <div
              @click="toggleSection('queries')"
              style="
                display: flex;
                align-items: center;
                gap: 0.5rem;
                cursor: pointer;
                padding: 0.5rem;
                background: #161b22;
                border-radius: 4px;
                margin-bottom: 0.5rem;
              "
              @mouseover="$event.currentTarget.style.background = '#21262d'"
              @mouseout="$event.currentTarget.style.background = '#161b22'"
            >
              <span
                :style="{ transform: expandedSections.queries ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.2s', display: 'inline-block' }"
                >▶</span
              >
              <h4 style="color: #58a6ff; font-size: 0.875rem; margin: 0; text-transform: uppercase; flex: 1">
                Queries
              </h4>
              <span style="color: #8b949e; font-size: 0.75rem">{{ filteredQueryFields.length }}</span>
            </div>
            <div v-if="expandedSections.queries" style="font-size: 0.8rem; margin-left: 0.5rem">
              <div v-for="field in filteredQueryFields" :key="field.name" style="margin-bottom: 0.5rem">
                <div
                  @click="toggleField('query-' + field.name)"
                  style="
                    padding: 0.5rem;
                    background: #0d1117;
                    border-radius: 4px;
                    border: 1px solid #30363d;
                    cursor: pointer;
                  "
                  @mouseover="$event.currentTarget.style.borderColor = '#58a6ff'"
                  @mouseout="$event.currentTarget.style.borderColor = '#30363d'"
                >
                  <div style="display: flex; align-items: center; gap: 0.5rem">
                    <span
                      :style="{ transform: isFieldExpanded('query-' + field.name) ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.2s', display: 'inline-block', fontSize: '0.7rem' }"
                      >▶</span
                    >
                    <div style="flex: 1">
                      <span style="font-weight: 500; color: #79c0ff">{{ field.name }}</span>
                      <span style="color: #8b949e; font-size: 0.7rem; margin-left: 0.5rem"
                        >: {{ getTypeString(field.type) }}</span
                      >
                    </div>
                    <button
                      @click.stop="insertFieldIntoQuery(field.name, field.args, 'Query', field.type)"
                      style="
                        background: #238636;
                        color: white;
                        border: none;
                        padding: 0.15rem 0.4rem;
                        border-radius: 3px;
                        font-size: 0.65rem;
                        cursor: pointer;
                      "
                      @mouseover="$event.currentTarget.style.background = '#2ea043'"
                      @mouseout="$event.currentTarget.style.background = '#238636'"
                      title="Insert into query"
                    >
                      +
                    </button>
                  </div>
                  <div
                    v-if="isFieldExpanded('query-' + field.name)"
                    style="margin-top: 0.5rem; padding-top: 0.5rem; border-top: 1px solid #30363d"
                  >
                    <div
                      v-if="field.description"
                      style="color: #8b949e; font-size: 0.7rem; margin-bottom: 0.5rem; font-style: italic"
                    >
                      {{ field.description }}
                    </div>
                    <div v-if="field.args && field.args.length > 0" style="font-size: 0.7rem; margin-bottom: 0.5rem">
                      <div style="color: #8b949e; margin-bottom: 0.25rem; font-weight: 500">Arguments:</div>
                      <div
                        v-for="arg in field.args"
                        :key="arg.name"
                        style="margin-left: 0.5rem; margin-bottom: 0.25rem"
                      >
                        <span style="color: #a5d6ff">{{ arg.name }}</span>
                        <span style="color: #8b949e">: {{ getTypeString(arg.type) }}</span>
                        <div v-if="arg.description" style="color: #6e7681; font-size: 0.65rem; margin-left: 0.5rem">
                          {{ arg.description }}
                        </div>
                      </div>
                    </div>
                    <div v-if="field.type" style="font-size: 0.7rem">
                      <div style="color: #8b949e; margin-bottom: 0.25rem; font-weight: 500">Returns:</div>
                      <div style="margin-left: 0.5rem">
                        <div style="margin-bottom: 0.25rem">
                          <span style="color: #79c0ff">{{ getTypeString(field.type) }}</span>
                        </div>
                        <div
                          v-if="getReturnTypeFields(field.type).length > 0"
                          style="margin-top: 0.5rem; padding: 0.5rem; background: #161b22; border-radius: 3px"
                        >
                          <div style="color: #8b949e; font-size: 0.65rem; margin-bottom: 0.25rem">Fields:</div>
                          <div
                            v-for="returnField in getReturnTypeFields(field.type)"
                            :key="returnField.name"
                            style="margin-left: 0.5rem; margin-bottom: 0.15rem; font-size: 0.65rem"
                          >
                            <span style="color: #a5d6ff">{{ returnField.name }}</span>
                            <span style="color: #8b949e">: {{ getTypeString(returnField.type) }}</span>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Mutation Type -->
            <div v-if="mutationType" class="schema-section" style="margin-bottom: 1rem">
              <div
                @click="toggleSection('mutations')"
                style="
                  display: flex;
                  align-items: center;
                  gap: 0.5rem;
                  cursor: pointer;
                  padding: 0.5rem;
                  background: #161b22;
                  border-radius: 4px;
                  margin-bottom: 0.5rem;
                "
                @mouseover="$event.currentTarget.style.background = '#21262d'"
                @mouseout="$event.currentTarget.style.background = '#161b22'"
              >
                <span
                  :style="{ transform: expandedSections.mutations ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.2s', display: 'inline-block' }"
                  >▶</span
                >
                <h4 style="color: #f0883e; font-size: 0.875rem; margin: 0; text-transform: uppercase; flex: 1">
                  Mutations
                </h4>
                <span style="color: #8b949e; font-size: 0.75rem">{{ filteredMutationFields.length }}</span>
              </div>
              <div v-if="expandedSections.mutations" style="font-size: 0.8rem; margin-left: 0.5rem">
                <div v-for="field in filteredMutationFields" :key="field.name" style="margin-bottom: 0.5rem">
                  <div
                    @click="toggleField('mutation-' + field.name)"
                    style="
                      padding: 0.5rem;
                      background: #0d1117;
                      border-radius: 4px;
                      border: 1px solid #30363d;
                      cursor: pointer;
                    "
                    @mouseover="$event.currentTarget.style.borderColor = '#f0883e'"
                    @mouseout="$event.currentTarget.style.borderColor = '#30363d'"
                  >
                    <div style="display: flex; align-items: center; gap: 0.5rem">
                      <span
                        :style="{ transform: isFieldExpanded('mutation-' + field.name) ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.2s', display: 'inline-block', fontSize: '0.7rem' }"
                        >▶</span
                      >
                      <div style="flex: 1">
                        <span style="font-weight: 500; color: #f0883e">{{ field.name }}</span>
                        <span style="color: #8b949e; font-size: 0.7rem; margin-left: 0.5rem"
                          >: {{ getTypeString(field.type) }}</span
                        >
                      </div>
                      <button
                        @click.stop="insertFieldIntoQuery(field.name, field.args, 'Mutation', field.type)"
                        style="
                          background: #da3633;
                          color: white;
                          border: none;
                          padding: 0.15rem 0.4rem;
                          border-radius: 3px;
                          font-size: 0.65rem;
                          cursor: pointer;
                        "
                        @mouseover="$event.currentTarget.style.background = '#f85149'"
                        @mouseout="$event.currentTarget.style.background = '#da3633'"
                        title="Insert into query"
                      >
                        +
                      </button>
                    </div>
                    <div
                      v-if="isFieldExpanded('mutation-' + field.name)"
                      style="margin-top: 0.5rem; padding-top: 0.5rem; border-top: 1px solid #30363d"
                    >
                      <div
                        v-if="field.description"
                        style="color: #8b949e; font-size: 0.7rem; margin-bottom: 0.5rem; font-style: italic"
                      >
                        {{ field.description }}
                      </div>
                      <div v-if="field.args && field.args.length > 0" style="font-size: 0.7rem; margin-bottom: 0.5rem">
                        <div style="color: #8b949e; margin-bottom: 0.25rem; font-weight: 500">Arguments:</div>
                        <div
                          v-for="arg in field.args"
                          :key="arg.name"
                          style="margin-left: 0.5rem; margin-bottom: 0.25rem"
                        >
                          <span style="color: #a5d6ff">{{ arg.name }}</span>
                          <span style="color: #8b949e">: {{ getTypeString(arg.type) }}</span>
                          <div v-if="arg.description" style="color: #6e7681; font-size: 0.65rem; margin-left: 0.5rem">
                            {{ arg.description }}
                          </div>
                        </div>
                      </div>
                      <div v-if="field.type" style="font-size: 0.7rem">
                        <div style="color: #8b949e; margin-bottom: 0.25rem; font-weight: 500">Returns:</div>
                        <div style="margin-left: 0.5rem">
                          <div style="margin-bottom: 0.25rem">
                            <span style="color: #f0883e">{{ getTypeString(field.type) }}</span>
                          </div>
                          <div
                            v-if="getReturnTypeFields(field.type).length > 0"
                            style="margin-top: 0.5rem; padding: 0.5rem; background: #161b22; border-radius: 3px"
                          >
                            <div style="color: #8b949e; font-size: 0.65rem; margin-bottom: 0.25rem">Fields:</div>
                            <div
                              v-for="returnField in getReturnTypeFields(field.type)"
                              :key="returnField.name"
                              style="margin-left: 0.5rem; margin-bottom: 0.15rem; font-size: 0.65rem"
                            >
                              <span style="color: #a5d6ff">{{ returnField.name }}</span>
                              <span style="color: #8b949e">: {{ getTypeString(returnField.type) }}</span>
                            </div>
                          </div>
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
              style="
                display: flex;
                align-items: center;
                gap: 0.5rem;
                cursor: pointer;
                padding: 0.5rem;
                background: #161b22;
                border-radius: 4px;
                margin-bottom: 0.5rem;
              "
              @mouseover="$event.currentTarget.style.background = '#21262d'"
              @mouseout="$event.currentTarget.style.background = '#161b22'"
            >
              <span
                :style="{ transform: expandedSections.types ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.2s', display: 'inline-block' }"
                >▶</span
              >
              <h4 style="color: #8b949e; font-size: 0.875rem; margin: 0; text-transform: uppercase; flex: 1">Types</h4>
              <span style="color: #8b949e; font-size: 0.75rem">{{ filteredObjectTypes.length }}</span>
            </div>
            <div v-if="expandedSections.types" style="max-height: 400px; overflow-y: auto; margin-left: 0.5rem">
              <div v-for="type in filteredObjectTypes" :key="type.name" style="margin-bottom: 0.5rem">
                <div
                  @click="toggleType(type.name)"
                  style="
                    padding: 0.5rem;
                    background: #0d1117;
                    border-radius: 4px;
                    border: 1px solid #30363d;
                    cursor: pointer;
                  "
                  @mouseover="$event.currentTarget.style.borderColor = '#8b949e'"
                  @mouseout="$event.currentTarget.style.borderColor = '#30363d'"
                >
                  <div style="display: flex; align-items: center; gap: 0.5rem">
                    <span
                      v-if="type.fields && type.fields.length > 0"
                      :style="{ transform: isTypeExpanded(type.name) ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.2s', display: 'inline-block', fontSize: '0.7rem' }"
                      >▶</span
                    >
                    <span v-else style="width: 0.7rem; display: inline-block"></span>
                    <span style="font-weight: 500; color: #c9d1d9; flex: 1; font-size: 0.75rem">{{ type.name }}</span>
                    <span v-if="type.fields" style="color: #8b949e; font-size: 0.65rem"
                      >{{ type.fields.length }} fields</span
                    >
                  </div>
                  <div
                    v-if="isTypeExpanded(type.name) && type.fields"
                    style="margin-top: 0.5rem; padding-top: 0.5rem; border-top: 1px solid #30363d"
                  >
                    <div
                      v-if="type.description"
                      style="color: #8b949e; font-size: 0.7rem; margin-bottom: 0.5rem; font-style: italic"
                    >
                      {{ type.description }}
                    </div>
                    <div
                      v-for="field in type.fields"
                      :key="field.name"
                      style="margin-bottom: 0.25rem; font-size: 0.7rem; margin-left: 0.5rem"
                    >
                      <span style="color: #a5d6ff">{{ field.name }}</span>
                      <span style="color: #8b949e">: {{ getTypeString(field.type) }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </aside>

    <main class="content" style="margin: 0; padding: 2rem">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem">
        <h2 style="margin: 0">GraphQL Explorer</h2>
        <div style="display: flex; gap: 0.5rem">
          <button @click="showSampleQueries = !showSampleQueries" class="btn-secondary">
            {{ showSampleQueries ? 'Hide' : 'Load' }} Sample Queries
          </button>
          <button
            @click="loadGraphQLSchema"
            :disabled="!canLoadSchema || loadingSchema"
            class="btn-secondary"
            :style="{ opacity: !canLoadSchema || loadingSchema ? 0.5 : 1 }"
          >
            {{ loadingSchema ? 'Loading...' : '📖 Schema' }}
          </button>
          <button @click="clearQuery" class="btn-secondary">Clear</button>
          <button
            @click="executeQuery"
            :disabled="!canExecute || executing"
            class="btn-primary"
            :style="{ opacity: !canExecute || executing ? 0.5 : 1 }"
          >
            {{ executing ? 'Executing...' : '▶ Execute' }}
          </button>
        </div>
      </div>

      <!-- Sample Queries Panel -->
      <div v-if="showSampleQueries" class="modal-section" style="margin-bottom: 1rem">
        <h4>Sample Queries</h4>
        <div v-if="sampleQueries.length === 0" style="color: #8b949e">
          No sample queries available. Create one from the Requests page.
        </div>
        <div v-else style="display: grid; grid-template-columns: repeat(auto-fill, minmax(250px, 1fr)); gap: 0.75rem">
          <div
            v-for="sq in sampleQueries"
            :key="sq.id"
            @click="loadSampleQuery(sq)"
            style="
              background: #161b22;
              border: 1px solid #30363d;
              border-radius: 4px;
              padding: 0.75rem;
              cursor: pointer;
              transition: border-color 0.2s;
            "
            @mouseover="$event.currentTarget.style.borderColor = '#58a6ff'"
            @mouseout="$event.currentTarget.style.borderColor = '#30363d'"
          >
            <div style="font-weight: 500; margin-bottom: 0.25rem">{{ sq.name }}</div>
            <div style="font-size: 0.75rem; color: #8b949e">{{ sq.server?.url || 'No server' }}</div>
          </div>
        </div>
      </div>

      <!-- Configuration Section -->
      <div class="modal-section" style="margin-bottom: 1rem">
        <div class="form-group" style="margin-bottom: 0">
          <label for="serverSelect">Server:</label>
          <select
            id="serverSelect"
            v-model="selectedServerId"
            style="
              width: 100%;
              padding: 0.5rem;
              background: #0d1117;
              border: 1px solid #30363d;
              border-radius: 4px;
              color: #c9d1d9;
            "
          >
            <option value="">-- Select Server --</option>
            <option v-for="server in servers" :key="server.id" :value="server.id">
              {{ server.name }} ({{ server.url }})
            </option>
          </select>
        </div>
      </div>

      <!-- Two-column layout for request and response -->
      <div style="display: grid; grid-template-columns: 50% 50%; gap: 1rem; margin-bottom: 1rem">
        <!-- Left Column: Request -->
        <div style="min-width: 0; overflow: hidden">
          <!-- Query Editor -->
          <div class="modal-section" style="margin-bottom: 1rem">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem">
              <h4 style="margin: 0">GraphQL Query</h4>
              <button
                @click="copyToClipboard(query)"
                class="btn-secondary"
                style="padding: 0.25rem 0.5rem; font-size: 0.75rem"
              >
                📋 Copy
              </button>
            </div>
            <div class="form-group" style="margin-bottom: 0.5rem">
              <!-- <label for="operationName">Operation Name (optional):</label> -->
              <input
                type="text"
                id="operationName"
                v-model="operationName"
                placeholder="e.g., FetchUsers"
                style="
                  width: 100%;
                  padding: 0.5rem;
                  background: #0d1117;
                  border: 1px solid #30363d;
                  border-radius: 4px;
                  color: #c9d1d9;
                  font-family: monospace;
                "
              />
            </div>
            <div class="graphql-editor-container" ref="queryEditorContainer"></div>
          </div>

          <!-- Variables Editor -->
          <div class="modal-section" style="margin-bottom: 1rem">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem">
              <h4 style="margin: 0">Variables (JSON)</h4>
              <button
                @click="copyToClipboard(variables)"
                class="btn-secondary"
                style="padding: 0.25rem 0.5rem; font-size: 0.75rem"
              >
                📋 Copy
              </button>
            </div>
            <div class="variables-editor-container" ref="variablesEditorContainer"></div>
          </div>
        </div>

        <!-- Right Column: Response (Always visible) -->
        <div class="modal-section" style="min-width: 0; overflow: hidden">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem">
            <h4 style="margin: 0">Response</h4>
            <div style="display: flex; gap: 0.5rem">
              <button
                v-if="executionId"
                @click="viewExecutionDetail"
                class="btn-secondary"
                style="padding: 0.35rem 0.75rem; font-size: 0.875rem"
              >
                View Full Details →
              </button>
              <button
                @click="copyToClipboard(formattedResult)"
                class="btn-secondary"
                style="padding: 0.25rem 0.5rem; font-size: 0.75rem"
                :disabled="!result"
              >
                📋 Copy
              </button>
            </div>
          </div>

          <!-- Executing State -->
          <div
            v-if="executing"
            style="
              padding: 2rem;
              text-align: center;
              color: #8b949e;
              background: #0d1117;
              border: 1px solid #30363d;
              border-radius: 4px;
            "
          >
            <div style="margin-bottom: 0.5rem; font-size: 1.5rem">⚡</div>
            <div style="font-weight: 500">Executing query...</div>
            <div style="font-size: 0.875rem; margin-top: 0.5rem">Request sent, waiting for response</div>
          </div>

          <!-- Error State -->
          <div v-else-if="error" class="alert alert-danger" style="display: block">{{ error }}</div>

          <!-- Result State -->
          <div v-else-if="result">
            <pre class="json-display" style="max-height: 500px; overflow: auto">{{ formattedResult }}</pre>
          </div>

          <!-- Empty State -->
          <div
            v-else
            style="
              padding: 2rem;
              text-align: center;
              color: #6c757d;
              background: #0d1117;
              border: 1px solid #30363d;
              border-radius: 4px;
            "
          >
            <div style="font-size: 1.5rem; margin-bottom: 0.5rem">📝</div>
            <div>Response will appear here after execution</div>
          </div>
        </div>
      </div>

      <!-- Log Stream Panel (shown during/after execution) -->
      <div v-if="showLogs || executing || requestIdHeader" class="modal-section" style="margin-top: 1rem">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem">
          <h4 style="margin: 0; display: flex; align-items: center; gap: 0.5rem">
            <span>Request Logs</span>
            <span v-if="requestIdHeader" style="font-size: 0.75rem; color: #8b949e; font-weight: normal">
              ({{ requestIdHeader.substring(0, 12) }}...)
            </span>
          </h4>
          <button @click="toggleLogs" class="btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem">
            {{ showLogs ? 'Hide' : 'Show' }}
          </button>
        </div>
        <div v-if="showLogs">
          <log-stream
            ref="logStream"
            :request-id-filter="requestIdHeader"
            :max-logs="500"
            :auto-scroll="true"
            :compact="true"
            :show-container="true"
            @log-clicked="handleLogClick"
          />
        </div>
      </div>
    </main>
  </div>
</div>

</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { API } from '@/utils/api'
import type { 
  Server,
  SampleQuery,
  ExecuteResponse,
  ExecutionDetail
} from '@/types'

import LogStream from '../components/LogStream.vue'
import { GraphQLEditorManager } from "@/utils/graphql-editor-manager";
import { copyToClipboard, applySyntaxHighlighting } from '@/utils/ui-utils'

export default defineComponent(// Export component definition (template will be provided by SPA loader)
{
  components: {
    LogStream,
  },
  data() {
    return {
      servers: [] as Server[],
      selectedServerId: "",
      query: "",
      operationName: "",
      variables: "{}",
      executing: false,
      result: null as string | null,
      error: null as string | null,
      executionId: null as number | null,
      requestIdHeader: null as string | null, // Request ID for log filtering
      showSampleQueries: false,
      sampleQueries: [] as SampleQuery[],
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
        console.error("Error parsing result:", e);
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
      const queryTypeObj = this.schemaTypes.find((t) => t.name === this.queryType.name);
      if (!queryTypeObj || !queryTypeObj.fields) return [];

      if (!this.schemaFilter) return queryTypeObj.fields;

      const filterLower = this.schemaFilter.toLowerCase();
      return queryTypeObj.fields.filter(
        (field) =>
          field.name.toLowerCase().includes(filterLower) ||
          (field.description && field.description.toLowerCase().includes(filterLower))
      );
    },

    filteredMutationFields() {
      if (!this.schema || !this.mutationType) return [];
      const mutationTypeObj = this.schemaTypes.find((t) => t.name === this.mutationType.name);
      if (!mutationTypeObj || !mutationTypeObj.fields) return [];

      if (!this.schemaFilter) return mutationTypeObj.fields;

      const filterLower = this.schemaFilter.toLowerCase();
      return mutationTypeObj.fields.filter(
        (field) =>
          field.name.toLowerCase().includes(filterLower) ||
          (field.description && field.description.toLowerCase().includes(filterLower))
      );
    },

    filteredObjectTypes() {
      const objectTypes = this.getObjectTypes();
      if (!this.schemaFilter) return objectTypes;

      const filterLower = this.schemaFilter.toLowerCase();
      return objectTypes.filter(
        (type) =>
          type.name.toLowerCase().includes(filterLower) ||
          (type.description && type.description.toLowerCase().includes(filterLower))
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

  updated() {
    this.$nextTick(() => {
      this.applySyntaxHighlighting();
    });
  },

  methods: {
    async loadServers() {
      try {
        this.servers = await API.get<Server[]>("/api/servers");
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
        this.sampleQueries = await API.get<SampleQuery[]>("/api/requests");
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
        const requestData: { query: string; operationName?: string; variables?: any } = {
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

        const response = await API.post<ExecuteResponse>("/api/execute", payload);

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

    async pollForResult(executionId: number, maxAttempts = 30, intervalMs = 1000) {
      for (let attempt = 0; attempt < maxAttempts; attempt++) {
        try {
          const execution = await API.get<ExecutionDetail>(`/api/executions/${executionId}`);

          console.log("execution", execution);

          // Extract request ID header for log filtering
          if (execution.execution.requestIdHeader && !this.requestIdHeader) {
            this.requestIdHeader = execution.execution.requestIdHeader;
            console.log("Set requestIdHeader for log filtering:", this.requestIdHeader);
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
        this.variables = data.variables ? JSON.stringify(data.variables, null, 2) : "{}";

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
        window.location.href = `/requests/detail/?id=${this.executionId}`;
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
      return scalarTypes.includes(type.name) || type.kind === "SCALAR" || type.kind === "ENUM";
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
      return this.schemaTypes.filter((t) => !t.name.startsWith("__") && t.kind === "INPUT_OBJECT");
    },

    getEnumTypes() {
      if (!this.schema) return [];
      return this.schemaTypes.filter((t) => !t.name.startsWith("__") && t.kind === "ENUM");
    },

    getScalarTypes() {
      if (!this.schema) return [];
      return this.schemaTypes.filter((t) => !t.name.startsWith("__") && t.kind === "SCALAR");
    },

    handleLogClick(log) {
      console.log("Log clicked:", log);
      // Could open a modal with log details if needed
    },

    toggleLogs() {
      this.showLogs = !this.showLogs;
    },

    applySyntaxHighlighting() {
      applySyntaxHighlighting();
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
})
</script>
