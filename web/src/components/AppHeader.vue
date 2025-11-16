<template>
  <header class="app-header">
    <div class="app-header-content">
      <h1>🔱 Logseidon</h1>
      <AppNav :activePage="activePage" />
      <slot></slot>
    </div>
  </header>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import AppNav from "./AppNav.vue";

export default defineComponent({
  name: "AppHeader",
  components: {
    AppNav,
  },
  props: {
    activePage: {
      type: String,
      default: "viewer",
    },
  },
});
</script>
