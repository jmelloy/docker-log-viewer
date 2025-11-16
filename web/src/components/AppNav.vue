<template>
  <nav class="app-nav">
    <router-link to="/logs" :class="{ active: activePage === 'viewer' }"> Log Viewer </router-link>
    <router-link to="/requests" :class="{ active: activePage === 'requests' || activePage === 'graphql-explorer' }">
      Request Manager
    </router-link>
    <router-link to="/requests?tab=graphql-explorer" :class="{ active: activePage === 'graphql-explorer' }">
      GraphQL Explorer
    </router-link>
    <router-link to="/settings" :class="{ active: activePage === 'settings' }"> Settings </router-link>
  </nav>
</template>

<script lang="ts">
import { defineComponent } from "vue";

export default defineComponent({
  name: "AppNav",
  props: {
    activePage: {
      type: String,
      default: "viewer",
    },
  },
});
</script>
