import { loadTemplate } from "./template-loader.js";

const template = await loadTemplate("/templates/modal.html");

export function createModal() {
  return {
    template,
    props: {
      title: {
        type: String,
        required: true,
      },
    },
    computed: {
      hasHeaderActions() {
        return !!this.$slots["header-actions"];
      },
      hasFooter() {
        return !!this.$slots.footer;
      },
    },
  };
}
