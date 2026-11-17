<template>
  <div class="modal" @click.self="close" @keydown.esc.stop="close">
    <form
      class="modal-content"
      role="dialog"
      aria-modal="true"
      aria-labelledby="savedRequestTitle"
      @submit.prevent="save"
    >
      <div class="modal-header">
        <h3 id="savedRequestTitle">{{ request ? "Edit Saved Request" : "New Saved Request" }}</h3>
        <button type="button" aria-label="Close" :disabled="saving" @click="close">×</button>
      </div>
      <div class="modal-body">
        <p v-if="error" class="alert alert-danger" role="alert">{{ error }}</p>
        <div class="form-group">
          <label for="savedRequestName">Name:</label>
          <input id="savedRequestName" v-model="form.name" type="text" placeholder="e.g., FetchUsers" />
        </div>
        <div class="form-group">
          <label for="savedRequestServer">Server (optional):</label>
          <select id="savedRequestServer" v-model="form.serverId" :disabled="form.createNewServer">
            <option value="">-- No Server --</option>
            <option v-for="server in servers" :key="server.id" :value="String(server.id)">
              {{ server.name }} ({{ server.url }})
            </option>
          </select>
        </div>
        <div class="form-group">
          <label class="checkbox-label">
            <input v-model="form.createNewServer" type="checkbox" @change="form.serverId = ''" />
            <span>Create New Server</span>
          </label>
        </div>
        <template v-if="form.createNewServer">
          <div class="form-group">
            <label for="savedRequestServerURL">URL (optional):</label>
            <input
              id="savedRequestServerURL"
              v-model="form.url"
              type="text"
              placeholder="https://api.example.com/graphql"
            />
          </div>
          <div class="form-group">
            <label for="savedRequestToken">Bearer Token (optional):</label>
            <input id="savedRequestToken" v-model="form.bearerToken" type="text" placeholder="your-token-here" />
          </div>
        </template>
        <div class="form-group">
          <label for="savedRequestHeaders">Custom Headers (JSON):</label>
          <textarea
            id="savedRequestHeaders"
            v-model="form.headers"
            rows="5"
            placeholder='{"X-Custom-Header": "value"}'
          ></textarea>
          <small>Overrides server headers for this saved request.</small>
        </div>
        <div class="form-group">
          <label for="savedRequestURL">Request URL (optional):</label>
          <input
            id="savedRequestURL"
            v-model="form.urlOverride"
            type="text"
            placeholder="/api/search or https://api.example.com/api/search"
          />
          <small>Leave blank to use the server URL.</small>
        </div>
        <div class="form-group">
          <label for="savedRequestData">Request Data (JSON):</label>
          <textarea
            id="savedRequestData"
            v-model="form.requestData"
            rows="15"
            placeholder='{"query": "{ users { id name } }"}'
          ></textarea>
        </div>
      </div>
      <div class="modal-footer">
        <button type="submit" class="btn-primary" :disabled="saving">{{ saving ? "Saving…" : "Save Request" }}</button>
        <button type="button" class="btn-secondary" :disabled="saving" @click="close">Cancel</button>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from "vue";
import { API } from "@/utils/api";
import { parseHeaders } from "@/utils/headers";
import type { SampleQuery, Server } from "@/types";

const props = defineProps<{ request: SampleQuery | null; servers: Server[] }>();
const emit = defineEmits<{ saved: []; close: [] }>();
const form = reactive({
  name: props.request?.name || "",
  requestData: props.request?.requestData || "",
  serverId: props.request?.serverId ? String(props.request.serverId) : "",
  headers: JSON.stringify(props.request?.headers || {}, null, 2),
  urlOverride: props.request?.urlOverride || "",
  createNewServer: false,
  url: "",
  bearerToken: "",
});
const saving = ref(false);
const error = ref("");

function close() {
  if (!saving.value) emit("close");
}

async function save() {
  if (saving.value) return;
  saving.value = true;
  error.value = "";
  try {
    const payload = {
      name: form.name,
      requestData: form.requestData,
      headers: parseHeaders(form.headers),
      urlOverride: form.urlOverride,
      serverId: !form.createNewServer && form.serverId ? Number(form.serverId) : null,
      url: form.createNewServer ? form.url : "",
      bearerToken: form.createNewServer ? form.bearerToken : "",
    };
    if (props.request) {
      await API.put(`/api/samples/${props.request.id}`, payload);
    } else {
      await API.post("/api/samples/", payload);
    }
    emit("saved");
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped>
.modal-content {
  max-width: 720px;
}

.modal-header,
.modal-footer {
  flex-shrink: 0;
}

.modal-body {
  min-height: 0;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.form-group small {
  display: block;
  margin-top: 0.5rem;
  color: var(--text-secondary);
  line-height: 1.5;
}

.form-group:last-child {
  margin-bottom: 0;
}
</style>
